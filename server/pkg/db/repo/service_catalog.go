package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ServiceCatalog struct {
	ID              uuid.UUID
	OrganizationID  uuid.UUID
	Code            string
	SutCode         *string
	Name            string
	Category        string
	Description     *string
	Unit            string
	VatRate         float64
	BasePrice       *float64
	RequiresDoctor  bool
	IsActive        bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type ServiceCatalogRepo struct {
	pool *pgxpool.Pool
}

func NewServiceCatalogRepo(pool *pgxpool.Pool) *ServiceCatalogRepo {
	return &ServiceCatalogRepo{pool: pool}
}

const svcCols = `id, organization_id, code, sut_code, name, category, description,
	unit, vat_rate, base_price, requires_doctor, is_active, created_at, updated_at`

func scanService(row pgx.Row) (*ServiceCatalog, error) {
	s := &ServiceCatalog{}
	err := row.Scan(&s.ID, &s.OrganizationID, &s.Code, &s.SutCode, &s.Name, &s.Category,
		&s.Description, &s.Unit, &s.VatRate, &s.BasePrice, &s.RequiresDoctor,
		&s.IsActive, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

type CreateServiceInput struct {
	OrganizationID uuid.UUID
	Code           string
	SutCode        *string
	Name           string
	Category       string
	Description    *string
	Unit           string
	VatRate        float64
	BasePrice      *float64
	RequiresDoctor bool
}

func (r *ServiceCatalogRepo) Create(ctx context.Context, in CreateServiceInput) (*ServiceCatalog, error) {
	if in.Unit == "" {
		in.Unit = "adet"
	}
	row := r.pool.QueryRow(ctx,
		`INSERT INTO service_catalog (organization_id, code, sut_code, name, category,
		   description, unit, vat_rate, base_price, requires_doctor)
		 VALUES ($1,$2,$3,$4,$5::service_category,$6,$7,$8,$9,$10)
		 RETURNING `+svcCols,
		in.OrganizationID, in.Code, in.SutCode, in.Name, in.Category,
		in.Description, in.Unit, in.VatRate, in.BasePrice, in.RequiresDoctor)
	return scanService(row)
}

type ListServiceFilter struct {
	Category   string
	ActiveOnly bool
	Search     string
}

func (r *ServiceCatalogRepo) List(ctx context.Context, orgID uuid.UUID, f ListServiceFilter) ([]ServiceCatalog, error) {
	q := `SELECT ` + svcCols + ` FROM service_catalog WHERE organization_id = $1`
	args := []any{orgID}
	if f.ActiveOnly {
		q += ` AND is_active = TRUE`
	}
	if f.Category != "" {
		args = append(args, f.Category)
		q += ` AND category = $` + itoa(len(args)) + `::service_category`
	}
	if f.Search != "" {
		args = append(args, "%"+f.Search+"%")
		q += ` AND (name ILIKE $` + itoa(len(args)) + ` OR code ILIKE $` + itoa(len(args)) + `)`
	}
	q += ` ORDER BY category, name LIMIT 500`
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ServiceCatalog{}
	for rows.Next() {
		s := ServiceCatalog{}
		if err := rows.Scan(&s.ID, &s.OrganizationID, &s.Code, &s.SutCode, &s.Name, &s.Category,
			&s.Description, &s.Unit, &s.VatRate, &s.BasePrice, &s.RequiresDoctor,
			&s.IsActive, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *ServiceCatalogRepo) GetByID(ctx context.Context, orgID, id uuid.UUID) (*ServiceCatalog, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+svcCols+` FROM service_catalog WHERE organization_id = $1 AND id = $2`,
		orgID, id)
	return scanService(row)
}

// Tiny strconv-free itoa for parameter index assembly (avoids extra import).
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	buf := [20]byte{}
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
