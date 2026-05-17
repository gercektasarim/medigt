package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/medigt/medigt/server/internal/integration/hl7"
	"github.com/medigt/medigt/server/pkg/db/repo"
)

// LabHL7Service ingests parsed HL7 ORU^R01 messages from lab autoanalizör
// middleware and writes results onto the matching lab_order_item rows.
//
// Lookup strategy:
//
//   1. Resolve the lab_order by placer_order_no (preferred — HBYS-generated)
//      or fallback to filler_order_no (analyzer's accession).
//   2. For each observation, match (lab_order_id, test_code) to a single
//      lab_order_item; unknown codes are ignored but logged in the result.
//   3. Update via existing LabRepo.UpdateItemResult (status='resulted', flip
//      parent order to 'resulted' atomically).
//
// Vendor noise (RE-routed messages, duplicate sends) is tolerated:
// already-resulted items get overwritten (caller can opt-in to refuse —
// not implemented here).
type LabHL7Service struct {
	pool *pgxpool.Pool
	lab  *repo.LabRepo
}

func NewLabHL7Service(pool *pgxpool.Pool, lab *repo.LabRepo) *LabHL7Service {
	return &LabHL7Service{pool: pool, lab: lab}
}

// IngestResult holds the per-message ingest outcome (used for HTTP response
// + ops dashboards).
type IngestResult struct {
	MessageControlID string             `json:"message_control_id"`
	OrderID          *uuid.UUID         `json:"order_id,omitempty"`
	OrderNo          string             `json:"order_no,omitempty"`
	Matched          int                `json:"matched_observations"`
	Unmatched        []UnmatchedRow     `json:"unmatched_observations,omitempty"`
	Errors           []string           `json:"errors,omitempty"`
}

type UnmatchedRow struct {
	TestCode string `json:"test_code"`
	Reason   string `json:"reason"`
}

var ErrOrderNotFound = errors.New("HL7 mesajının order'ı bulunamadı")

// Ingest looks up the order, walks observations, and applies each result.
// branchID is required so cross-tenant ingest is impossible (an analyzer
// shouldn't be able to write into another hospital's orders).
func (s *LabHL7Service) Ingest(ctx context.Context, branchID uuid.UUID, msg *hl7.Message) (*IngestResult, error) {
	out := &IngestResult{MessageControlID: msg.MessageControlID}

	orderID, orderNo, err := s.resolveOrder(ctx, branchID, msg)
	if err != nil {
		return out, err
	}
	out.OrderID = &orderID
	out.OrderNo = orderNo

	// Load order items so we can match test_code → lab_order_item.id
	// without N round-trips per observation.
	rows, err := s.pool.Query(ctx,
		`SELECT id, test_code FROM lab_order_item WHERE lab_order_id = $1`, orderID)
	if err != nil {
		return out, fmt.Errorf("lab_order_item yüklenemedi: %w", err)
	}
	itemsByCode := map[string]uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		var code string
		if err := rows.Scan(&id, &code); err != nil {
			rows.Close()
			return out, err
		}
		itemsByCode[strings.ToUpper(strings.TrimSpace(code))] = id
	}
	rows.Close()

	for _, obs := range msg.Observations {
		code := strings.ToUpper(strings.TrimSpace(obs.TestCode))
		if code == "" {
			out.Unmatched = append(out.Unmatched, UnmatchedRow{TestCode: "", Reason: "OBX-3 boş"})
			continue
		}
		itemID, ok := itemsByCode[code]
		if !ok {
			out.Unmatched = append(out.Unmatched, UnmatchedRow{
				TestCode: code,
				Reason:   "bu order'da bu test yok",
			})
			continue
		}
		in := repo.UpdateItemResultInput{
			Flag: mapHL7Flag(obs.AbnormalFlag),
		}
		if obs.NumericValue != nil {
			in.ValueNumeric = obs.NumericValue
		}
		if v := strings.TrimSpace(obs.Value); v != "" {
			in.ValueText = &v
		}
		if _, err := s.lab.UpdateItemResult(ctx, itemID, in); err != nil {
			out.Errors = append(out.Errors, fmt.Sprintf("%s: %v", code, err))
			continue
		}
		out.Matched++
	}
	return out, nil
}

// resolveOrder finds the lab_order using placer_order_no first, then
// filler. branchID enforces tenant isolation.
func (s *LabHL7Service) resolveOrder(ctx context.Context, branchID uuid.UUID, msg *hl7.Message) (uuid.UUID, string, error) {
	tryNo := func(orderNo string) (uuid.UUID, error) {
		orderNo = strings.TrimSpace(orderNo)
		if orderNo == "" {
			return uuid.Nil, pgx.ErrNoRows
		}
		var id uuid.UUID
		err := s.pool.QueryRow(ctx,
			`SELECT id FROM lab_order WHERE branch_id = $1 AND order_no = $2`,
			branchID, orderNo).Scan(&id)
		return id, err
	}
	if id, err := tryNo(msg.PlacerOrderNo); err == nil {
		return id, msg.PlacerOrderNo, nil
	}
	if id, err := tryNo(msg.FillerOrderNo); err == nil {
		return id, msg.FillerOrderNo, nil
	}
	return uuid.Nil, "", ErrOrderNotFound
}

// mapHL7Flag converts HL7 abnormal flags (Table 0078) to our lab_result_flag
// enum values. Unknown flags map to nil (no flag).
func mapHL7Flag(hl7Flag string) *string {
	hl7Flag = strings.ToUpper(strings.TrimSpace(hl7Flag))
	switch hl7Flag {
	case "":
		return nil
	case "N":
		v := "normal"
		return &v
	case "L":
		v := "low"
		return &v
	case "H":
		v := "high"
		return &v
	case "LL":
		v := "critical_low"
		return &v
	case "HH":
		v := "critical_high"
		return &v
	case "A", "AA":
		v := "abnormal"
		return &v
	}
	return nil
}
