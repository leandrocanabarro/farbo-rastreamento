package tcp

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/farbo/tracker-platform/backend/internal/config"
	"github.com/farbo/tracker-platform/backend/internal/protocols"
	"github.com/farbo/tracker-platform/backend/internal/telemetry"
)

// ErrRejectSession sinaliza que a sessão deve ser encerrada (ex.: IMEI
// desconhecido). O ingestor devolve este erro embrulhado.
var ErrRejectSession = errors.New("sessão recusada")

// Ingestor recebe o resultado da decodificação. Implementado pelo domínio.
type Ingestor interface {
	// HandleMessage processa uma mensagem normalizada. Devolver um erro
	// embrulhando ErrRejectSession encerra a conexão.
	HandleMessage(ctx context.Context, conn *DeviceConnection, msg protocols.TrackerMessage) error
	// HandleInvalid registra tráfego que não pôde ser interpretado (§38).
	HandleInvalid(ctx context.Context, conn *DeviceConnection, payload []byte, protocolName, reason string)
	// HandleDisconnect avisa que a sessão terminou.
	HandleDisconnect(ctx context.Context, conn *DeviceConnection)
}

// Server aceita conexões de rastreadores em uma porta TCP.
type Server struct {
	cfg      config.TCP
	registry *protocols.ProtocolRegistry
	manager  *Manager
	ingestor Ingestor
	log      *slog.Logger
	metrics  *telemetry.Metrics

	listener net.Listener
	wg       sync.WaitGroup

	// sem limita o número de sessões simultâneas.
	sem chan struct{}
}

func NewServer(
	cfg config.TCP,
	registry *protocols.ProtocolRegistry,
	manager *Manager,
	ingestor Ingestor,
	log *slog.Logger,
	metrics *telemetry.Metrics,
) *Server {
	return &Server{
		cfg:      cfg,
		registry: registry,
		manager:  manager,
		ingestor: ingestor,
		log:      log.With("component", "tcp"),
		metrics:  metrics,
		sem:      make(chan struct{}, cfg.MaxConnections),
	}
}

func (s *Server) Addr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// ListenAndServe bloqueia até o contexto ser cancelado.
func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	lc := net.ListenConfig{KeepAlive: s.cfg.KeepAlive}
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	s.listener = ln
	s.log.Info("servidor TCP de rastreadores no ar", "addr", ln.Addr().String())

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				s.log.Info("encerrando servidor TCP, aguardando sessões")
				s.wg.Wait()
				return nil
			}
			s.log.Warn("falha no accept", "err", err)
			// Evita girar em falso num erro persistente de accept.
			time.Sleep(50 * time.Millisecond)
			continue
		}

		select {
		case s.sem <- struct{}{}:
		default:
			s.log.Warn("limite de conexões atingido, recusando",
				"remote", conn.RemoteAddr().String(), "limit", s.cfg.MaxConnections)
			_ = conn.Close()
			continue
		}

		s.wg.Add(1)
		go func() {
			defer func() {
				<-s.sem
				s.wg.Done()
			}()
			s.handle(ctx, conn)
		}()
	}
}

func (s *Server) handle(ctx context.Context, netConn net.Conn) {
	conn := newConnection(netConn, s.cfg.WriteTimeout)

	defer func() {
		// Uma sessão com pânico não pode derrubar as outras (§7).
		if r := recover(); r != nil {
			s.log.Error("pânico na sessão do rastreador",
				"remote", conn.RemoteAddr(), "imei", telemetry.IMEI(conn.IMEI()), "recover", r)
		}
		_ = conn.Close()
		s.manager.UnregisterConn(conn)
		s.ingestor.HandleDisconnect(context.WithoutCancel(ctx), conn)
		s.metrics.Connections.Set(float64(s.manager.Count()))
	}()

	if tcpConn, ok := netConn.(*net.TCPConn); ok {
		_ = tcpConn.SetKeepAlive(true)
		_ = tcpConn.SetKeepAlivePeriod(s.cfg.KeepAlive)
	}

	sess := &session{
		server: s,
		conn:   conn,
		log:    s.log.With("remote", conn.RemoteAddr(), "session", conn.ID.String()),
	}
	sess.run(ctx)
}
