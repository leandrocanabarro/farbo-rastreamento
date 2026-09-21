// Package geocoding converte coordenadas em um endereço legível.
//
// Usa a API pública de geocodificação reversa do Nominatim (OpenStreetMap) —
// o mesmo provedor que já fornece os tiles do mapa no frontend — respeitando
// a política de uso do serviço: no máximo 1 requisição por segundo e um
// User-Agent que identifica a aplicação.
// https://operations.osmfoundation.org/policies/nominatim/
package geocoding

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	baseURL        = "https://nominatim.openstreetmap.org/reverse"
	requestTimeout = 8 * time.Second

	// O endereço de uma coordenada não muda: o cache pode durar bastante.
	cacheTTL = 30 * 24 * time.Hour

	// cachePrecision arredonda a chave do cache a ~11 m — o bastante para um
	// veículo parado (ruído normal de GPS) sempre bater na mesma entrada em
	// vez de gerar uma consulta nova a cada posição recebida.
	cachePrecision = 4
)

type cacheEntry struct {
	address   string
	expiresAt time.Time
}

// Service converte coordenadas em endereço, com cache e limite de taxa
// compartilhado entre todas as chamadas — o limite do Nominatim é por
// aplicação, não por usuário do painel.
type Service struct {
	client    *http.Client
	limiter   *rate.Limiter
	userAgent string
	log       *slog.Logger

	mu    sync.Mutex
	cache map[string]cacheEntry
}

func NewService(contactEmail string, log *slog.Logger) *Service {
	ua := "tracker-platform/1.0 (self-hosted vehicle tracker)"
	if contactEmail = strings.TrimSpace(contactEmail); contactEmail != "" {
		ua = fmt.Sprintf("tracker-platform/1.0 (self-hosted vehicle tracker; contact: %s)", contactEmail)
	}
	return &Service{
		client:    &http.Client{Timeout: requestTimeout},
		limiter:   rate.NewLimiter(rate.Every(1100*time.Millisecond), 1),
		userAgent: ua,
		log:       log.With("component", "geocoding"),
		cache:     map[string]cacheEntry{},
	}
}

type nominatimResponse struct {
	DisplayName string            `json:"display_name"`
	Address     map[string]string `json:"address"`
	Error       string            `json:"error"`
}

// Reverse devolve um endereço legível para a coordenada informada.
func (s *Service) Reverse(ctx context.Context, lat, lon float64) (string, error) {
	key := cacheKey(lat, lon)

	s.mu.Lock()
	if entry, ok := s.cache[key]; ok && time.Now().Before(entry.expiresAt) {
		s.mu.Unlock()
		return entry.address, nil
	}
	s.mu.Unlock()

	// O Wait aqui é o que garante o limite de 1 req/s mesmo com várias abas
	// do painel pedindo endereços diferentes ao mesmo tempo.
	if err := s.limiter.Wait(ctx); err != nil {
		return "", fmt.Errorf("aguardando limite de requisições: %w", err)
	}

	address, err := s.fetch(ctx, lat, lon)
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	s.cache[key] = cacheEntry{address: address, expiresAt: time.Now().Add(cacheTTL)}
	s.mu.Unlock()

	return address, nil
}

func (s *Service) fetch(ctx context.Context, lat, lon float64) (string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	query := url.Values{
		"format":         {"jsonv2"},
		"lat":            {strconv.FormatFloat(lat, 'f', 6, 64)},
		"lon":            {strconv.FormatFloat(lon, 'f', 6, 64)},
		"zoom":           {"18"},
		"addressdetails": {"1"},
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, baseURL+"?"+query.Encode(), nil)
	if err != nil {
		return "", err
	}
	// Exigido pela política de uso do Nominatim: identificar a aplicação.
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9,en;q=0.5")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("consultando geocodificação: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("geocodificação devolveu status %d", resp.StatusCode)
	}

	var body nominatimResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("decodificando resposta da geocodificação: %w", err)
	}
	if body.Error != "" {
		return "", fmt.Errorf("geocodificação: %s", body.Error)
	}

	return formatAddress(body), nil
}

// formatAddress monta um endereço curto a partir dos componentes do
// Nominatim. É preferível ao "display_name" completo, que costuma incluir
// país, CEP e outros detalhes que não cabem numa etiqueta de mapa.
func formatAddress(body nominatimResponse) string {
	addr := body.Address

	street := addr["road"]
	if street != "" {
		if number := addr["house_number"]; number != "" {
			street = street + ", " + number
		}
	}

	area := firstNonEmpty(addr["suburb"], addr["neighbourhood"], addr["quarter"])
	city := firstNonEmpty(addr["city"], addr["town"], addr["village"], addr["municipality"])

	parts := make([]string, 0, 3)
	for _, p := range []string{street, area, city} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		return body.DisplayName
	}
	return strings.Join(parts, ", ")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func cacheKey(lat, lon float64) string {
	return fmt.Sprintf("%.*f,%.*f", cachePrecision, lat, cachePrecision, lon)
}

// PurgeExpired remove entradas vencidas do cache. Chamado periodicamente para
// o mapa em memória não crescer sem limite ao longo de meses de uso.
func (s *Service) PurgeExpired() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, entry := range s.cache {
		if now.After(entry.expiresAt) {
			delete(s.cache, key)
		}
	}
}
