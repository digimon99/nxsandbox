package apps

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Deployment struct {
	ID         string    `json:"id"`
	AppID      string    `json:"app_id"`
	Version    string    `json:"version"`
	BinaryPath string    `json:"binary_path"`
	Checksum   string    `json:"checksum"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) CreateApp(ctx context.Context, userID, name, slug string) (*App, error) {
	var a App
	err := s.pool.QueryRow(ctx, `
		INSERT INTO apps(user_id, name, slug)
		VALUES ($1::uuid, $2, $3)
		RETURNING id::text, user_id::text, name, slug, status, created_at, updated_at
	`, userID, name, slug).Scan(&a.ID, &a.UserID, &a.Name, &a.Slug, &a.Status, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Store) ListAppsByUser(ctx context.Context, userID string) ([]App, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, user_id::text, name, slug, status, created_at, updated_at
		FROM apps
		WHERE user_id = $1::uuid
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	apps := make([]App, 0)
	for rows.Next() {
		var a App
		if err := rows.Scan(&a.ID, &a.UserID, &a.Name, &a.Slug, &a.Status, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		apps = append(apps, a)
	}
	return apps, rows.Err()
}

func (s *Store) ListDeploymentsByApp(ctx context.Context, appID string) ([]Deployment, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, app_id::text, version, binary_path, checksum, status, created_at
		FROM deployments
		WHERE app_id = $1::uuid
		ORDER BY created_at DESC
	`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Deployment, 0)
	for rows.Next() {
		var d Deployment
		if err := rows.Scan(&d.ID, &d.AppID, &d.Version, &d.BinaryPath, &d.Checksum, &d.Status, &d.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	return items, rows.Err()
}
