package medula

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/medigt/medigt/server/internal/integration/erecete"
	"github.com/medigt/medigt/server/internal/integration/its"
	"github.com/medigt/medigt/server/pkg/db/repo"
)

// OutboxWorker drains medula_outgoing_message and dispatches each row
// to the appropriate Client (Medula, e-Reçete or İTS). The dispatch
// table grows whenever a new outbox message_type is introduced —
// every new message_type needs (1) a case here, (2) a matching
// Complete* pair in MedulaRepo (or sibling repo files).
//
// Worker is horizontally safe: ClaimNext uses FOR UPDATE SKIP LOCKED so
// multiple instances can run on different nodes.
type OutboxWorker struct {
	pool          *pgxpool.Pool
	medula        *repo.MedulaRepo
	client        Client
	ereceteClient erecete.Client
	itsClient     its.Client
	log           *slog.Logger
	pollGap       time.Duration
}

func NewOutboxWorker(
	pool *pgxpool.Pool,
	medula *repo.MedulaRepo,
	client Client,
	ereceteClient erecete.Client,
	itsClient its.Client,
	log *slog.Logger,
) *OutboxWorker {
	return &OutboxWorker{
		pool:          pool,
		medula:        medula,
		client:        client,
		ereceteClient: ereceteClient,
		itsClient:     itsClient,
		log:           log,
		pollGap:       2 * time.Second,
	}
}

func (w *OutboxWorker) Run(ctx context.Context) {
	w.log.Info("medula outbox worker starting")
	for {
		select {
		case <-ctx.Done():
			w.log.Info("medula outbox worker stopping")
			return
		default:
		}
		processed, err := w.pollOnce(ctx)
		if err != nil {
			w.log.Error("medula outbox poll failed", "err", err)
		}
		if !processed {
			select {
			case <-ctx.Done():
				return
			case <-time.After(w.pollGap):
			}
		}
	}
}

func (w *OutboxWorker) pollOnce(ctx context.Context) (bool, error) {
	msg, err := w.medula.ClaimNext(ctx)
	if err != nil {
		return false, err
	}
	if msg == nil {
		return false, nil
	}
	switch msg.MessageType {
	case "provision_request":
		return true, w.handleProvision(ctx, msg)
	case "provision_cancel":
		return true, w.handleProvisionCancel(ctx, msg)
	case "takip_close":
		return true, w.handleTakipClose(ctx, msg)
	case "invoice_submit":
		return true, w.handleInvoiceSubmit(ctx, msg)
	case "invoice_cancel":
		return true, w.handleInvoiceCancel(ctx, msg)
	case "referral_create":
		return true, w.handleReferralCreate(ctx, msg)
	case "eraport_submit":
		return true, w.handleEraportSubmit(ctx, msg)
	case "eraport_cancel":
		return true, w.handleEraportCancel(ctx, msg)
	case "erecete_submit":
		return true, w.handleEreceteSubmit(ctx, msg)
	case "erecete_cancel":
		return true, w.handleEreceteCancel(ctx, msg)
	case "its_notify":
		return true, w.handleItsNotify(ctx, msg)
	default:
		w.log.Warn("medula outbox: unknown message type", "type", msg.MessageType)
		return true, w.medula.CompleteFailure(ctx, msg.ID, msg.TargetID, "UNKNOWN_MSG_TYPE",
			"bilinmeyen mesaj türü: "+msg.MessageType, msg.RetryCount)
	}
}

// ---------- Provision (request / cancel / takip close) ----------

func (w *OutboxWorker) handleProvision(ctx context.Context, msg *repo.OutboxMessage) error {
	var patientTC string
	var provisionType, branchCode string
	var institutionID *uuid.UUID
	row := w.pool.QueryRow(ctx,
		`SELECT p.identifier_value, mp.provision_type, COALESCE(mp.branch_code, ''), mp.institution_id
		 FROM medula_provision mp
		 JOIN patient p ON p.id = mp.patient_id
		 WHERE mp.id = $1`, msg.TargetID)
	if err := row.Scan(&patientTC, &provisionType, &branchCode, &institutionID); err != nil {
		return w.medula.CompleteFailure(ctx, msg.ID, msg.TargetID, "PROVISION_NOT_FOUND",
			fmt.Sprintf("provizyon yüklenemedi: %v", err), msg.RetryCount)
	}

	res, err := w.client.RequestProvision(ctx, ProvisionInput{
		ProvisionID: msg.TargetID, PatientTC: patientTC,
		ProvisionType: provisionType, BranchCode: branchCode, InstitutionID: institutionID,
	})
	if err != nil {
		return w.medula.CompleteFailure(ctx, msg.ID, msg.TargetID, "SOAP_ERROR", err.Error(), msg.RetryCount)
	}
	if !res.Success {
		return w.medula.CompleteFailure(ctx, msg.ID, msg.TargetID, res.ResponseCode,
			fmt.Sprintf("SGK reddi: %s", res.ResponseCode), msg.RetryCount)
	}
	return w.medula.CompleteSuccess(ctx, msg.ID, msg.TargetID, res.TakipNo, res.ResponseCode, res.ResponseRaw)
}

func (w *OutboxWorker) handleProvisionCancel(ctx context.Context, msg *repo.OutboxMessage) error {
	takipNo, _ := msg.Payload["takip_no"].(string)
	reason, _ := msg.Payload["reason"].(string)

	res, err := w.client.CancelProvision(ctx, CancelProvisionInput{
		ProvisionID: msg.TargetID, TakipNo: takipNo, Reason: reason,
	})
	if err != nil {
		return w.medula.CompleteFailure(ctx, msg.ID, msg.TargetID, "SOAP_ERROR", err.Error(), msg.RetryCount)
	}
	if !res.Success {
		return w.medula.CompleteFailure(ctx, msg.ID, msg.TargetID, res.ResponseCode,
			"SGK reddi: "+res.ResponseCode, msg.RetryCount)
	}
	return w.medula.CompleteProvisionCancellation(ctx, msg.ID, msg.TargetID, res.ResponseCode, res.ResponseRaw)
}

func (w *OutboxWorker) handleTakipClose(ctx context.Context, msg *repo.OutboxMessage) error {
	takipNo, _ := msg.Payload["takip_no"].(string)

	res, err := w.client.CloseTakip(ctx, CloseTakipInput{ProvisionID: msg.TargetID, TakipNo: takipNo})
	if err != nil {
		return w.medula.CompleteFailure(ctx, msg.ID, msg.TargetID, "SOAP_ERROR", err.Error(), msg.RetryCount)
	}
	if !res.Success {
		return w.medula.CompleteFailure(ctx, msg.ID, msg.TargetID, res.ResponseCode,
			"SGK reddi: "+res.ResponseCode, msg.RetryCount)
	}
	return w.medula.CompleteTakipClosure(ctx, msg.ID, msg.TargetID, res.ResponseCode, res.ResponseRaw)
}

// ---------- Invoice (submit / cancel) ----------

func (w *OutboxWorker) handleInvoiceSubmit(ctx context.Context, msg *repo.OutboxMessage) error {
	var invoiceID uuid.UUID
	var takipNo *string
	if err := w.pool.QueryRow(ctx,
		`SELECT s.invoice_id, mp.takip_no
		 FROM medula_invoice_submission s
		 LEFT JOIN medula_provision mp ON mp.id = s.provision_id
		 WHERE s.id = $1`, msg.TargetID).Scan(&invoiceID, &takipNo); err != nil {
		return w.medula.CompleteInvoiceSubmissionFailure(ctx, msg.ID, msg.TargetID,
			"SUBMISSION_NOT_FOUND", fmt.Sprintf("submission yüklenemedi: %v", err), msg.RetryCount)
	}
	total := toFloat(msg.Payload["total"])
	lineCount := toInt(msg.Payload["line_count"])

	res, err := w.client.SubmitInvoice(ctx, InvoiceSubmitInput{
		SubmissionID: msg.TargetID, InvoiceID: invoiceID,
		TakipNo: derefStringL(takipNo), Total: total, LineCount: lineCount,
	})
	if err != nil {
		return w.medula.CompleteInvoiceSubmissionFailure(ctx, msg.ID, msg.TargetID, "SOAP_ERROR", err.Error(), msg.RetryCount)
	}
	if !res.Success {
		return w.medula.CompleteInvoiceSubmissionFailure(ctx, msg.ID, msg.TargetID,
			res.ResponseCode, "SGK reddi: "+res.ResponseCode, msg.RetryCount)
	}
	return w.medula.CompleteInvoiceSubmissionSuccess(ctx, msg.ID, msg.TargetID,
		res.BatchNo, res.SGKInvoiceNo, res.ResponseCode, res.ResponseRaw)
}

func (w *OutboxWorker) handleInvoiceCancel(ctx context.Context, msg *repo.OutboxMessage) error {
	sgkInvoiceNo, _ := msg.Payload["sgk_invoice_no"].(string)
	reason, _ := msg.Payload["reason"].(string)

	res, err := w.client.CancelInvoice(ctx, CancelInvoiceInput{
		SubmissionID: msg.TargetID, SGKInvoiceNo: sgkInvoiceNo, Reason: reason,
	})
	if err != nil {
		return w.medula.CompleteInvoiceSubmissionFailure(ctx, msg.ID, msg.TargetID, "SOAP_ERROR", err.Error(), msg.RetryCount)
	}
	if !res.Success {
		return w.medula.CompleteInvoiceSubmissionFailure(ctx, msg.ID, msg.TargetID,
			res.ResponseCode, "SGK reddi: "+res.ResponseCode, msg.RetryCount)
	}
	return w.medula.CompleteInvoiceSubmissionCancellation(ctx, msg.ID, msg.TargetID, res.ResponseCode, res.ResponseRaw)
}

// ---------- Referral ----------

func (w *OutboxWorker) handleReferralCreate(ctx context.Context, msg *repo.OutboxMessage) error {
	var patientTC, targetProviderCode, referralType, reason string
	var targetBranchCode, diagnosis *string
	if err := w.pool.QueryRow(ctx,
		`SELECT p.identifier_value, r.target_provider_code, r.referral_type, r.reason,
		        r.target_branch_code, r.diagnosis_icd10
		 FROM medula_referral r
		 JOIN patient p ON p.id = r.patient_id
		 WHERE r.id = $1`, msg.TargetID).Scan(&patientTC, &targetProviderCode,
		&referralType, &reason, &targetBranchCode, &diagnosis); err != nil {
		return w.medula.CompleteReferralFailure(ctx, msg.ID, msg.TargetID, "REFERRAL_NOT_FOUND",
			fmt.Sprintf("sevk yüklenemedi: %v", err), msg.RetryCount)
	}

	res, err := w.client.CreateReferral(ctx, ReferralInput{
		ReferralID: msg.TargetID, PatientTC: patientTC,
		TargetProviderCode: targetProviderCode,
		TargetBranchCode:   derefStringL(targetBranchCode),
		ReferralType:       referralType, Reason: reason,
		DiagnosisICD10: derefStringL(diagnosis),
	})
	if err != nil {
		return w.medula.CompleteReferralFailure(ctx, msg.ID, msg.TargetID, "SOAP_ERROR", err.Error(), msg.RetryCount)
	}
	if !res.Success {
		return w.medula.CompleteReferralFailure(ctx, msg.ID, msg.TargetID,
			res.ResponseCode, "SGK reddi: "+res.ResponseCode, msg.RetryCount)
	}
	return w.medula.CompleteReferralSuccess(ctx, msg.ID, msg.TargetID, res.SevkNo, res.ResponseCode, res.ResponseRaw)
}

// ---------- e-Rapor ----------

func (w *OutboxWorker) handleEraportSubmit(ctx context.Context, msg *repo.OutboxMessage) error {
	var patientTC string
	var doctorMedulaCode *string
	var kind, validFrom string
	var validTo *string
	var diagnoses, drugs []byte
	if err := w.pool.QueryRow(ctx,
		`SELECT p.identifier_value, dr.medula_doctor_code, e.kind::text,
		        TO_CHAR(e.valid_from, 'YYYY-MM-DD'),
		        TO_CHAR(e.valid_to, 'YYYY-MM-DD'),
		        e.diagnoses_icd10, e.drug_codes
		 FROM medula_eraport e
		 JOIN patient p ON p.id = e.patient_id
		 LEFT JOIN doctor dr ON dr.id = e.doctor_id
		 WHERE e.id = $1`, msg.TargetID).Scan(
		&patientTC, &doctorMedulaCode, &kind, &validFrom, &validTo, &diagnoses, &drugs); err != nil {
		return w.medula.CompleteEraportFailure(ctx, msg.ID, msg.TargetID, "ERAPORT_NOT_FOUND",
			fmt.Sprintf("e-rapor yüklenemedi: %v", err), msg.RetryCount)
	}
	var dxList, drugList []string
	_ = json.Unmarshal(diagnoses, &dxList)
	_ = json.Unmarshal(drugs, &drugList)

	res, err := w.client.SubmitEraport(ctx, EraportInput{
		EraportID: msg.TargetID, PatientTC: patientTC,
		DoctorTC: derefStringL(doctorMedulaCode), Kind: kind,
		DiagnosesICD10: dxList, DrugCodes: drugList,
		ValidFrom: validFrom, ValidTo: derefStringL(validTo),
	})
	if err != nil {
		return w.medula.CompleteEraportFailure(ctx, msg.ID, msg.TargetID, "SOAP_ERROR", err.Error(), msg.RetryCount)
	}
	if !res.Success {
		return w.medula.CompleteEraportFailure(ctx, msg.ID, msg.TargetID,
			res.ResponseCode, "SGK reddi: "+res.ResponseCode, msg.RetryCount)
	}
	return w.medula.CompleteEraportSuccess(ctx, msg.ID, msg.TargetID, res.EraportNo, res.ResponseCode, res.ResponseRaw)
}

func (w *OutboxWorker) handleEraportCancel(ctx context.Context, msg *repo.OutboxMessage) error {
	eraportNo, _ := msg.Payload["eraport_no"].(string)
	reason, _ := msg.Payload["reason"].(string)

	res, err := w.client.CancelEraport(ctx, CancelEraportInput{
		EraportID: msg.TargetID, EraportNo: eraportNo, Reason: reason,
	})
	if err != nil {
		return w.medula.CompleteEraportFailure(ctx, msg.ID, msg.TargetID, "SOAP_ERROR", err.Error(), msg.RetryCount)
	}
	if !res.Success {
		return w.medula.CompleteEraportFailure(ctx, msg.ID, msg.TargetID,
			res.ResponseCode, "SGK reddi: "+res.ResponseCode, msg.RetryCount)
	}
	return w.medula.CompleteEraportCancellation(ctx, msg.ID, msg.TargetID, res.ResponseCode, res.ResponseRaw)
}

// ---------- Helpers ----------

func derefStringL(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func toFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case json.Number:
		f, _ := x.Float64()
		return f
	}
	return 0
}

func toInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case json.Number:
		i, _ := x.Int64()
		return int(i)
	}
	return 0
}

// ---------- e-Reçete (Sağlık Bakanlığı) ----------

func (w *OutboxWorker) handleEreceteSubmit(ctx context.Context, msg *repo.OutboxMessage) error {
	rxCtx, err := w.medula.LoadEreceteContext(ctx, msg.TargetID)
	if err != nil {
		return w.medula.CompleteEreceteSubmitFailure(ctx, msg.ID, msg.TargetID, "RX_NOT_FOUND",
			fmt.Sprintf("reçete yüklenemedi: %v", err), msg.RetryCount)
	}
	res, err := w.ereceteClient.Submit(ctx, erecete.SubmitInput{
		PrescriptionID: msg.TargetID,
		PrescriptionNo: rxCtx.PrescriptionNo,
		DoctorTC:       rxCtx.DoctorTC,
		PatientTC:      rxCtx.PatientTC,
		DiagnosesICD10: rxCtx.DiagnosesICD10,
		DrugATCCodes:   rxCtx.DrugATCCodes,
	})
	if err != nil {
		return w.medula.CompleteEreceteSubmitFailure(ctx, msg.ID, msg.TargetID, "SOAP_ERROR", err.Error(), msg.RetryCount)
	}
	if !res.Success {
		return w.medula.CompleteEreceteSubmitFailure(ctx, msg.ID, msg.TargetID,
			res.ResponseCode, "Bakanlık reddi: "+res.ResponseCode, msg.RetryCount)
	}
	return w.medula.CompleteEreceteSubmitSuccess(ctx, msg.ID, msg.TargetID,
		res.EPrescriptionNo, res.ResponseCode, res.Raw)
}

func (w *OutboxWorker) handleEreceteCancel(ctx context.Context, msg *repo.OutboxMessage) error {
	ePrescriptionNo, _ := msg.Payload["e_prescription_no"].(string)
	reason, _ := msg.Payload["reason"].(string)
	res, err := w.ereceteClient.Cancel(ctx, erecete.CancelInput{
		PrescriptionID:  msg.TargetID,
		EPrescriptionNo: ePrescriptionNo,
		Reason:          reason,
	})
	if err != nil {
		return w.medula.CompleteEreceteSubmitFailure(ctx, msg.ID, msg.TargetID, "SOAP_ERROR", err.Error(), msg.RetryCount)
	}
	if !res.Success {
		return w.medula.CompleteEreceteSubmitFailure(ctx, msg.ID, msg.TargetID,
			res.ResponseCode, "Bakanlık reddi: "+res.ResponseCode, msg.RetryCount)
	}
	// Cancel success: set status to cancelled via a quick update through
	// the success path (we treat cancelled as a terminal "ok" state and
	// just stamp it via a thin SQL).
	if _, err := w.pool.Exec(ctx,
		`UPDATE prescription SET e_prescription_status = 'cancelled' WHERE id = $1`,
		msg.TargetID); err != nil {
		return err
	}
	// Mark outbox sent.
	_, err = w.pool.Exec(ctx,
		`UPDATE medula_outgoing_message SET status = 'sent', completed_at = NOW(), last_error = NULL
		 WHERE id = $1`, msg.ID)
	return err
}

// ---------- İTS (İlaç Takip Sistemi) ----------

func (w *OutboxWorker) handleItsNotify(ctx context.Context, msg *repo.OutboxMessage) error {
	itsCtx, err := w.medula.LoadItsContext(ctx, msg.TargetID)
	if err != nil {
		return w.medula.CompleteItsNotifyFailure(ctx, msg.ID, msg.TargetID, "DISPENSE_NOT_FOUND",
			fmt.Sprintf("dispense yüklenemedi: %v", err), msg.RetryCount)
	}
	res, err := w.itsClient.Notify(ctx, its.NotifyInput{
		DispenseID:   msg.TargetID,
		Karekod:      itsCtx.Karekod,
		PatientTC:    itsCtx.PatientTC,
		DispensedAt:  time.Now(),
		PharmacistTC: itsCtx.PharmacistTC,
		Quantity:     itsCtx.Quantity,
	})
	if err != nil {
		return w.medula.CompleteItsNotifyFailure(ctx, msg.ID, msg.TargetID, "SOAP_ERROR", err.Error(), msg.RetryCount)
	}
	if !res.Success {
		return w.medula.CompleteItsNotifyFailure(ctx, msg.ID, msg.TargetID,
			res.ResponseCode, "İTS reddi: "+res.ResponseCode, msg.RetryCount)
	}
	return w.medula.CompleteItsNotifySuccess(ctx, msg.ID, msg.TargetID, res.ResponseCode, res.Raw)
}

// uuid import keep-alive — handler funcs reference uuid only via msg.TargetID.
var _ = uuid.Nil
