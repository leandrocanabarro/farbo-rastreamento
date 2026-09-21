// Package api expõe a API REST e monta as rotas do painel.
package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

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
	"github.com/farbo/tracker-platform/backend/internal/tcp"
	"github.com/farbo/tracker-platform/backend/internal/telemetry"
	"github.com/farbo/tracker-platform/backend/internal/tracking"
	"github.com/farbo/tracker-platform/backend/internal/vehicles"
	ws "github.com/farbo/tracker-platform/backend/internal/websocket"
)

// Deps reúne tudo o que os handlers precisam.
type Deps struct {
	Config    *config.Config
	Log       *slog.Logger
	Metrics   *telemetry.Metrics
	DB        *database.DB
	Auth      *auth.Service
	Devices   *devices.Service
	Vehicles  *vehicles.Service
	Geofences *geofences.Service
	Geocoder  *geocoding.Service
	Events    *events.Service
	Commands  *commands.Service
	Audit     *audit.Service
	Positions *tracking.Repository
	States    *tracking.StateStore
	Raw       *tracking.RawPacketRepository
	Ingestor  *tracking.Ingestor
	Conns     *tcp.Manager
	Registry  *protocols.ProtocolRegistry
	WS        *ws.Handler
	Hub       *ws.Hub
}

type Server struct {
	Deps
	router chi.Router
}

func NewServer(deps Deps) *Server {
	s := &Server{Deps: deps}
	s.router = s.routes()
	return s
}

func (s *Server) Handler() http.Handler { return s.router }

func (s *Server) routes() chi.Router {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(loggingMiddleware(s.Log))
	r.Use(metricsMiddleware(s.Metrics))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   s.Config.HTTP.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Observabilidade sem autenticação: fica atrás da rede interna (§28).
	r.Get("/health", s.handleHealth)
	r.Get("/ready", s.handleReady)
	r.Handle("/metrics", s.Metrics.Handler())

	limiter := newRateLimiter(s.Config.HTTP.RateLimitRPS, s.Config.HTTP.RateLimitBurst)
	// O login recebe um limite bem mais apertado que o resto da API.
	loginLimiter := newRateLimiter(0.5, 5)

	r.Route("/api", func(r chi.Router) {
		r.Use(limiter.middleware)

		r.Route("/auth", func(r chi.Router) {
			r.With(loginLimiter.middleware).Post("/login", s.handleLogin)
			r.Post("/refresh", s.handleRefresh)
			r.Post("/logout", s.handleLogout)
			r.With(s.Auth.Middleware).Get("/me", s.handleMe)
		})

		r.Group(func(r chi.Router) {
			r.Use(s.Auth.Middleware)

			r.Get("/protocols", s.handleListProtocols)
			r.Get("/geocoding/reverse", s.handleReverseGeocode)

			r.Route("/vehicles", func(r chi.Router) {
				r.Get("/", s.handleListVehicles)
				r.With(auth.RequireRole(auth.RoleAdmin)).Post("/", s.handleCreateVehicle)

				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", s.handleGetVehicle)
					r.With(auth.RequireRole(auth.RoleAdmin)).Patch("/", s.handleUpdateVehicle)
					r.With(auth.RequireRole(auth.RoleAdmin)).Delete("/", s.handleDeleteVehicle)

					r.Get("/position", s.handleVehiclePosition)
					r.Get("/positions", s.handleVehiclePositions)
					r.Get("/events", s.handleVehicleEvents)
					r.Get("/commands", s.handleVehicleCommands)

					// Comandos exigem perfil de operador ou admin.
					r.Group(func(r chi.Router) {
						r.Use(auth.RequireRole(auth.RoleAdmin, auth.RoleOperator))
						r.Post("/commands/engine-cut", s.handleEngineCut)
						r.Post("/commands/engine-resume", s.handleEngineResume)
						r.Post("/commands/request-position", s.handleRequestPosition)
						r.Post("/commands/request-status", s.handleRequestStatus)
						r.Post("/commands", s.handleGenericCommand)
					})
				})
			})

			r.Route("/devices", func(r chi.Router) {
				r.Get("/", s.handleListDevices)
				r.With(auth.RequireRole(auth.RoleAdmin)).Post("/", s.handleCreateDevice)

				r.Route("/{id}", func(r chi.Router) {
					r.Get("/", s.handleGetDevice)
					r.With(auth.RequireRole(auth.RoleAdmin)).Patch("/", s.handleUpdateDevice)
					r.With(auth.RequireRole(auth.RoleAdmin)).Delete("/", s.handleDeleteDevice)
					r.Get("/status", s.handleDeviceStatus)
					r.Get("/provisioning", s.handleDeviceProvisioning)
					r.Get("/commands", s.handleDeviceCommands)
				})
			})

			r.Route("/geofences", func(r chi.Router) {
				r.Get("/", s.handleListGeofences)
				r.With(auth.RequireRole(auth.RoleAdmin)).Post("/", s.handleCreateGeofence)
				r.With(auth.RequireRole(auth.RoleAdmin)).Patch("/{id}", s.handleUpdateGeofence)
				r.With(auth.RequireRole(auth.RoleAdmin)).Delete("/{id}", s.handleDeleteGeofence)
			})

			r.Get("/events", s.handleRecentEvents)

			r.Route("/users", func(r chi.Router) {
				r.Use(auth.RequireRole(auth.RoleAdmin))
				r.Get("/", s.handleListUsers)
				r.Post("/", s.handleCreateUser)
			})

			r.Route("/diagnostics", func(r chi.Router) {
				r.Use(auth.RequireRole(auth.RoleAdmin))
				r.Get("/connections", s.handleConnections)
				r.Get("/raw-packets", s.handleRawPackets)
				r.Get("/audit-logs", s.handleAuditLogs)
			})
		})
	})

	// O WebSocket autentica pelo mesmo middleware, lendo ?token= porque o
	// navegador não permite header na abertura da conexão.
	r.With(s.Auth.Middleware).Handle("/ws", s.WS)

	return r
}

// ListenAndServe sobe o servidor HTTP e encerra junto com o contexto.
func (s *Server) ListenAndServe(ctx context.Context) error {
	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", s.Config.HTTP.Port),
		Handler:           s.router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       s.Config.HTTP.ReadTimeout,
		WriteTimeout:      s.Config.HTTP.WriteTimeout,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		s.Log.Info("API HTTP no ar", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(
			context.WithoutCancel(ctx), s.Config.HTTP.ShutdownTimeout)
		defer cancel()
		s.Log.Info("encerrando API HTTP")
		return server.Shutdown(shutdownCtx)
	}
}
