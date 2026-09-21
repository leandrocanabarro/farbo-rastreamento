// Package database cuida da conexão com o PostgreSQL e da aplicação de migrations.
package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/farbo/tracker-platform/backend/internal/config"
	"github.com/farbo/tracker-platform/backend/migrations"
)

var (
	// ErrNotFound é devolvido quando a consulta não encontra o registro.
	ErrNotFound = errors.New("registro não encontrado")
	// ErrConflict é devolvido em violação de unicidade.
	ErrConflict = errors.New("registro já existe")
	// ErrForeignKey é devolvido em violação de chave estrangeira.
	ErrForeignKey = errors.New("referência inválida")
)

type DB struct {
	*pgxpool.Pool
}

func Connect(ctx context.Context, cfg config.Postgres) (*DB, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("dsn inválido: %w", err)
	}
	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MaxConnLifetime = time.Hour
	poolCfg.MaxConnIdleTime = 30 * time.Minute

	// Garante que uuid.UUID seja enviado como uuid mesmo quando o pgx não
	// consegue inferir o OID do parâmetro.
	poolCfg.AfterConnect = func(_ context.Context, conn *pgx.Conn) error {
		m := conn.TypeMap()
		m.RegisterDefaultPgType(uuid.UUID{}, "uuid")
		m.RegisterDefaultPgType(&uuid.UUID{}, "uuid")
		m.RegisterDefaultPgType([]uuid.UUID{}, "_uuid")
		return nil
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("criando pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres indisponível: %w", err)
	}
	return &DB{Pool: pool}, nil
}

// Migrate aplica os arquivos SQL embutidos, em ordem lexicográfica, uma vez cada.
func (db *DB) Migrate(ctx context.Context, log *slog.Logger) error {
	if _, err := db.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`); err != nil {
		return fmt.Errorf("criando schema_migrations: %w", err)
	}

	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		var applied bool
		if err := db.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, name,
		).Scan(&applied); err != nil {
			return err
		}
		if applied {
			continue
		}

		body, err := migrations.FS.ReadFile(name)
		if err != nil {
			return err
		}

		tx, err := db.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(body)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migration %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, name); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		log.Info("migration aplicada", "version", name)
	}
	return nil
}

// MapError traduz erros do driver para os sentinelas do pacote.
func MapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return ErrConflict
		case "23503":
			return ErrForeignKey
		}
	}
	return err
}

// Scanner abstrai pgx.Row e pgx.Rows para reaproveitar funções de scan.
type Scanner interface {
	Scan(dest ...any) error
}
