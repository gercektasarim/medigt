package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/medigt/medigt/server/internal/config"
	"github.com/medigt/medigt/server/internal/integration/medula"
	"github.com/medigt/medigt/server/internal/integration/mernis"
	"github.com/medigt/medigt/server/internal/integration/pacs"
	"github.com/medigt/medigt/server/internal/middleware"
	"github.com/medigt/medigt/server/internal/realtime"
	"github.com/medigt/medigt/server/internal/service"
	"github.com/medigt/medigt/server/pkg/db/repo"
)

type Deps struct {
	Log             *slog.Logger
	Pool            *pgxpool.Pool
	Hub             *realtime.Hub
	Cfg             *config.Config
	Auth            *service.AuthService
	Tenant          *service.TenantService
	Users           *repo.UserRepo
	Sessions        *repo.SessionRepo
	Orgs            *repo.OrganizationRepo
	Branches        *repo.BranchRepo
	Memberships     *repo.MembershipRepo
	Specializations *repo.SpecializationRepo
	Staff           *repo.StaffRepo
	Doctors         *repo.DoctorRepo
	Institutions    *repo.InstitutionRepo
	Services        *repo.ServiceCatalogRepo
	ServicePrices   *repo.ServicePriceRepo
	Icd10           *repo.Icd10Repo
	Patients        *repo.PatientRepo
	PatientSvc      *service.PatientService
	Appointments    *repo.AppointmentRepo
	Visits          *repo.VisitRepo
	VisitSvc        *service.VisitService
	Diagnoses       *repo.DiagnosisRepo
	Prescriptions   *repo.PrescriptionRepo
	Vitals          *repo.VitalSignsRepo
	Lab             *repo.LabRepo
	Radiology       *repo.RadiologyRepo
	Wards           *repo.WardRepo
	Beds            *repo.BedRepo
	Admissions      *repo.AdmissionRepo
	AdmissionSvc    *service.AdmissionService
	OperatingRooms  *repo.OperatingRoomRepo
	Surgeries       *repo.SurgeryRepo
	DialysisMachines *repo.DialysisMachineRepo
	DialysisSessions *repo.DialysisSessionRepo
	Medications     *repo.MedicationRepo
	Warehouses      *repo.WarehouseRepo
	Stock           *repo.StockRepo
	Movements       *repo.MovementRepo
	StockSvc        *service.StockService
	Eczane          *repo.EczaneRepo
	CashRegisters   *repo.CashRegisterRepo
	CashMovements   *repo.CashMovementRepo
	CashSvc         *service.CashService
	Invoices        *repo.InvoiceRepo
	Payments        *repo.PaymentRepo
	InvoiceSvc      *service.InvoiceService
	Hakedis         *repo.HakedisRepo
	Mernis          *repo.MernisRepo
	MernisSvc       mernis.Service
	Medula          *repo.MedulaRepo
	MedulaClient    medula.Client
	Cari            *repo.CariRepo
	Refunds         *repo.RefundRepo
	Installments    *repo.InstallmentPlanRepo
	FinanceExt      *service.FinanceExtensionsService
	SignatureRepo   *repo.SignatureRepo
	Signatures      *service.SignatureService
	PACSClient      pacs.Client
	ImageRefs       *repo.ImageReferenceRepo
	LabHL7          *service.LabHL7Service
	Audit           *repo.AuditRepo
	MAR             *repo.MARRepo
	Enabiz          *repo.EnabizRepo
	HL7Outbound     *repo.HL7OutboundRepo
	ADT             *service.ADTService
}

type Handler struct {
	deps Deps
}

func New(deps Deps) *Handler {
	return &Handler{deps: deps}
}

func (h *Handler) Router() http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(middleware.Logger(h.deps.Log))

	origins := h.deps.Cfg.AllowedOrigins
	if len(origins) == 0 {
		origins = []string{h.deps.Cfg.FrontendOrigin}
	}
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Organization-ID", "X-Branch-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/api/health", h.healthCheck)
	r.Get("/api/config", h.publicConfig)
	r.HandleFunc("/ws", h.websocket)

	// Public auth endpoints
	r.Post("/api/auth/send-code", h.authSendCode)
	r.Post("/api/auth/verify-code", h.authVerifyCode)
	r.Post("/api/auth/refresh", h.authRefresh)

	// Authenticated routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireAuth(h.deps.Cfg.JWTSecret))
		r.Use(middleware.ResolveTenant())

		r.Post("/api/auth/logout", h.authLogout)
		r.Get("/api/me", h.me)

		r.Get("/api/organizations", h.listOrganizations)
		r.Post("/api/organizations", h.createOrganization)

		r.Get("/api/organizations/{orgSlug}/branches", h.listBranches)
		r.Post("/api/organizations/{orgSlug}/branches", h.createBranch)

		// Master data: specializations / staff / doctors / institutions.
		// All scoped by the X-Organization-ID header (tenant middleware).
		r.Get("/api/specializations", h.listSpecializations)
		r.Post("/api/specializations", h.createSpecialization)
		r.Delete("/api/specializations/{id}", h.deleteSpecialization)

		r.Get("/api/staff", h.listStaff)
		r.Post("/api/staff", h.createStaff)

		r.Get("/api/doctors", h.listDoctors)
		r.Post("/api/doctors", h.createDoctor)

		r.Get("/api/institutions", h.listInstitutions)
		r.Post("/api/institutions", h.createInstitution)

		// Service catalog + per-institution pricing
		r.Get("/api/services", h.listServices)
		r.Post("/api/services", h.createService)
		r.Get("/api/services/{id}/prices", h.listServicePrices)
		r.Post("/api/services/{id}/prices", h.createServicePrice)
		r.Post("/api/service-prices/bulk-preview", h.previewBulkPriceUpdate)
		r.Post("/api/service-prices/bulk-update", h.applyBulkPriceUpdate)

		// ICD-10 search (system catalog + org-specific) + admin TSV bulk import
		r.Get("/api/icd10", h.searchIcd10)
		r.Post("/api/icd10/import", h.importIcd10TSV)

		// Patients (core clinical entity)
		r.Get("/api/patients", h.listPatients)
		r.Post("/api/patients", h.createPatient)
		r.Get("/api/patients/{id}", h.getPatient)

		// Lightweight TC kimlik validator — used by patient create form.
		r.Post("/api/util/tc/validate", h.validateTC)

		// Appointments (branch-scoped — reads X-Branch-ID)
		r.Get("/api/appointments", h.listAppointments)
		r.Post("/api/appointments", h.createAppointment)
		r.Post("/api/appointments/{id}/status", h.updateAppointmentStatus)
		r.Post("/api/appointments/{id}/cancel", h.cancelAppointment)

		// Visits (clinical encounters) — branch-scoped
		r.Get("/api/visits", h.listVisits)
		r.Get("/api/visits/{id}", h.getVisit)
		r.Post("/api/visits/start-from-appointment", h.startVisitFromAppointment)
		r.Patch("/api/visits/{id}/notes", h.updateVisitNotes)
		r.Post("/api/visits/{id}/complete", h.completeVisit)

		// Diagnoses (per-visit)
		r.Get("/api/visits/{visitId}/diagnoses", h.listDiagnoses)
		r.Post("/api/visits/{visitId}/diagnoses", h.addDiagnosis)
		r.Delete("/api/visits/{visitId}/diagnoses/{id}", h.deleteDiagnosis)

		// Prescriptions (per-visit + sign)
		r.Get("/api/visits/{visitId}/prescriptions", h.listPrescriptions)
		r.Post("/api/visits/{visitId}/prescriptions", h.createPrescription)
		r.Post("/api/prescriptions/{id}/sign", h.signPrescription)

		// Vital signs (per-visit)
		r.Get("/api/visits/{visitId}/vitals", h.listVisitVitals)
		r.Post("/api/visits/{visitId}/vitals", h.addVisitVitals)

		// Laboratory — catalog + orders + result entry
		r.Get("/api/lab-tests", h.searchLabTests)
		r.Get("/api/lab-orders", h.listLabOrders)
		r.Post("/api/lab-orders", h.createLabOrder)
		r.Get("/api/lab-orders/{id}", h.getLabOrder)
		r.Post("/api/lab-orders/{id}/status", h.updateLabOrderStatus)
		r.Patch("/api/lab-order-items/{itemId}/result", h.updateLabItemResult)

		// Radiology — catalog + orders + report entry
		r.Get("/api/radiology-procedures", h.searchRadProcedures)
		r.Get("/api/radiology-orders", h.listRadOrders)
		r.Post("/api/radiology-orders", h.createRadOrder)
		r.Get("/api/radiology-orders/{id}", h.getRadOrder)
		r.Post("/api/radiology-orders/{id}/status", h.updateRadOrderStatus)
		r.Patch("/api/radiology-orders/{id}/report", h.saveRadReport)
		r.Post("/api/radiology-orders/{id}/verify", h.verifyRadReport)
		r.Get("/api/radiology-orders/{id}/images", h.listOrderImageReferences)

		// HL7 lab autoanalyzer inbound (ORU^R01)
		r.Post("/api/integrations/hl7/lab-result", h.hl7InboundORU)

		// Onboarding checklist
		r.Get("/api/onboarding/status", h.getOnboardingStatus)
		r.Post("/api/onboarding/seed-defaults", h.seedOnboardingDefaults)

		// Inpatient — wards, beds, admissions
		r.Get("/api/wards", h.listWards)
		r.Post("/api/wards", h.createWard)
		r.Post("/api/wards/{wardId}/beds", h.createBed)
		r.Get("/api/bed-map", h.getBedMap)
		r.Post("/api/beds/{id}/status", h.updateBedStatus)

		r.Get("/api/admissions", h.listAdmissions)
		r.Post("/api/admissions", h.admit)
		r.Get("/api/admissions/{id}", h.getAdmission)
		r.Get("/api/admissions/{id}/transfers", h.listAdmissionTransfers)
		r.Get("/api/admissions/{id}/adt-messages", h.listAdmissionADTMessages)

		// Video assistant / walk-in kiosk intake — one POST creates patient
		// (if missing) + appointment in 'arrived' state.
		r.Post("/api/intake", h.intake)
		// NLU slot-filler — turns a Turkish transcript into structured
		// data for the current dialog step.
		r.Post("/api/intake/parse", h.parseIntake)

		// Gelen kutusu — kullanıcının eyleme açık iş kalemleri.
		r.Get("/api/inbox", h.listInbox)
		r.Post("/api/admissions/{id}/transfer", h.transferAdmission)
		r.Post("/api/admissions/{id}/discharge", h.discharge)

		// Nursing dashboard
		r.Get("/api/inpatient-board", h.getInpatientBoard)
		r.Post("/api/patients/{id}/vitals", h.addPatientVitals)

		// Surgery
		r.Get("/api/operating-rooms", h.listOperatingRooms)
		r.Post("/api/operating-rooms", h.createOperatingRoom)
		r.Get("/api/surgeries", h.listSurgeries)
		r.Post("/api/surgeries", h.createSurgery)
		r.Get("/api/surgeries/{id}", h.getSurgery)
		r.Post("/api/surgeries/{id}/status", h.updateSurgeryStatus)
		r.Patch("/api/surgeries/{id}/op-note", h.saveSurgeryOpNote)

		// Dialysis
		r.Get("/api/dialysis-machines", h.listDialysisMachines)
		r.Post("/api/dialysis-machines", h.createDialysisMachine)
		r.Get("/api/dialysis-sessions", h.listDialysisSessions)
		r.Post("/api/dialysis-sessions", h.createDialysisSession)
		r.Get("/api/dialysis-sessions/{id}", h.getDialysisSession)
		r.Post("/api/dialysis-sessions/{id}/status", h.updateDialysisStatus)
		r.Patch("/api/dialysis-sessions/{id}/record", h.saveDialysisRecord)

		// Medication catalog
		r.Get("/api/medications", h.listMedications)
		r.Post("/api/medications", h.createMedication)
		r.Get("/api/medications/{id}", h.getMedication)

		// Warehouses + stock + movements
		r.Get("/api/warehouses", h.listWarehouses)
		r.Post("/api/warehouses", h.createWarehouse)
		r.Get("/api/stock", h.listStock)
		r.Get("/api/stock-movements", h.listMovements)
		r.Post("/api/stock-movements/receive", h.receiveStock)
		r.Post("/api/stock-movements/adjust", h.adjustStock)

		// Eczane (dispensing) queue + FEFO lookup + dispense + history
		r.Get("/api/eczane/pending", h.listEczanePending)
		r.Get("/api/eczane/fefo", h.listFEFOLots)
		r.Get("/api/eczane/history", h.listDispenseHistory)
		r.Post("/api/prescription-items/{itemId}/dispense", h.dispensePrescriptionItem)

		// Vezne (cash register sessions)
		r.Get("/api/cash-registers/my", h.myRegister)
		r.Get("/api/cash-registers", h.listRegisters)
		r.Post("/api/cash-registers", h.openRegister)
		r.Post("/api/cash-registers/{id}/close", h.closeRegister)
		r.Get("/api/cash-registers/{id}/movements", h.listRegisterMovements)
		r.Post("/api/cash-registers/{id}/movements", h.recordRegisterMovement)
		r.Get("/api/cash-registers/{id}/z-report", h.zReport)

		// Invoice + payment
		r.Get("/api/invoices", h.listInvoices)
		r.Post("/api/invoices", h.createInvoice)
		r.Get("/api/invoices/{id}", h.getInvoice)
		r.Post("/api/invoices/{id}/finalize", h.finalizeInvoice)
		r.Post("/api/invoices/{id}/cancel", h.cancelInvoice)
		r.Get("/api/invoices/{id}/payments", h.listInvoicePayments)
		r.Post("/api/payments", h.recordPayment)

		// Hakediş (doctor earnings)
		r.Get("/api/hakedis", h.listHakedisSummary)
		r.Get("/api/hakedis/{doctorId}/items", h.listHakedisItems)
		r.Get("/api/hakedis/{doctorId}/rules", h.listCommissionRules)
		r.Post("/api/hakedis/{doctorId}/rules", h.createCommissionRule)
		r.Post("/api/hakedis/bulk-rules/preview", h.previewBulkRules)
		r.Post("/api/hakedis/bulk-rules", h.bulkCreateRules)

		// Audit log viewer (KVKK)
		r.Get("/api/audit-log", h.listAuditLog)
		r.Get("/api/audit-log/facets", h.listAuditFacets)

		// e-Nabız outbox status + manual enqueue (auto-hooks come later)
		r.Get("/api/enabiz/messages", h.listEnabizMessages)
		r.Post("/api/enabiz/enqueue", h.enqueueEnabiz)

		// MAR — Medication Administration Record (yatan hasta ilaç yönetimi)
		r.Get("/api/admissions/{admissionId}/medication-orders", h.listMedicationOrders)
		r.Post("/api/admissions/{admissionId}/medication-orders", h.createMedicationOrder)
		r.Get("/api/admissions/{admissionId}/administrations", h.listAdministrationsForAdmission)
		r.Patch("/api/medication-orders/{id}/status", h.updateMedicationOrderStatus)
		r.Post("/api/medication-orders/{id}/administrations", h.recordAdministration)
		r.Get("/api/medication-orders/{id}/administrations", h.listAdministrationsForOrder)

		// Reports (generic runner; id ∈ registeredReports)
		r.Get("/api/reports/{id}", h.runReport)

		// MERNIS (NVI TC kimlik doğrulama)
		r.Post("/api/mernis/verify", h.verifyMernis)
		r.Get("/api/mernis/logs", h.listMernisLog)

		// Medula (SGK provizyon — outbox-driven)
		r.Get("/api/medula/provisions", h.listMedulaProvisions)
		r.Post("/api/medula/provisions", h.createMedulaProvision)
		r.Get("/api/medula/provisions/{id}", h.getMedulaProvision)
		r.Post("/api/medula/provisions/{id}/cancel", h.cancelMedulaProvision)
		r.Post("/api/medula/provisions/{id}/close-takip", h.closeMedulaTakip)

		// Medula — fatura gönderim
		r.Get("/api/medula/invoice-submissions", h.listInvoiceSubmissions)
		r.Post("/api/medula/invoice-submissions", h.createInvoiceSubmission)
		r.Post("/api/medula/invoice-submissions/{id}/cancel", h.cancelInvoiceSubmission)

		// Medula — sevk
		r.Get("/api/medula/referrals", h.listMedulaReferrals)
		r.Post("/api/medula/referrals", h.createMedulaReferral)

		// Medula — e-rapor
		r.Get("/api/medula/eraports", h.listMedulaEraports)
		r.Post("/api/medula/eraports", h.createMedulaEraport)
		r.Post("/api/medula/eraports/{id}/cancel", h.cancelMedulaEraport)

		// Medula — sync sorgular
		r.Get("/api/medula/queries/takip/{takipNo}", h.queryTakip)
		r.Get("/api/medula/queries/eraport/{eraportNo}", h.queryEraport)
		r.Get("/api/medula/queries/doctor/{tc}", h.queryDoctor)
		r.Get("/api/medula/queries/branches", h.queryBranches)
		r.Get("/api/medula/queries/treatment-types", h.queryTreatmentTypes)
		r.Get("/api/medula/queries/drug-payment/{barcode}", h.queryDrugPayment)

		// Cari hesap (avans)
		r.Get("/api/patients/{id}/cari", h.getPatientAccount)
		r.Post("/api/patients/{id}/advance", h.receiveAdvance)
		r.Post("/api/patients/{id}/advance-refund", h.refundAdvance)
		r.Post("/api/advances/apply", h.applyAdvance)

		// İade (refund)
		r.Get("/api/refunds", h.listRefunds)
		r.Post("/api/refunds", h.processRefund)
		r.Get("/api/invoices/{id}/refunds", h.listInvoiceRefunds)

		// Taksit planı
		r.Get("/api/invoices/{id}/installment-plan", h.getInstallmentPlan)
		r.Post("/api/invoices/{id}/installment-plan", h.createInstallmentPlan)
		r.Get("/api/installments/upcoming", h.listUpcomingInstallments)

		// e-İmza (TURKKEP / E-Tugra cloud)
		r.Post("/api/signatures", h.initSignature)
		r.Get("/api/signatures/mine", h.listMySignatures)
		r.Get("/api/signatures/{id}", h.getSignature)
		r.Post("/api/signatures/{id}/poll", h.pollSignature)
		r.Post("/api/signatures/{id}/cancel", h.cancelSignature)
	})

	return r
}

func (h *Handler) healthCheck(w http.ResponseWriter, r *http.Request) {
	if err := h.deps.Pool.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (h *Handler) publicConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"allow_signup":     h.deps.Cfg.AllowSignup,
		"google_client_id": h.deps.Cfg.GoogleClientID,
		"frontend_origin":  h.deps.Cfg.FrontendOrigin,
		"app_env":          h.deps.Cfg.AppEnv,
	})
}

func (h *Handler) websocket(w http.ResponseWriter, r *http.Request) {
	realtime.ServeWS(h.deps.Hub, h.deps.Log, w, r)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	})
}
