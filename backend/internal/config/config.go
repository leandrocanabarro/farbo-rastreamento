// Package config carrega toda a configuração a partir de variáveis de ambiente.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env       string
	HTTP      HTTP
	TCP       TCP
	Postgres  Postgres
	Redis     Redis
	Auth      Auth
	Tracking  Tracking
	Commands  Commands
	Telemetry Telemetry
	Bootstrap Bootstrap
}

type HTTP struct {
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
	CORSOrigins     []string
	RateLimitRPS    float64
	RateLimitBurst  int
}

type TCP struct {
	Port int
	// MaxPacketSize limita o buffer por conexão, evitando exaustão de memória (§7).
	MaxPacketSize int
	// ReadTimeout derruba conexões mudas.
	ReadTimeout time.Duration
	// WriteTimeout limita o envio de um comando.
	WriteTimeout time.Duration
	// MaxConnections limita o total de sessões simultâneas.
	MaxConnections int
	// KeepAlive do socket TCP.
	KeepAlive time.Duration
}

type Postgres struct {
	Host     string
	Port     int
	Database string
	User     string
	Password string
	SSLMode  string
	MaxConns int32
}

func (p Postgres) DSN() string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(p.User, p.Password),
		Host:   fmt.Sprintf("%s:%d", p.Host, p.Port),
		Path:   "/" + p.Database,
	}
	q := u.Query()
	q.Set("sslmode", p.SSLMode)
	u.RawQuery = q.Encode()
	return u.String()
}

type Redis struct {
	Enabled  bool
	Host     string
	Port     int
	Password string
	DB       int
}

func (r Redis) Addr() string { return fmt.Sprintf("%s:%d", r.Host, r.Port) }

type Auth struct {
	JWTSecret       []byte
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	BcryptCost      int
}

type Tracking struct {
	// StaleAfter: sem pacotes por este tempo o dispositivo vira STALE (§17).
	StaleAfter time.Duration
	// OfflineAfter: sem pacotes por este tempo o dispositivo vira OFFLINE.
	OfflineAfter time.Duration
	// StatusSweepInterval é a frequência da varredura de status.
	StatusSweepInterval time.Duration
	// MaxHistoryPoints limita o retorno da API de histórico (§12).
	MaxHistoryPoints int
	// SimplifyToleranceMeters é a tolerância do Douglas-Peucker no histórico.
	SimplifyToleranceMeters float64
	// DefaultSpeedLimitKmh usado quando o veículo não define o próprio limite.
	DefaultSpeedLimitKmh float64
	// OverspeedHysteresisKmh evita oscilação do estado de excesso (§21).
	OverspeedHysteresisKmh float64
}

type Commands struct {
	// AckTimeout: sem ACK neste prazo o comando vira TIMEOUT (§16).
	AckTimeout time.Duration
	// EngineCutMaxSpeedKmh é o limite de segurança para o corte (§14).
	EngineCutMaxSpeedKmh float64
	// EngineCutMaxPositionAge recusa o corte se a última posição for antiga demais.
	EngineCutMaxPositionAge time.Duration
	// SweepInterval é a frequência da varredura de comandos vencidos.
	SweepInterval time.Duration
}

type Telemetry struct {
	ServiceName  string
	LogLevel     string
	LogFormat    string // json | text
	OTLPEndpoint string // vazio desliga o tracing
	TraceRatio   float64
	MaskIMEI     bool
}

type Bootstrap struct {
	AdminEmail    string
	AdminPassword string
	AdminName     string
}

func Load() (*Config, error) {
	cfg := &Config{
		Env: str("APP_ENV", "development"),
		HTTP: HTTP{
			Port:            num("HTTP_PORT", 8080),
			ReadTimeout:     dur("HTTP_READ_TIMEOUT", 15*time.Second),
			WriteTimeout:    dur("HTTP_WRITE_TIMEOUT", 30*time.Second),
			ShutdownTimeout: dur("HTTP_SHUTDOWN_TIMEOUT", 20*time.Second),
			CORSOrigins:     csv("CORS_ORIGINS", "http://localhost:5173,http://localhost:3000"),
			RateLimitRPS:    flt("RATE_LIMIT_RPS", 20),
			RateLimitBurst:  num("RATE_LIMIT_BURST", 40),
		},
		TCP: TCP{
			Port:           num("TCP_PORT", 5000),
			MaxPacketSize:  num("TCP_MAX_PACKET_SIZE", 8192),
			ReadTimeout:    dur("TCP_READ_TIMEOUT", 10*time.Minute),
			WriteTimeout:   dur("TCP_WRITE_TIMEOUT", 10*time.Second),
			MaxConnections: num("TCP_MAX_CONNECTIONS", 10000),
			KeepAlive:      dur("TCP_KEEPALIVE", 60*time.Second),
		},
		Postgres: Postgres{
			Host:     str("POSTGRES_HOST", "localhost"),
			Port:     num("POSTGRES_PORT", 5432),
			Database: str("POSTGRES_DB", "tracker"),
			User:     str("POSTGRES_USER", "tracker"),
			Password: str("POSTGRES_PASSWORD", ""),
			SSLMode:  str("POSTGRES_SSLMODE", "disable"),
			MaxConns: int32(num("POSTGRES_MAX_CONNS", 15)),
		},
		Redis: Redis{
			Enabled:  bl("REDIS_ENABLED", false),
			Host:     str("REDIS_HOST", "localhost"),
			Port:     num("REDIS_PORT", 6379),
			Password: str("REDIS_PASSWORD", ""),
			DB:       num("REDIS_DB", 0),
		},
		Auth: Auth{
			JWTSecret:       []byte(str("JWT_SECRET", "")),
			AccessTokenTTL:  dur("JWT_ACCESS_TTL", 15*time.Minute),
			RefreshTokenTTL: dur("JWT_REFRESH_TTL", 720*time.Hour),
			BcryptCost:      num("BCRYPT_COST", 12),
		},
		Tracking: Tracking{
			StaleAfter:              dur("DEVICE_STALE_AFTER", 2*time.Minute),
			OfflineAfter:            dur("DEVICE_OFFLINE_AFTER", 5*time.Minute),
			StatusSweepInterval:     dur("DEVICE_STATUS_SWEEP", 30*time.Second),
			MaxHistoryPoints:        num("HISTORY_MAX_POINTS", 5000),
			SimplifyToleranceMeters: flt("HISTORY_SIMPLIFY_TOLERANCE_M", 5),
			DefaultSpeedLimitKmh:    flt("DEFAULT_SPEED_LIMIT_KMH", 0),
			OverspeedHysteresisKmh:  flt("OVERSPEED_HYSTERESIS_KMH", 5),
		},
		Commands: Commands{
			AckTimeout:              dur("COMMAND_ACK_TIMEOUT", 15*time.Second),
			EngineCutMaxSpeedKmh:    flt("ENGINE_CUT_MAX_SPEED_KMH", 5),
			EngineCutMaxPositionAge: dur("ENGINE_CUT_MAX_POSITION_AGE", 10*time.Minute),
			SweepInterval:           dur("COMMAND_SWEEP_INTERVAL", 5*time.Second),
		},
		Telemetry: Telemetry{
			ServiceName:  str("OTEL_SERVICE_NAME", "tracker-platform"),
			LogLevel:     str("LOG_LEVEL", "info"),
			LogFormat:    str("LOG_FORMAT", "json"),
			OTLPEndpoint: str("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
			TraceRatio:   flt("OTEL_TRACE_SAMPLE_RATIO", 1),
			MaskIMEI:     bl("LOG_MASK_IMEI", true),
		},
		Bootstrap: Bootstrap{
			AdminEmail:    str("ADMIN_EMAIL", ""),
			AdminPassword: str("ADMIN_PASSWORD", ""),
			AdminName:     str("ADMIN_NAME", "Administrador"),
		},
	}

	if len(cfg.Auth.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET é obrigatório e precisa de ao menos 32 caracteres")
	}
	if cfg.Postgres.Password == "" {
		return nil, fmt.Errorf("POSTGRES_PASSWORD é obrigatório")
	}
	if cfg.TCP.MaxPacketSize < 512 || cfg.TCP.MaxPacketSize > 1<<20 {
		return nil, fmt.Errorf("TCP_MAX_PACKET_SIZE fora da faixa aceitável (512..1048576)")
	}
	return cfg, nil
}

func str(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func num(key string, def int) int {
	if v, err := strconv.Atoi(str(key, "")); err == nil {
		return v
	}
	return def
}

func flt(key string, def float64) float64 {
	if v, err := strconv.ParseFloat(str(key, ""), 64); err == nil {
		return v
	}
	return def
}

func bl(key string, def bool) bool {
	if v, err := strconv.ParseBool(str(key, "")); err == nil {
		return v
	}
	return def
}

func dur(key string, def time.Duration) time.Duration {
	if v, err := time.ParseDuration(str(key, "")); err == nil {
		return v
	}
	return def
}

func csv(key, def string) []string {
	raw := strings.Split(str(key, def), ",")
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}
