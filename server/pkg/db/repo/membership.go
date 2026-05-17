package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OrgMembership struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	UserID         uuid.UUID
	SystemRole     string
	Status         string
	JoinedAt       *time.Time
}

type MembershipRepo struct {
	pool *pgxpool.Pool
}

func NewMembershipRepo(pool *pgxpool.Pool) *MembershipRepo {
	return &MembershipRepo{pool: pool}
}

func (r *MembershipRepo) Create(ctx context.Context, orgID, userID uuid.UUID, systemRole string) (*OrgMembership, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO org_membership (organization_id, user_id, system_role, status, joined_at)
		 VALUES ($1, $2, $3, 'active', NOW())
		 RETURNING id, organization_id, user_id, system_role, status, joined_at`,
		orgID, userID, systemRole)
	m := &OrgMembership{}
	err := row.Scan(&m.ID, &m.OrganizationID, &m.UserID, &m.SystemRole, &m.Status, &m.JoinedAt)
	return m, err
}

func (r *MembershipRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]OrgMembership, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, user_id, system_role, status, joined_at
		 FROM org_membership
		 WHERE user_id = $1 AND status = 'active'`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OrgMembership{}
	for rows.Next() {
		m := OrgMembership{}
		if err := rows.Scan(&m.ID, &m.OrganizationID, &m.UserID, &m.SystemRole,
			&m.Status, &m.JoinedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// HasMembership returns true when the user is an active member of the org.
func (r *MembershipRepo) HasMembership(ctx context.Context, userID, orgID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (
		   SELECT 1 FROM org_membership
		   WHERE user_id = $1 AND organization_id = $2 AND status = 'active'
		 )`, userID, orgID).Scan(&exists)
	return exists, err
}
