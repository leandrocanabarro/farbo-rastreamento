// Comando server: sobe a API HTTP, o servidor TCP de rastreadores e as
// rotinas de manutenção da plataforma.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/farbo/tracker-platform/backend/internal/api"
	"github.com/farbo/tracker-platform/backend/internal/audit"
	"github.com/farbo/tracker-platform/backend/internal/auth"
	"github.com/farbo/tracker-platform/backend/internal/commands"
	"github.com/farbo/tracker-platform/backend/internal/config"
	"github.com/farbo/tracker-platform/backend/internal/database"
	"github.com/farbo/tracker-platform/backend/internal/devices"
	"github.com/farbo/tracker-platform/backend/internal/events"
	"github.com/farbo/tracker-platform/backend/internal/geocoding"
	"github.com/farbo/tracker-platform/backend/internal/geofences"
	"github.com/farbo/tracker-platform/backend/internal/protocols"
	"github.com/farbo/tracker-platform/backend/internal/protocols/gt06"
	"github.com/farbo/tracker-platform/backend/internal/protocols/h02"
	"github.com/farbo/tracker-platform/backend/internal/protocols/tkstar"
	"github.com/farbo/tracker-platform/backend/internal/tcp"
	"github.com/farbo/tracker-platform/backend/internal/telemetry"
	"github.com/farbo/tracker-platform/backend/internal/tracking"
	"github.com/farbo/tracker-platform/backend/internal/vehicles"
	ws "github.com/farbo/tracker-platform/backend/internal/websocket"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "erro fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log := telemetry.NewLogger(cfg.Telemetry.LogLevel, cfg.Telemetry.LogFormat)
	slog.SetDefault(log)
	telemetry.SetMaskIMEI(cfg.Telemetry.MaskIMEI)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := telemetry.SetupTracing(ctx, cfg.Telemetry.ServiceName,
		cfg.Telemetry.OTLPEndpoint, cfg.Env, cfg.Telemetry.TraceRatio)
	if err != nil {
		return fmt.Errorf("configurando tracing: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownTracing(shutdownCtx)
	}()

	metrics := telemetry.NewMetrics()

	// ---- Banco ----
	db, err := database.Connect(ctx, cfg.Postgres)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.Migrate(ctx, log); err != nil {
		return fmt.Errorf("aplicando migrations: %w", err)
	}

	// ---- Protocolos ----
	//
	// A ordem importa: a detecção usa o primeiro adaptador que reconhecer o
	// tráfego, então os formatos com assinatura forte vêm antes, e o modo de
	// captura (que aceita qualquer coisa) vem por último.
	registry := protocols.NewRegistry(
		gt06.New(envBool("GT06_ACK_GPS", false)),
		h02.New(envInt("H02_BINARY_FRAME_LENGTH", 0)),
		tkstar.NewV1(),
		tkstar.NewV1Extended(),
		tkstar.NewV4(envBool("TKSTAR_V4_CAPTURE", false)),
	)
	logProtocols(log, registry)

	// ---- WebSocket ----
	hub := ws.NewHub(log, func(total int) {
		metrics.WebsocketClients.Set(float64(total))
	})

	var redisClient *redis.Client
	if cfg.Redis.Enabled {
		redisClient = redis.NewClient(&redis.Options{
			Addr:     cfg.Redis.Addr(),
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		})
		if err := redisClient.Ping(ctx).Err(); err != nil {
			// Redis é opcional: sem ele o hub continua servindo esta instância.
			log.Warn("Redis indisponível; seguindo sem replicação entre instâncias", "err", err)
			_ = redisClient.Close()
			redisClient = nil
		} else {
			bridge := ws.NewBridge(redisClient, hub, log)
			go bridge.Run(ctx)
			defer func() { _ = redisClient.Close() }()
		}
	}

	// ---- Repositórios e serviços ----
	deviceRepo := devices.NewRepository(db)
	vehicleRepo := vehicles.NewRepository(db)
	positionRepo := tracking.NewRepository(db)
	stateRepo := tracking.NewStateRepository(db)
	eventRepo := events.NewRepository(db)
	commandRepo := commands.NewRepository(db)
	geofenceRepo := geofences.NewRepository(db)
	auditRepo := audit.NewRepository(db)
	userRepo := auth.NewRepository(db)
	rawRepo := tracking.NewRawPacketRepository(db)

	authSvc := auth.NewService(userRepo, cfg.Auth, log)
	if err := authSvc.EnsureBootstrapUser(ctx, cfg.Bootstrap); err != nil {
		return err
	}

	deviceSvc := devices.NewService(deviceRepo, registry)
	vehicleSvc := vehicles.NewService(vehicleRepo)
	eventSvc := events.NewService(eventRepo, hub, log)
	auditSvc := audit.NewService(auditRepo, log)
	geofenceSvc := geofences.NewService(geofenceRepo)
	if err := geofenceSvc.Refresh(ctx); err != nil {
		return fmt.Errorf("carregando cercas: %w", err)
	}

	// Reaproveita o e-mail do admin como contato do User-Agent exigido pela
	// política de uso do Nominatim — evita mais uma variável de ambiente só
	// para isso.
	geocodingSvc := geocoding.NewService(cfg.Bootstrap.AdminEmail, log)

	stateStore := tracking.NewStateStore(stateRepo)
	if err := stateStore.Load(ctx); err != nil {
		return fmt.Errorf("carregando estado dos dispositivos: %w", err)
	}

	connManager := tcp.NewManager(func(total int) {
		metrics.Connections.Set(float64(total))
	})

	commandSvc := commands.NewService(
		commandRepo, registry, commandSender{connManager},
		tracking.NewSnapshotProvider(positionRepo),
		eventSvc, auditSvc, hub, cfg.Commands, metrics, log,
	)

	ingestor := tracking.NewIngestor(
		deviceSvc, vehicleSvc, positionRepo, stateStore, eventSvc,
		geofenceSvc, commandSvc, rawRepo, hub, cfg.Tracking, metrics, log,
	)

	// ---- Servidores ----
	tcpServer := tcp.NewServer(cfg.TCP, registry, connManager, ingestor, log, metrics)

	apiServer := api.NewServer(api.Deps{
		Config: cfg, Log: log, Metrics: metrics, DB: db,
		Auth: authSvc, Devices: deviceSvc, Vehicles: vehicleSvc,
		Geofences: geofenceSvc, Geocoder: geocodingSvc, Events: eventSvc, Commands: commandSvc,
		Audit: auditSvc, Positions: positionRepo, States: stateStore,
		Raw: rawRepo, Ingestor: ingestor, Conns: connManager, Registry: registry,
		WS: ws.NewHandler(hub, cfg.HTTP.CORSOrigins), Hub: hub,
	})

	var wg sync.WaitGroup
	errCh := make(chan error, 2)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := apiServer.ListenAndServe(ctx); err != nil {
			errCh <- fmt.Errorf("API HTTP: %w", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		addr := fmt.Sprintf(":%d", cfg.TCP.Port)
		if err := tcpServer.ListenAndServe(ctx, addr); err != nil {
			errCh <- fmt.Errorf("servidor TCP: %w", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		runWorkers(ctx, cfg, ingestor, commandSvc, authSvc, rawRepo, geocodingSvc, log)
	}()

	log.Info("plataforma no ar",
		"env", cfg.Env, "http", cfg.HTTP.Port, "tcp", cfg.TCP.Port,
		"redis", redisClient != nil)

	select {
	case err := <-errCh:
		stop()
		wg.Wait()
		return err
	case <-ctx.Done():
		log.Info("sinal de encerramento recebido, finalizando")
		wg.Wait()
		return nil
	}
}

// runWorkers concentra as rotinas periódicas.
func runWorkers(
	ctx context.Context,
	cfg *config.Config,
	ingestor *tracking.Ingestor,
	commandSvc *commands.Service,
	authSvc *auth.Service,
	rawRepo *tracking.RawPacketRepository,
	geocodingSvc *geocoding.Service,
	log *slog.Logger,
) {
	statusTicker := time.NewTicker(cfg.Tracking.StatusSweepInterval)
	commandTicker := time.NewTicker(cfg.Commands.SweepInterval)
	cleanupTicker := time.NewTicker(6 * time.Hour)
	defer statusTicker.Stop()
	defer commandTicker.Stop()
	defer cleanupTicker.Stop()

	// Primeira varredura imediata para o painel já abrir com o status correto.
	ingestor.SweepStatuses(ctx)

	for {
		select {
		case <-ctx.Done():
			return

		case <-statusTicker.C:
			ingestor.SweepStatuses(ctx)

		case <-commandTicker.C:
			commandSvc.SweepTimeouts(ctx)

		case <-cleanupTicker.C:
			authSvc.CleanupExpiredTokens(ctx)
			geocodingSvc.PurgeExpired()
			cutoff := time.Now().Add(-30 * 24 * time.Hour)
			if removed, err := rawRepo.DeleteOlderThan(ctx, cutoff); err != nil {
				log.Warn("falha ao limpar pacotes crus", "err", err)
			} else if removed > 0 {
				log.Info("pacotes crus antigos removidos", "count", removed)
			}
		}
	}
}

// commandSender adapta o gerenciador de conexões à interface do módulo de
// comandos, traduzindo o erro de "não conectado".
type commandSender struct {
	manager *tcp.Manager
}

func (s commandSender) Send(imei string, payload []byte) error {
	err := s.manager.Send(imei, payload)
	if errors.Is(err, tcp.ErrNotConnected) {
		return commands.ErrNotConnected
	}
	return err
}

func logProtocols(log *slog.Logger, registry *protocols.ProtocolRegistry) {
	for _, descriptor := range registry.Descriptors() {
		level := slog.LevelInfo
		if descriptor.Confidence != protocols.Documented {
			// Quem opera precisa saber que há adaptador não confirmado ativo.
			level = slog.LevelWarn
		}
		log.Log(context.Background(), level, "protocolo registrado",
			"name", descriptor.Name,
			"confidence", descriptor.Confidence,
			"commands", len(descriptor.Commands))
	}
}

func envInt(key string, def int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return def
	}
	return value
}

func envBool(key string, def bool) bool {
	switch os.Getenv(key) {
	case "true", "1", "yes":
		return true
	case "false", "0", "no":
		return false
	default:
		return def
	}
}
