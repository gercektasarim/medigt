package repo

import (
	"context"
	"encoding/json"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AuditEntry mirrors one row of the audit_log table.
//
// KVKK rules: details JSONB may include only structural metadata + ID
// references. NEVER full TC, names, phone, address, etc. Use util.MaskTC
// for any identifier you must reference.
type AuditEntry struct {
	ID             int64
	OrganizationID *uuid.UUID
	BranchID       *uuid.UUID
	ActorUserID    *uuid.UUID
	ActorSessionID *uuid.UUID
	Action         string
	EntityType     string
	EntityID       *string
	Details        json.RawMessage
	IPAddress      *string
	UserAgent      *string
	CreatedAt      time.Time
	// Joined fields (populated by List with actor enrichment):
	ActorEmail *string
	ActorName  *string
}

type AuditRepo struct{ pool *pgxpool.Pool }

func NewAuditRepo(pool *pgxpool.Pool) *AuditRepo { return &AuditRepo{pool: pool} }

// WriteInput is the structural payload for a single audit row. Most fields
// are optional; only Action + EntityType are required (CHECK NOT NULL).
type WriteInput struct {
	OrganizationID *uuid.UUID
	BranchID       *uuid.UUID
	ActorUserID    *uuid.UUID
	ActorSessionID *uuid.UUID
	Action         string
	EntityType     string
	EntityID       *string
	Details        any // marshalled to JSONB; nil -> "{}"
	IPAddress      string
	UserAgent      string
}

func (r *AuditRepo) Write(ctx context.Context, in WriteInput) error {
	var details []byte
	if in.Details != nil {
		b, err := json.Marshal(in.Details)
		if err != nil {
			return err
		}
		details = b
	} else {
		details = []byte("{}")
	}
	var ip *string
	if in.IPAddress != "" {
		if host, _, err := net.SplitHostPort(in.IPAddress); err == nil {
			ip = &host
		} else {
			ip = &in.IPAddress
		}
	}
	var ua *string
	if in.UserAgent != "" {
		ua = &in.UserAgent
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO audit_log
		   (organization_id, branch_id, actor_user_id, actor_session_id,
		    action, entity_type, entity_id, details, ip_address, user_agent)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8::JSONB, $9::INET, $10)`,
		in.OrganizationID, in.BranchID, in.ActorUserID, in.ActorSessionID,
		in.Action, in.EntityType, in.EntityID, details, ip, ua)
	return err
}

// ListFilter scopes a paginated audit_log query. organization_id is
// always enforced for tenant isolation. Other fields are optional — empty
// values mean "no filter".
type ListFilter struct {
	OrganizationID uuid.UUID
	BranchID       *uuid.UUID
	ActorUserID    *uuid.UUID
	Action         string
	EntityType     string
	EntityID       string
	From           *time.Time
	To             *time.Time
	Limit          int
	Offset         int
}

func (r *AuditRepo) List(ctx context.Context, f ListFilter) ([]AuditEntry, int, error) {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}

	// Build WHERE dynamically. We always start at $1 = orgID.
	where := "WHERE al.organization_id = $1"
	args := []any{f.OrganizationID}
	idx := 2
	if f.BranchID != nil {
		where += " AND al.branch_id = $" + itoa(idx)
		args = append(args, *f.BranchID)
		idx++
	}
	if f.ActorUserID != nil {
		where += " AND al.actor_user_id = $" + itoa(idx)
		args = append(args, *f.ActorUserID)
		idx++
	}
	if f.Action != "" {
		where += " AND al.action = $" + itoa(idx)
		args = append(args, f.Action)
		idx++
	}
	if f.EntityType != "" {
		where += " AND al.entity_type = $" + itoa(idx)
		args = append(args, f.EntityType)
		idx++
	}
	if f.EntityID != "" {
		where += " AND al.entity_id = $" + itoa(idx)
		args = append(args, f.EntityID)
		idx++
	}
	if f.From != nil {
		where += " AND al.created_at >= $" + itoa(idx)
		args = append(args, *f.From)
		idx++
	}
	if f.To != nil {
		where += " AND al.created_at < $" + itoa(idx)
		args = append(args, *f.To)
		idx++
	}

	// Total count (filters only) for pagination UI.
	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM audit_log al "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limitIdx := itoa(idx)
	offsetIdx := itoa(idx + 1)
	args = append(args, f.Limit, f.Offset)

	rows, err := r.pool.Query(ctx,
		`SELECT al.id, al.organization_id, al.branch_id, al.actor_user_id, al.actor_session_id,
		        al.action, al.entity_type, al.entity_id, al.details,
		        al.ip_address::text, al.user_agent, al.created_at,
		        u.email, NULLIF(u.name, '')
		 FROM audit_log al
		 LEFT JOIN app_user u ON u.id = al.actor_user_id
		 `+where+`
		 ORDER BY al.created_at DESC
		 LIMIT $`+limitIdx+` OFFSET $`+offsetIdx, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []AuditEntry{}
	for rows.Next() {
		e := AuditEntry{}
		if err := rows.Scan(&e.ID, &e.OrganizationID, &e.BranchID, &e.ActorUserID, &e.ActorSessionID,
			&e.Action, &e.EntityType, &e.EntityID, &e.Details,
			&e.IPAddress, &e.UserAgent, &e.CreatedAt,
			&e.ActorEmail, &e.ActorName); err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}

// DistinctActions lists every action string used in the org's audit log,
// for the filter dropdown in the UI.
func (r *AuditRepo) DistinctActions(ctx context.Context, orgID uuid.UUID) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT action FROM audit_log
		 WHERE organization_id = $1
		 ORDER BY action`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// DistinctEntityTypes — same idea, scoped per org.
func (r *AuditRepo) DistinctEntityTypes(ctx context.Context, orgID uuid.UUID) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT entity_type FROM audit_log
		 WHERE organization_id = $1
		 ORDER BY entity_type`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

