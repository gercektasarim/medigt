package repo

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Session struct {
	ID                uuid.UUID
	UserID            uuid.UUID
	RefreshTokenHash  string
	UserAgent         *string
	ExpiresAt         time.Time
	RevokedAt         *time.Time
	CreatedAt         time.Time
}

type SessionRepo struct {
	pool *pgxpool.Pool
}

func NewSessionRepo(pool *pgxpool.Pool) *SessionRepo {
	return &SessionRepo{pool: pool}
}

// sanitizeIP strips :port (and IPv6 brackets) from net/http RemoteAddr so the
// remainder is a parseable inet literal; returns "" when nothing valid is left.
func sanitizeIP(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(addr); err == nil {
		addr = host
	}
	addr = strings.Trim(addr, "[]")
	if net.ParseIP(addr) == nil {
		return ""
	}
	return addr
}

func (r *SessionRepo) Create(ctx context.Context, userID uuid.UUID, refreshHash string,
	userAgent string, ip string, ttl time.Duration) (*Session, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO user_session (user_id, refresh_token_hash, user_agent, ip_address, expires_at)
		 VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, '')::inet, NOW() + make_interval(secs => $5))
		 RETURNING id, user_id, refresh_token_hash, user_agent, expires_at, revoked_at, created_at`,
		userID, refreshHash, userAgent, sanitizeIP(ip), ttl.Seconds())
	s := &Session{}
	err := row.Scan(&s.ID, &s.UserID, &s.RefreshTokenHash, &s.UserAgent,
		&s.ExpiresAt, &s.RevokedAt, &s.CreatedAt)
	return s, err
}

func (r *SessionRepo) Revoke(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE user_session SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`, id)
	return err
}

func (r *SessionRepo) GetByRefreshHash(ctx context.Context, refreshHash string) (*Session, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, user_id, refresh_token_hash, user_agent, expires_at, revoked_at, created_at
		 FROM user_session
		 WHERE refresh_token_hash = $1 AND revoked_at IS NULL AND expires_at > NOW()`,
		refreshHash)
	s := &Session{}
	err := row.Scan(&s.ID, &s.UserID, &s.RefreshTokenHash, &s.UserAgent,
		&s.ExpiresAt, &s.RevokedAt, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}
