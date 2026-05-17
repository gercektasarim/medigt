package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/medigt/medigt/server/internal/integration/pacs"
	"github.com/medigt/medigt/server/internal/middleware"
	"github.com/medigt/medigt/server/internal/util"
	"github.com/medigt/medigt/server/pkg/db/repo"
)

// ---------- Procedure payloads ----------

type radProcedurePayload struct {
	ID               string  `json:"id"`
	Code             string  `json:"code"`
	Name             string  `json:"name"`
	Modality         string  `json:"modality"`
	BodyRegion       *string `json:"body_region,omitempty"`
	SutCode          *string `json:"sut_code,omitempty"`
	EstimatedMinutes *int    `json:"estimated_minutes,omitempty"`
	PreparationNotes *string `json:"preparation_notes,omitempty"`
	IsSystem         bool    `json:"is_system"`
}

func toRadProcPayload(p *repo.RadiologyProcedure) radProcedurePayload {
	return radProcedurePayload{
		ID: p.ID.String(), Code: p.Code, Name: p.Name,
		Modality: p.Modality, BodyRegion: p.BodyRegion, SutCode: p.SutCode,
		EstimatedMinutes: p.EstimatedMinutes, PreparationNotes: p.PreparationNotes,
		IsSystem: p.IsSystem,
	}
}

func (h *Handler) searchRadProcedures(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	q := r.URL.Query().Get("q")
	modality := r.URL.Query().Get("modality")
	limit := 50
	if s := r.URL.Query().Get("limit"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			limit = v
		}
	}
	items, err := h.deps.Radiology.SearchProcedures(r.Context(), orgID, q, modality, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]radProcedurePayload, 0, len(items))
	for i := range items {
		out = append(out, toRadProcPayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

// ---------- Order payload ----------

type radOrderPayload struct {
	ID                  string     `json:"id"`
	OrderNo             string     `json:"order_no"`
	Status              string     `json:"status"`
	Priority            string     `json:"priority"`
	VisitID             *string    `json:"visit_id,omitempty"`
	PatientID           string     `json:"patient_id"`
	PatientMRN          string     `json:"patient_mrn"`
	PatientFirstName    string     `json:"patient_first_name"`
	PatientLastName     string     `json:"patient_last_name"`
	DoctorFirstName     *string    `json:"doctor_first_name,omitempty"`
	DoctorLastName      *string    `json:"doctor_last_name,omitempty"`
	DoctorTitle         *string    `json:"doctor_title,omitempty"`
	ProcedureID         string     `json:"procedure_id"`
	ProcedureCode       string     `json:"procedure_code"`
	ProcedureName       string     `json:"procedure_name"`
	Modality            string     `json:"modality"`
	BodyRegion          *string    `json:"body_region,omitempty"`
	ClinicalIndication  *string    `json:"clinical_indication,omitempty"`
	ClinicalQuestion    *string    `json:"clinical_question,omitempty"`
	Notes               *string    `json:"notes,omitempty"`
	ScheduledAt         *time.Time `json:"scheduled_at,omitempty"`
	AcquiredAt          *time.Time `json:"acquired_at,omitempty"`
	Findings            *string    `json:"findings,omitempty"`
	Impression          *string    `json:"impression,omitempty"`
	Recommendations     *string    `json:"recommendations,omitempty"`
	ReportedAt          *time.Time `json:"reported_at,omitempty"`
	VerifiedAt          *time.Time `json:"verified_at,omitempty"`
	PacsStudyUID        *string    `json:"pacs_study_uid,omitempty"`
	PacsAccessionNumber *string    `json:"pacs_accession_number,omitempty"`
	ThumbnailURL        *string    `json:"thumbnail_url,omitempty"`
	OrderedAt           time.Time  `json:"ordered_at"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

func baseRadOrder(o *repo.RadiologyOrder) radOrderPayload {
	p := radOrderPayload{
		ID: o.ID.String(), OrderNo: o.OrderNo, Status: o.Status, Priority: o.Priority,
		PatientID: o.PatientID.String(),
		ProcedureID: o.ProcedureID.String(), ProcedureCode: o.ProcedureCode,
		ProcedureName: o.ProcedureName, Modality: o.Modality, BodyRegion: o.BodyRegion,
		ClinicalIndication: o.ClinicalIndication, ClinicalQuestion: o.ClinicalQuestion,
		Notes: o.Notes,
		ScheduledAt: o.ScheduledAt, AcquiredAt: o.AcquiredAt,
		Findings: o.Findings, Impression: o.Impression, Recommendations: o.Recommendations,
		ReportedAt: o.ReportedAt, VerifiedAt: o.VerifiedAt,
		PacsStudyUID: o.PacsStudyUID, PacsAccessionNumber: o.PacsAccessionNumber,
		ThumbnailURL: o.ThumbnailURL,
		OrderedAt: o.OrderedAt, CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt,
	}
	if o.VisitID != nil {
		s := o.VisitID.String()
		p.VisitID = &s
	}
	return p
}

func joinedRadOrder(w *repo.RadiologyOrderWithJoins) radOrderPayload {
	p := baseRadOrder(&w.Order)
	p.PatientMRN = w.PatientMRN
	p.PatientFirstName = w.PatientFirstName
	p.PatientLastName = w.PatientLastName
	p.DoctorFirstName = w.DoctorFirstName
	p.DoctorLastName = w.DoctorLastName
	p.DoctorTitle = w.DoctorTitle
	return p
}

func (h *Handler) listRadOrders(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	f := repo.ListRadOrderFilter{
		Status:   r.URL.Query().Get("status"),
		Modality: r.URL.Query().Get("modality"),
	}
	if v := r.URL.Query().Get("visit_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			f.VisitID = &id
		}
	}
	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
			f.From = &t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local).Add(24 * time.Hour)
			f.To = &t
		}
	}

	items, err := h.deps.Radiology.ListOrders(r.Context(), branchID, f)
	if err != nil {
		h.deps.Log.Error("list radiology orders failed", "err", err)
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]radOrderPayload, 0, len(items))
	for i := range items {
		out = append(out, joinedRadOrder(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) getRadOrder(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	order, err := h.deps.Radiology.GetOrderByID(r.Context(), branchID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "radyoloji istek bulunamadı")
		return
	}
	writeJSON(w, http.StatusOK, joinedRadOrder(order))
}

type createRadOrderReq struct {
	VisitID            string `json:"visit_id"`
	PatientID          string `json:"patient_id"`
	ProcedureID        string `json:"procedure_id"`
	OrderingDoctorID   string `json:"ordering_doctor_id"`
	Priority           string `json:"priority"`
	ClinicalIndication string `json:"clinical_indication"`
	ClinicalQuestion   string `json:"clinical_question"`
	Notes              string `json:"notes"`
}

func (h *Handler) createRadOrder(w http.ResponseWriter, r *http.Request) {
	orgID := mustOrgID(w, r)
	if orgID == uuid.Nil {
		return
	}
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	var req createRadOrderReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	procID, err := uuid.Parse(req.ProcedureID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_procedure", "procedure_id zorunlu")
		return
	}

	in := repo.CreateRadOrderInput{
		OrganizationID:     orgID,
		BranchID:           branchID,
		ProcedureID:        procID,
		Priority:           req.Priority,
		ClinicalIndication: emptyToNil(&req.ClinicalIndication),
		ClinicalQuestion:   emptyToNil(&req.ClinicalQuestion),
		Notes:              emptyToNil(&req.Notes),
	}

	if req.VisitID != "" {
		vid, err := uuid.Parse(req.VisitID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_visit", "visit_id geçersiz")
			return
		}
		visit, err := h.deps.Visits.GetByID(r.Context(), branchID, vid)
		if err != nil {
			writeError(w, http.StatusNotFound, "visit_not_found", "muayene bulunamadı")
			return
		}
		in.VisitID = &visit.ID
		in.PatientID = visit.PatientID
		if visit.DoctorID != nil {
			in.OrderingDoctorID = visit.DoctorID
		}
	} else {
		pid, err := uuid.Parse(req.PatientID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_patient", "patient_id veya visit_id zorunlu")
			return
		}
		in.PatientID = pid
	}
	if req.OrderingDoctorID != "" {
		if id, err := uuid.Parse(req.OrderingDoctorID); err == nil {
			in.OrderingDoctorID = &id
		}
	}
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			in.OrderedByUserID = &uid
		}
	}

	nextNo, err := h.deps.Radiology.NextOrderNo(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "seq_failed", "istek numarası alınamadı")
		return
	}
	in.OrderNo = util.FormatMRN(nextNo)

	order, err := h.deps.Radiology.Create(r.Context(), in)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusBadRequest, "bad_procedure", "prosedür bulunamadı")
			return
		}
		h.deps.Log.Error("create radiology order failed", "err", err)
		writeError(w, http.StatusInternalServerError, "create_failed", "oluşturma başarısız")
		return
	}

	// PACS hook: schedule a study UID for this order and write an
	// image_reference row. Non-fatal — if PACS client errors, the order
	// still returns success; UI can show "PACS bekliyor" indicator until
	// a manual sync.
	if h.deps.PACSClient != nil {
		go h.scheduleRadiologyInPACS(order)
	}

	writeJSON(w, http.StatusCreated, baseRadOrder(order))
}

// scheduleRadiologyInPACS reserves a Study UID in PACS for the freshly-
// created order and writes the resulting image_reference. Runs in its own
// goroutine — failures don't block the HTTP response.
func (h *Handler) scheduleRadiologyInPACS(order *repo.RadiologyOrder) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	scheduledAt := time.Now()
	if order.ScheduledAt != nil {
		scheduledAt = *order.ScheduledAt
	}
	resp, err := h.deps.PACSClient.ScheduleStudy(ctx, pacs.ScheduleInput{
		OrderID:     order.ID,
		OrderNo:     order.OrderNo,
		PatientID:   order.PatientID,
		Modality:    order.Modality,
		Description: order.ProcedureName,
		ScheduledAt: scheduledAt,
	})
	if err != nil {
		h.deps.Log.Warn("PACS schedule failed", "order_no", order.OrderNo, "err", err)
		return
	}

	desc := order.ProcedureName
	if _, err := h.deps.ImageRefs.Create(ctx, repo.CreateImageReferenceInput{
		OrganizationID:   order.OrganizationID,
		BranchID:         order.BranchID,
		RadiologyOrderID: &order.ID,
		PatientID:        order.PatientID,
		StudyInstanceUID: resp.StudyInstanceUID,
		Modality:         order.Modality,
		Description:      &desc,
		PACSBaseURL:      ptrOrNilStr(resp.PACSBaseURL),
		ThumbnailURL:     ptrOrNilStr(resp.ThumbnailURL),
	}); err != nil {
		h.deps.Log.Warn("image_reference insert failed", "order_no", order.OrderNo, "err", err)
		return
	}

	if _, err := h.deps.Pool.Exec(ctx,
		`UPDATE radiology_order SET pacs_study_uid = $2 WHERE id = $1`,
		order.ID, resp.StudyInstanceUID); err != nil {
		h.deps.Log.Warn("radiology_order pacs_study_uid back-fill failed",
			"order_no", order.OrderNo, "err", err)
	}
}

func ptrOrNilStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

type updateRadStatusReq struct {
	Status string `json:"status"`
}

var validRadStatuses = map[string]bool{
	"scheduled": true, "in_progress": true, "acquired": true, "cancelled": true,
}

func (h *Handler) updateRadOrderStatus(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	var req updateRadStatusReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	if !validRadStatuses[req.Status] {
		writeError(w, http.StatusBadRequest, "bad_status", "geçersiz durum")
		return
	}
	var byUser *uuid.UUID
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			byUser = &uid
		}
	}
	if err := h.deps.Radiology.UpdateOrderStatus(r.Context(), branchID, id, req.Status, byUser); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "istek bulunamadı")
			return
		}
		writeError(w, http.StatusInternalServerError, "update_failed", "güncelleme başarısız")
		return
	}
	order, err := h.deps.Radiology.GetOrderByID(r.Context(), branchID, id)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusOK, joinedRadOrder(order))
}

type saveRadReportReq struct {
	Findings            *string `json:"findings"`
	Impression          *string `json:"impression"`
	Recommendations     *string `json:"recommendations"`
	ReportingDoctorID   *string `json:"reporting_doctor_id"`
	PacsStudyUID        *string `json:"pacs_study_uid"`
	PacsAccessionNumber *string `json:"pacs_accession_number"`
	ThumbnailURL        *string `json:"thumbnail_url"`
}

func (h *Handler) saveRadReport(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	var req saveRadReportReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "geçersiz istek gövdesi")
		return
	}
	in := repo.SaveReportInput{
		Findings:            emptyToNil(req.Findings),
		Impression:          emptyToNil(req.Impression),
		Recommendations:     emptyToNil(req.Recommendations),
		PacsStudyUID:        emptyToNil(req.PacsStudyUID),
		PacsAccessionNumber: emptyToNil(req.PacsAccessionNumber),
		ThumbnailURL:        emptyToNil(req.ThumbnailURL),
	}
	if req.ReportingDoctorID != nil && *req.ReportingDoctorID != "" {
		did, err := uuid.Parse(*req.ReportingDoctorID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_doctor", "reporting_doctor_id geçersiz")
			return
		}
		in.ReportingDoctorID = &did
	}

	order, err := h.deps.Radiology.SaveReport(r.Context(), branchID, id, in)
	if err != nil {
		h.deps.Log.Error("save rad report failed", "err", err)
		writeError(w, http.StatusInternalServerError, "save_failed", "rapor kaydedilemedi")
		return
	}
	writeJSON(w, http.StatusOK, baseRadOrder(order))
}

func (h *Handler) verifyRadReport(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	var byUser *uuid.UUID
	if s := middleware.UserIDFromContext(r.Context()); s != "" {
		if uid, err := uuid.Parse(s); err == nil {
			byUser = &uid
		}
	}
	if err := h.deps.Radiology.VerifyReport(r.Context(), branchID, id, byUser); err != nil {
		writeError(w, http.StatusConflict, "not_reportable", "rapor onaylanamadı (önce yazılmalı)")
		return
	}
	h.auditAccess(r.Context(), r, "radiology.report.sign", "radiology_order", id.String(), nil)
	order, err := h.deps.Radiology.GetOrderByID(r.Context(), branchID, id)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusOK, joinedRadOrder(order))
}

// ---------- PACS: image references + viewer URL ----------

type imageReferencePayload struct {
	ID                string     `json:"id"`
	StudyInstanceUID  string     `json:"study_instance_uid"`
	SeriesInstanceUID *string    `json:"series_instance_uid,omitempty"`
	Modality          string     `json:"modality"`
	StudyDate         *time.Time `json:"study_date,omitempty"`
	Description       *string    `json:"description,omitempty"`
	InstanceCount     int        `json:"instance_count"`
	PACSBaseURL       *string    `json:"pacs_base_url,omitempty"`
	ThumbnailURL      *string    `json:"thumbnail_url,omitempty"`
	ViewerURL         string     `json:"viewer_url"`
	SubmittedAt       *time.Time `json:"submitted_at,omitempty"`
	LastSyncedAt      *time.Time `json:"last_synced_at,omitempty"`
}

func (h *Handler) toImageRefPayload(r *repo.ImageReference) imageReferencePayload {
	p := imageReferencePayload{
		ID: r.ID.String(), StudyInstanceUID: r.StudyInstanceUID,
		SeriesInstanceUID: r.SeriesInstanceUID,
		Modality:          r.Modality, StudyDate: r.StudyDate,
		Description: r.Description, InstanceCount: r.InstanceCount,
		PACSBaseURL: r.PACSBaseURL, ThumbnailURL: r.ThumbnailURL,
		SubmittedAt: r.SubmittedAt, LastSyncedAt: r.LastSyncedAt,
	}
	if h.deps.PACSClient != nil {
		p.ViewerURL = h.deps.PACSClient.ViewerURL(r.StudyInstanceUID)
	}
	return p
}

func (h *Handler) listOrderImageReferences(w http.ResponseWriter, r *http.Request) {
	branchID := mustBranchID(w, r)
	if branchID == uuid.Nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_id", "id geçersiz")
		return
	}
	if _, err := h.deps.Radiology.GetOrderByID(r.Context(), branchID, id); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "istek bulunamadı")
		return
	}
	items, err := h.deps.ImageRefs.ListForOrder(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}
	out := make([]imageReferencePayload, 0, len(items))
	for i := range items {
		out = append(out, h.toImageRefPayload(&items[i]))
	}
	writeJSON(w, http.StatusOK, out)
}
