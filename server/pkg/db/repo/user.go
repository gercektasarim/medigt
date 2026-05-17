package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID               uuid.UUID
	Email            string
	Name             string
	Phone            *string
	AvatarURL        *string
	IsActive         bool
	TotpEnabled      bool
	LastLoginAt      *time.Time
	FailedLoginCount int
	LockedUntil      *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

var ErrNotFound = errors.New("not found")

type UserRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

const userSelectCols = `id, email, name, phone, avatar_url, is_active, totp_enabled,
	last_login_at, failed_login_count, locked_until, created_at, updated_at`

func scanUser(row pgx.Row) (*User, error) {
	u := &User{}
	err := row.Scan(&u.ID, &u.Email, &u.Name, &u.Phone, &u.AvatarURL, &u.IsActive,
		&u.TotpEnabled, &u.LastLoginAt, &u.FailedLoginCount, &u.LockedUntil,
		&u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*User, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+userSelectCols+` FROM app_user WHERE lower(email) = lower($1)`,
		email)
	return scanUser(row)
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+userSelectCols+` FROM app_user WHERE id = $1`, id)
	return scanUser(row)
}

// CreateForEmail inserts a passive user (no password yet) used by email-code login.
// The name defaults to the email local-part until the user updates their profile.
func (r *UserRepo) CreateForEmail(ctx context.Context, email, name string) (*User, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO app_user (email, name) VALUES ($1, $2)
		 RETURNING `+userSelectCols, email, name)
	return scanUser(row)
}

func (r *UserRepo) TouchLastLogin(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE app_user SET last_login_at = NOW(), failed_login_count = 0
		 WHERE id = $1`, id)
	return err
}

func (r *UserRepo) UpdateName(ctx context.Context, id uuid.UUID, name string) error {
	_, err := r.pool.Exec(ctx, `UPDATE app_user SET name = $2 WHERE id = $1`, id, name)
	return err
}
