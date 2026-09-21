// Package auth trata autenticação (JWT + refresh token) e autorização (RBAC).
package auth

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/farbo/tracker-platform/backend/internal/database"
)

// Perfis de acesso (RBAC, §27).
const (
	RoleAdmin    = "admin"    // tudo, inclusive cadastro e usuários
	RoleOperator = "operator" // opera o painel e envia comandos
	RoleViewer   = "viewer"   // somente leitura
)

func ValidRole(role string) bool {
	switch role {
	case RoleAdmin, RoleOperator, RoleViewer:
		return true
	}
	return false
}

type User struct {
	ID    uuid.UUID `json:"id"`
	Email string    `json:"email"`
	Name  string    `json:"name"`
	Role  string    `json:"role"`

	// PasswordHash nunca sai em JSON.
	PasswordHash string `json:"-"`

	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

const userColumns = `id, email, name, role, password_hash, active, created_at, updated_at`

type Repository struct{ db *database.DB }

func NewRepository(db *database.DB) *Repository { return &Repository{db: db} }

func scanUser(row database.Scanner) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.PasswordHash,
		&u.Active, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, database.MapError(err)
	}
	return &u, nil
}

func (r *Repository) Create(ctx context.Context, u *User) error {
	return database.MapError(r.db.QueryRow(ctx, `
		INSERT INTO users (email, name, role, password_hash)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at`,
		strings.TrimSpace(u.Email), u.Name, u.Role, u.PasswordHash,
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt))
}

func (r *Repository) GetByEmail(ctx context.Context, email string) (*User, error) {
	return scanUser(r.db.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE lower(email) = lower($1)`, strings.TrimSpace(email)))
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	return scanUser(r.db.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id))
}

func (r *Repository) List(ctx context.Context) ([]*User, error) {
	rows, err := r.db.Query(ctx, `SELECT `+userColumns+` FROM users ORDER BY created_at`)
	if err != nil {
		return nil, database.MapError(err)
	}
	defer rows.Close()

	out := []*User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (r *Repository) Count(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n)
	return n, database.MapError(err)
}

// ---------------------------------------------------------------------------
// Refresh tokens
// ---------------------------------------------------------------------------

// StoreRefreshToken guarda apenas o hash: o valor em claro fica só no cliente.
func (r *Repository) StoreRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time, userAgent string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at, user_agent)
		VALUES ($1, $2, $3, $4)`, userID, tokenHash, expiresAt, userAgent)
	return database.MapError(err)
}

type refreshRecord struct {
	ID     uuid.UUID
	UserID uuid.UUID
}

// ConsumeRefreshToken valida e revoga o token numa única operação, de modo que
// um refresh token só possa ser usado uma vez (rotação).
func (r *Repository) ConsumeRefreshToken(ctx context.Context, tokenHash string) (*refreshRecord, error) {
	var rec refreshRecord
	err := r.db.QueryRow(ctx, `
		UPDATE refresh_tokens SET revoked_at = NOW()
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > NOW()
		RETURNING id, user_id`, tokenHash).Scan(&rec.ID, &rec.UserID)
	if err != nil {
		return nil, database.MapError(err)
	}
	return &rec, nil
}

func (r *Repository) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = NOW() WHERE token_hash = $1 AND revoked_at IS NULL`,
		tokenHash)
	return database.MapError(err)
}

func (r *Repository) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL`,
		userID)
	return database.MapError(err)
}

// DeleteExpiredRefreshTokens limpa a tabela periodicamente.
func (r *Repository) DeleteExpiredRefreshTokens(ctx context.Context) (int64, error) {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM refresh_tokens WHERE expires_at < NOW() - INTERVAL '30 days'`)
	if err != nil {
		return 0, database.MapError(err)
	}
	return tag.RowsAffected(), nil
}
