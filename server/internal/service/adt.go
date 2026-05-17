package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/medigt/medigt/server/internal/integration/hl7"
	"github.com/medigt/medigt/server/pkg/db/repo"
)

// ADTService builds + enqueues HL7 ADT messages for admission events.
//
// Call this from the admission handler AFTER the AdmissionService
// transaction commits — emit a message-per-event so downstream consumers
// (PACS, LIS, regional HIE) see the same state we just persisted.
//
// The actual dispatch happens out-of-band via hl7.ADTOutboxWorker; this
// service just stamps a row in hl7_outbound_message.
type ADTService struct {
	pool *pgxpool.Pool
	repo *repo.HL7OutboundRepo
	log  *slog.Logger
}

func NewADTService(pool *pgxpool.Pool, r *repo.HL7OutboundRepo, log *slog.Logger) *ADTService {
	return &ADTService{pool: pool, repo: r, log: log}
}

// Emit looks up everything HL7 needs about the admission's patient, ward,
// bed and doctor, builds the ADT message + enqueues it. Failure is
// LOGGED, not returned — an ADT enqueue glitch must NOT roll back the
// user's admission. The outbox can be backfilled by re-emitting if the
// row didn't land.
func (s *ADTService) Emit(ctx context.Context, event hl7.ADTEvent, admissionID uuid.UUID) {
	row, err := s.loadAdmissionContext(ctx, admissionID)
	if err != nil {
		s.log.Warn("adt: load admission context failed", "err", err, "admission", admissionID)
		return
	}

	facility := hl7.SendingFacility{
		Application: "MEDIGT",
		Facility:    row.facilityCode,
		// Receiver is filled by config in a future iteration — for V1 a
		// generic "DOWNSTREAM" string is enough; the mock dispatcher and
		// MLLP peer don't care about it.
		Receiver:    "DOWNSTREAM",
		ReceiverFac: "ANY",
	}
	patient := hl7.PatientInfo{
		MRN:       row.patientMRN,
		TC:        row.patientTC,
		LastName:  row.patientLastName,
		FirstName: row.patientFirstName,
		BirthDate: row.patientBirthDate,
		Sex:       genderToHL7(row.patientGender),
	}
	visit := hl7.VisitInfo{
		AdmissionNo:     row.admissionNo,
		PatientClass:    "I",
		WardCode:        row.wardCode,
		WardName:        row.wardName,
		BedCode:         row.bedCode,
		AttendingDoctor: row.attendingDoctor,
		AdmissionAt:     &row.admittedAt,
		DischargeAt:     row.dischargedAt,
		DischargeKind:   row.dischargeKind,
	}

	controlID := "MSGCTRL-" + uuid.New().String()
	body := hl7.BuildADT(event, facility, patient, visit, nil, controlID, time.Now())

	if _, err := s.repo.Enqueue(ctx, repo.EnqueueHL7Input{
		OrganizationID:   row.orgID,
		BranchID:         row.branchID,
		MessageControlID: controlID,
		EventType:        string(event),
		PatientID:        row.patientID,
		AdmissionID:      &admissionID,
		RawMessage:       body,
	}); err != nil {
		s.log.Warn("adt: enqueue failed", "err", err, "admission", admissionID, "event", event)
	}
}

type admissionADTContext struct {
	orgID            uuid.UUID
	branchID         uuid.UUID
	facilityCode     string
	admissionNo      string
	admittedAt       time.Time
	dischargedAt     *time.Time
	dischargeKind    string
	patientID        uuid.UUID
	patientMRN       string
	patientTC        string
	patientLastName  string
	patientFirstName string
	patientBirthDate *time.Time
	patientGender    string
	wardCode         string
	wardName         string
	bedCode          string
	attendingDoctor  string
}

func (s *ADTService) loadAdmissionContext(ctx context.Context, admissionID uuid.UUID) (*admissionADTContext, error) {
	c := &admissionADTContext{}
	var dischargeKind *string
	err := s.pool.QueryRow(ctx,
		`SELECT a.organization_id, a.branch_id,
		        COALESCE(br.sgk_facility_code, ''),
		        a.admission_no, a.admitted_at, a.discharged_at, a.discharge_kind::text,
		        a.patient_id, p.mrn, COALESCE(p.identifier_value, ''),
		        p.last_name, p.first_name, p.birth_date, COALESCE(p.gender, ''),
		        w.code, w.name, COALESCE(b.code, ''),
		        COALESCE(NULLIF(CONCAT_WS(' ', sm.title, sm.first_name, sm.last_name), ''), '')
		 FROM admission a
		 JOIN branch br ON br.id = a.branch_id
		 JOIN patient p ON p.id = a.patient_id
		 LEFT JOIN ward w ON w.id = a.ward_id
		 LEFT JOIN bed b ON b.id = a.bed_id
		 LEFT JOIN doctor d ON d.id = a.admitting_doctor_id
		 LEFT JOIN staff_member sm ON sm.id = d.staff_member_id
		 WHERE a.id = $1`, admissionID).Scan(
		&c.orgID, &c.branchID, &c.facilityCode,
		&c.admissionNo, &c.admittedAt, &c.dischargedAt, &dischargeKind,
		&c.patientID, &c.patientMRN, &c.patientTC,
		&c.patientLastName, &c.patientFirstName, &c.patientBirthDate, &c.patientGender,
		&c.wardCode, &c.wardName, &c.bedCode,
		&c.attendingDoctor)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, repo.ErrNotFound
		}
		return nil, err
	}
	if dischargeKind != nil {
		c.dischargeKind = *dischargeKind
	}
	return c, nil
}

// genderToHL7 maps our gender enum (male/female/other/unknown) to HL7
// AdministrativeSex (M/F/O/U).
func genderToHL7(g string) string {
	switch g {
	case "male":
		return "M"
	case "female":
		return "F"
	case "other":
		return "O"
	default:
		return "U"
	}
}
