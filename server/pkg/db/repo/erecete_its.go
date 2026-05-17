package repo

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErecekItsRepo bundles the prescription + prescription_dispense extension
// updates needed by the e-Reçete and İTS outbox workers. Lives alongside
// MedulaRepo because it shares the same outbox table; the worker dispatches
// based on message_type.
//
// Method naming follows the pattern Complete*Success / Complete*Failure /
// Complete*Cancellation in medula_extended.go.

// ---------- e-Reçete (prescription) ----------

// PrescriptionEreceteContext is the small slice of prescription + visit data
// the worker needs to call the e-Reçete client.
type PrescriptionEreceteContext struct {
	OrgID          uuid.UUID
	PrescriptionNo string
	DoctorTC       string
	PatientTC      string
	DiagnosesICD10 []string
	DrugATCCodes   []string
}

// LoadEreceteContext fetches everything the e-Reçete client needs in one query.
func (r *MedulaRepo) LoadEreceteContext(ctx context.Context, prescriptionID uuid.UUID) (*PrescriptionEreceteContext, error) {
	rxRow := r.pool.QueryRow(ctx,
		`SELECT p.organization_id, p.prescription_no,
		        COALESCE(dr.medula_doctor_code, ''),
		        COALESCE(pat.identifier_value, '')
		 FROM prescription p
		 JOIN patient pat ON pat.id = p.patient_id
		 LEFT JOIN doctor dr ON dr.id = p.doctor_id
		 WHERE p.id = $1`, prescriptionID)
	out := &PrescriptionEreceteContext{}
	if err := rxRow.Scan(&out.OrgID, &out.PrescriptionNo, &out.DoctorTC, &out.PatientTC); err != nil {
		return nil, err
	}
	// Diagnoses on the same visit (rough proxy for "this rx's diagnoses").
	var visitID uuid.UUID
	_ = r.pool.QueryRow(ctx, `SELECT visit_id FROM prescription WHERE id = $1`, prescriptionID).Scan(&visitID)
	if visitID != uuid.Nil {
		rows, err := r.pool.Query(ctx,
			`SELECT icd10_code FROM diagnosis WHERE visit_id = $1`, visitID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var c string
				if err := rows.Scan(&c); err == nil {
					out.DiagnosesICD10 = append(out.DiagnosesICD10, c)
				}
			}
		}
	}
	// Drug ATC codes from the prescription_item rows (medication catalog).
	atcRows, err := r.pool.Query(ctx,
		`SELECT m.atc_code FROM prescription_item pi
		 LEFT JOIN medication m ON m.id = pi.medication_id
		 WHERE pi.prescription_id = $1 AND m.atc_code IS NOT NULL`, prescriptionID)
	if err == nil {
		defer atcRows.Close()
		for atcRows.Next() {
			var c string
			if err := atcRows.Scan(&c); err == nil && c != "" {
				out.DrugATCCodes = append(out.DrugATCCodes, c)
			}
		}
	}
	return out, nil
}

// CompleteEreceteSubmitSuccess marks outbox sent + prescription submitted.
func (r *MedulaRepo) CompleteEreceteSubmitSuccess(ctx context.Context, msgID, prescriptionID uuid.UUID, ePrescriptionNo, responseCode string, payload map[string]any) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = r.markOutboxSentTx(ctx, tx, msgID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx,
		`UPDATE prescription
		   SET e_prescription_no = $2,
		       e_prescription_status = 'submitted',
		       e_prescription_submitted_at = NOW(),
		       e_prescription_response = $3::JSONB,
		       e_prescription_error = NULL
		 WHERE id = $1`,
		prescriptionID, ePrescriptionNo, mustMarshal(payload)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *MedulaRepo) CompleteEreceteSubmitFailure(ctx context.Context, msgID, prescriptionID uuid.UUID, responseCode, errorMsg string, retryCount int) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	finalStatus, err := r.markOutboxFailedTx(ctx, tx, msgID, errorMsg, retryCount)
	if err != nil {
		return err
	}
	if finalStatus == "dead" {
		if _, err = tx.Exec(ctx,
			`UPDATE prescription
			   SET e_prescription_status = 'failed', e_prescription_error = $2
			 WHERE id = $1`, prescriptionID, errorMsg); err != nil {
			return err
		}
	} else {
		// Stay in 'in_progress' for retries (so UI distinguishes from terminal).
		_, _ = tx.Exec(ctx,
			`UPDATE prescription SET e_prescription_status = 'in_progress',
			     e_prescription_error = $2
			 WHERE id = $1 AND e_prescription_status NOT IN ('submitted','cancelled','failed')`,
			prescriptionID, errorMsg)
	}
	return tx.Commit(ctx)
}

// ---------- İTS (prescription_dispense) ----------

type DispenseItsContext struct {
	Karekod      string
	PatientTC    string
	Quantity     float64
	PharmacistTC string
}

func (r *MedulaRepo) LoadItsContext(ctx context.Context, dispenseID uuid.UUID) (*DispenseItsContext, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT COALESCE(d.karekod, d.lot_no, ''),
		        COALESCE(p.identifier_value, ''),
		        d.quantity,
		        ''
		 FROM prescription_dispense d
		 JOIN prescription_item it ON it.id = d.prescription_item_id
		 JOIN prescription rx ON rx.id = it.prescription_id
		 JOIN patient p ON p.id = rx.patient_id
		 WHERE d.id = $1`, dispenseID)
	out := &DispenseItsContext{}
	if err := row.Scan(&out.Karekod, &out.PatientTC, &out.Quantity, &out.PharmacistTC); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *MedulaRepo) CompleteItsNotifySuccess(ctx context.Context, msgID, dispenseID uuid.UUID, responseCode string, payload map[string]any) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = r.markOutboxSentTx(ctx, tx, msgID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx,
		`UPDATE prescription_dispense
		   SET its_status = 'notified',
		       its_notified_at = NOW(),
		       its_response = $2::JSONB,
		       its_error = NULL
		 WHERE id = $1`,
		dispenseID, mustMarshal(payload)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *MedulaRepo) CompleteItsNotifyFailure(ctx context.Context, msgID, dispenseID uuid.UUID, responseCode, errorMsg string, retryCount int) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	finalStatus, err := r.markOutboxFailedTx(ctx, tx, msgID, errorMsg, retryCount)
	if err != nil {
		return err
	}
	if finalStatus == "dead" {
		if _, err = tx.Exec(ctx,
			`UPDATE prescription_dispense
			   SET its_status = 'failed', its_error = $2
			 WHERE id = $1`, dispenseID, errorMsg); err != nil {
			return err
		}
	} else {
		_, _ = tx.Exec(ctx,
			`UPDATE prescription_dispense SET its_status = 'in_progress', its_error = $2
			 WHERE id = $1 AND its_status NOT IN ('notified','failed')`,
			dispenseID, errorMsg)
	}
	return tx.Commit(ctx)
}
