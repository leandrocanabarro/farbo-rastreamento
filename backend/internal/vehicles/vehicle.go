// Package vehicles trata o cadastro dos veículos e o vínculo com o rastreador.
package vehicles

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/farbo/tracker-platform/backend/internal/database"
)

type Vehicle struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Plate string    `json:"plate"`
	Brand string    `json:"brand"`
	Model string    `json:"model"`
	Year  *int      `json:"year"`
	Color string    `json:"color"`

	// SpeedLimitKmh nulo faz o serviço usar o limite global (§21).
	SpeedLimitKmh *float64 `json:"speedLimitKmh"`

	DeviceID *uuid.UUID `json:"deviceId"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Input struct {
	Name          string     `json:"name"`
	Plate         string     `json:"plate"`
	Brand         string     `json:"brand"`
	Model         string     `json:"model"`
	Year          *int       `json:"year"`
	Color         string     `json:"color"`
	SpeedLimitKmh *float64   `json:"speedLimitKmh"`
	DeviceID      *uuid.UUID `json:"deviceId"`
}

const columns = `id, name, COALESCE(plate, ''), COALESCE(brand, ''), COALESCE(model, ''), year,
	COALESCE(color, ''), speed_limit_kmh, device_id, created_at, updated_at`

type Repository struct{ db *database.DB }

func NewRepository(db *database.DB) *Repository { return &Repository{db: db} }

func scan(row database.Scanner) (*Vehicle, error) {
	var v Vehicle
	err := row.Scan(&v.ID, &v.Name, &v.Plate, &v.Brand, &v.Model, &v.Year, &v.Color,
		&v.SpeedLimitKmh, &v.DeviceID, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, database.MapError(err)
	}
	return &v, nil
}

func (r *Repository) Create(ctx context.Context, in Input) (*Vehicle, error) {
	return scan(r.db.QueryRow(ctx, `
		INSERT INTO vehicles (name, plate, brand, model, year, color, speed_limit_kmh, device_id)
		VALUES ($1, NULLIF($2, ''), $3, $4, $5, $6, $7, $8)
		RETURNING `+columns,
		in.Name, strings.ToUpper(strings.TrimSpace(in.Plate)), in.Brand, in.Model,
		in.Year, in.Color, in.SpeedLimitKmh, in.DeviceID))
}

func (r *Repository) Update(ctx context.Context, id uuid.UUID, in Input) (*Vehicle, error) {
	return scan(r.db.QueryRow(ctx, `
		UPDATE vehicles SET name = $2, plate = NULLIF($3, ''), brand = $4, model = $5,
			year = $6, color = $7, speed_limit_kmh = $8, device_id = $9, updated_at = NOW()
		WHERE id = $1
		RETURNING `+columns,
		id, in.Name, strings.ToUpper(strings.TrimSpace(in.Plate)), in.Brand, in.Model,
		in.Year, in.Color, in.SpeedLimitKmh, in.DeviceID))
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM vehicles WHERE id = $1`, id)
	if err != nil {
		return database.MapError(err)
	}
	if tag.RowsAffected() == 0 {
		return database.ErrNotFound
	}
	return nil
}

func (r *Repository) GetByID(ctx context.Context, id uuid.UUID) (*Vehicle, error) {
	return scan(r.db.QueryRow(ctx, `SELECT `+columns+` FROM vehicles WHERE id = $1`, id))
}

func (r *Repository) GetByDeviceID(ctx context.Context, deviceID uuid.UUID) (*Vehicle, error) {
	return scan(r.db.QueryRow(ctx, `SELECT `+columns+` FROM vehicles WHERE device_id = $1`, deviceID))
}

func (r *Repository) List(ctx context.Context) ([]*Vehicle, error) {
	rows, err := r.db.Query(ctx, `SELECT `+columns+` FROM vehicles ORDER BY name`)
	if err != nil {
		return nil, database.MapError(err)
	}
	defer rows.Close()

	out := []*Vehicle{}
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

type Service struct{ repo *Repository }

func NewService(repo *Repository) *Service { return &Service{repo: repo} }

func (s *Service) List(ctx context.Context) ([]*Vehicle, error) { return s.repo.List(ctx) }

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Vehicle, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) GetByDeviceID(ctx context.Context, deviceID uuid.UUID) (*Vehicle, error) {
	return s.repo.GetByDeviceID(ctx, deviceID)
}

func (s *Service) Create(ctx context.Context, in Input) (*Vehicle, error) {
	if err := validate(in); err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, in)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, in Input) (*Vehicle, error) {
	if err := validate(in); err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, id, in)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error { return s.repo.Delete(ctx, id) }

// ValidationError descreve um payload recusado.
type ValidationError struct{ Message string }

func (e ValidationError) Error() string { return e.Message }

func validate(in Input) error {
	if strings.TrimSpace(in.Name) == "" {
		return ValidationError{Message: "o nome do veículo é obrigatório"}
	}
	if len(in.Name) > 100 {
		return ValidationError{Message: "nome do veículo longo demais"}
	}
	if in.Year != nil && (*in.Year < 1900 || *in.Year > time.Now().Year()+1) {
		return ValidationError{Message: fmt.Sprintf("ano fora da faixa 1900..%d", time.Now().Year()+1)}
	}
	if in.SpeedLimitKmh != nil && (*in.SpeedLimitKmh < 0 || *in.SpeedLimitKmh > 300) {
		return ValidationError{Message: "limite de velocidade fora da faixa 0..300 km/h"}
	}
	return nil
}
