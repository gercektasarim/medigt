package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/medigt/medigt/server/internal/middleware"
	"github.com/medigt/medigt/server/internal/util"
	"github.com/medigt/medigt/server/pkg/db/repo"
)

type prescriptionItemPayload struct {
	ID             string  `json:"id"`
	MedicationName string  `json:"medication_name"`
	Dosage         *string `json:"dosage,omitempty"`
	Frequency      *string `json:"frequency,omitempty"`
	DurationDays   *int    `json:"duration_days,omitempty"`
	Quantity       *string `json:"quantity,omitempty"`
	Instructions   *string `json:"instructions,omitempty"`
	SortOrder      int     `json:"sort_order"`
}

type prescriptionPayload struct {
	ID              string                    `json:"id"`
	VisitID         string                    `json:"visit_id"`
	PatientID       string                    `json:"patient_id"`
	DoctorID        *string                   `json:"doctor_id,omitempty"`
	PrescriptionNo  string                    `json:"prescription_no"`
	EPrescriptionNo *string                   `json:"e_prescription_no,omitempty"`
	Status          string                    `json:"status"`
	Notes           *string                   `json:"notes,omitempty"`
	SignedAt        *time.Time                `json:"signed_at,omitempty"`
	Items           []prescriptionItemPayload `json:"items"`
	CreatedAt       time.Time                 `json:"created_at"`
	UpdatedAt       time.Time                 `json:"updated_at"`
}

func toRxPayload(p *repo.PrescriptionWithItems) prescriptionPayload {
	out := prescriptionPayload{
		ID: p.Prescription.ID.String(), VisitID: p.Prescription.VisitID.String(),
		PatientID:       p.Prescription.PatientID.String(),
		PrescriptionNo:  p.Prescription.PrescriptionNo,
		EPrescriptionNo: p.Prescription.EPrescriptionNo,
		Status:          p.Prescription.Status,
		Notes:           p.Prescription.Notes,
		SignedAt:        p.Prescription.SignedAt,
		CreatedAt:       p.Prescription.CreatedAt,
		UpdatedAt:       p.Prescription.UpdatedAt,
		Items:           make([]prescriptionItemPayload, 0, len(p.Items)),
	}
	if p.Prescription.DoctorID != nil {
		s := p.Prescription.DoctorID.String()
		out.DoctorID = &s
	}
	for _, it := range p.Items {
		out.Items = append(out.Items, prescriptionItemPayload{
			ID: it.ID.String(), MedicationName: it.MedicationName,
			Dosage: it.Dosage, Frequency: it.Frequency,
			DurationDays: it.DurationDays, Quantity: it.Quantity,
			Instructions: it.Instructions, SortOrder: it.SortOrder,
		})
	}
	return out
}

func (h *Handler) listPrescriptions(w http.ResponseWriter, r *http.Request) {
	visitID, err := uuid.Parse(chi.URLParam(r, "visitId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_visit", "visitId geçersiz")
		return
	}
	items, err := h.deps.Prescriptions.ListForVisit(r.Context(), visitID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]prescriptionPayload, 0, len(items))
	for i := range items {
		out = append(out, toRxPayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

type createRxItemReq struct {
	MedicationName string  `json:"medication_name"`
	Dosage         *string `json:"dosage"`
	Frequency      *string `json:"frequency"`
	DurationDays   *int    `json:"duration_days"`
	Quantity       *string `json:"quantity"`
	Instructions   *string `json:"instructions"`
}

type createRxReq struct {
	Notes *string           `json:"notes"`
	Items []createRxItemReq `json:"items"`
}

func (h *Handler) createPrescription(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	visitID, err := uuid.Parse(chi.URLParam(r, "visitId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_visit", "visitId geçersiz")
		return
	}
	visit, err := h.deps.Visits.GetByID(r.Context(), branchID, visitID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "muayene bulunamadı")
		return
	}

	var req createRxReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	if len(req.Items) == 0 {
		writeError(w, http.StatusBadRequest, "missing_items", "en az 1 ilaç gerekli")
		return
	}
	items := make([]repo.CreatePrescriptionItemInput, 0, len(req.Items))
	for _, it := range req.Items {
		name := strings.TrimSpace(it.MedicationName)
		if name == "" {
			writeError(w, http.StatusBadRequest, "missing_med", "ilaç adı boş olamaz")
			return
		}
		items = append(items, repo.CreatePrescriptionItemInput{
			MedicationName: name,
			Dosage:         emptyToNil(it.Dosage),
			Frequency:      emptyToNil(it.Frequency),
			DurationDays:   it.DurationDays,
			Quantity:       emptyToNil(it.Quantity),
			Instructions:   emptyToNil(it.Instructions),
		})
	}

	nextNo, err := h.deps.Prescriptions.NextNo(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "seq_failed", "reçete numarası alınamadı")
		return
	}

	created, err := h.deps.Prescriptions.Create(r.Context(), repo.CreatePrescriptionInput{
		OrganizationID: orgID,
		VisitID:        visitID,
		PatientID:      visit.PatientID,
		DoctorID:       visit.DoctorID,
		PrescriptionNo: util.FormatMRN(nextNo),
		Notes:          req.Notes,
		Items:          items,
	})
	if err != nil {
		h.deps.Log.Error("create prescription failed", "err", err)
		writeError(w, http.StatusInternalServerError, "create_failed", "oluşturma başarısız")
		return
	}
	writeJSON(w, http.StatusCreated, toRxPayload(created))
}

type signPrescriptionReq struct {
	// Optional e-imza link. If supplied, server verifies the signature is
	// 'signed', belongs to the current user, and targets this prescription.
	DigitalSignatureID string `json:"digital_signature_id"`
}

func (h *Handler) signPrescription(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	// Body is optional — empty/missing JSON means "imzala without e-imza".
	var req signPrescriptionReq
	if r.ContentLength > 0 {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	var sigID *uuid.UUID
	if req.DigitalSignatureID != "" {
		uid, err := uuid.Parse(req.DigitalSignatureID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_signature", "digital_signature_id geçersiz")
			return
		}
		// Verify the signature is signed AND belongs to the calling user
		// AND targets this prescription.
		signerStr := middleware.UserIDFromContext(r.Context())
		signerID, perr := uuid.Parse(signerStr)
		if perr != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "kullanıcı yok")
			return
		}
		if err := h.deps.Signatures.VerifyLinked(r.Context(), orgID, uid, signerID, "prescription", id); err != nil {
			writeError(w, http.StatusConflict, "signature_invalid", err.Error())
			return
		}
		sigID = &uid
	}

	if err := h.deps.Prescriptions.SignWithSignature(r.Context(), id, sigID); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusConflict, "not_signable", "reçete imzalanamaz (taslak değil)")
			return
		}
		writeError(w, http.StatusInternalServerError, "sign_failed", "imzalama başarısız")
		return
	}
	details := map[string]any{}
	if sigID != nil {
		details["digital_signature_id"] = sigID.String()
	}
	h.auditAccess(r.Context(), r, "prescription.sign", "prescription", id.String(), details)
	w.WriteHeader(http.StatusNoContent)
}
