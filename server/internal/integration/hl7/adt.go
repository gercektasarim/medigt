package hl7

import (
	"fmt"
	"strings"
	"time"
)

// ADT (Admission, Discharge, Transfer) message builder.
//
// We support a minimal HL7 v2.5 envelope with MSH + EVN + PID + PV1. DG1
// (diagnoses) is appended when supplied. Real-world downstream consumers
// (PACS, regional HIE, billing) typically validate MSH + PID + PV1 and
// tolerate missing optional segments; we emit only what we have.
//
// Encoding follows the HL7 default: '|^~\&'. We don't escape characters
// in field values for V1 — production senders use a strict escaper for
// '|', '^', '~', '\', '&'. TODO when the first peer rejects an escape;
// for now the input fields are sourced from app data that doesn't carry
// delimiter chars.

// SendingFacility is the four-tuple HL7 expects in MSH-3/4 (application
// + facility). We keep it as a single record so callers don't sprinkle
// magic strings around the codebase.
type SendingFacility struct {
	Application string // örn. "MEDIGT"
	Facility    string // hospital code / sgk_facility_code
	Receiver    string // peer system name (PACS, LIS, ...)
	ReceiverFac string // peer facility code
}

// ADTEvent is the HL7 trigger code: A01 admit, A02 transfer, A03
// discharge, A04 outpatient register, A08 patient info update.
type ADTEvent string

const (
	EventAdmit     ADTEvent = "A01"
	EventTransfer  ADTEvent = "A02"
	EventDischarge ADTEvent = "A03"
	EventRegister  ADTEvent = "A04"
	EventUpdate    ADTEvent = "A08"
)

// PatientInfo carries everything PID-3 / PID-5 / PID-7 / PID-8 need.
type PatientInfo struct {
	MRN        string
	TC         string // identifier authority MERNIS
	LastName   string
	FirstName  string
	BirthDate  *time.Time
	Sex        string // M / F / U
}

// VisitInfo carries the PV1 fields a downstream consumer typically cares
// about. All fields optional; empty fields emit as bare separators which
// is HL7-legal.
type VisitInfo struct {
	AdmissionNo      string
	PatientClass     string // I (inpatient), O (outpatient), E (emergency), P (preadmit)
	WardCode         string
	WardName         string
	BedCode          string
	AttendingDoctor  string // free-text "Uzm. Dr. Selin Yıldız"
	AdmissionAt      *time.Time
	DischargeAt      *time.Time
	DischargeKind    string // home / referred / against_advice / ...
}

// Diagnosis is one DG1 entry — ICD-10 code + description. is_primary
// drives DG1-6 (Diagnosis Type) — "A" = admitting, "W" = working,
// "F" = final, "M" = medical record (default for non-primary).
type Diagnosis struct {
	ICD10Code   string
	Description string
	IsPrimary   bool
}

// BuildADT returns the wire-format pipe-bar HL7 message + the
// auto-generated MessageControlID so the outbox can store it for
// idempotency checks.
//
// The message is delimited with the standard '|^~\&' separators and
// uses \r segment terminators per HL7 over MLLP.
func BuildADT(
	event ADTEvent,
	facility SendingFacility,
	patient PatientInfo,
	visit VisitInfo,
	diagnoses []Diagnosis,
	messageControlID string,
	now time.Time,
) string {
	const sep = "\r"
	ts := func(t time.Time) string {
		return t.UTC().Format("20060102150405")
	}
	tsPtr := func(t *time.Time) string {
		if t == nil {
			return ""
		}
		return ts(*t)
	}

	var b strings.Builder
	// MSH
	// MSH | encoding | sender app | sender fac | receiver app | receiver fac |
	//   datetime | security | message type | control id | processing id | version
	fmt.Fprintf(&b, "MSH|^~\\&|%s|%s|%s|%s|%s||ADT^%s|%s|P|2.5",
		facility.Application, facility.Facility,
		facility.Receiver, facility.ReceiverFac,
		ts(now), event, messageControlID)
	b.WriteString(sep)

	// EVN — Event Type. EVN-1 = trigger code, EVN-2 = recorded date/time.
	fmt.Fprintf(&b, "EVN|%s|%s", event, ts(now))
	b.WriteString(sep)

	// PID — Patient Identification.
	// PID-3: identifier list "MRN^^^MEDIGT^MR~TC^^^MERNIS^NN"
	idList := patient.MRN + "^^^MEDIGT^MR"
	if patient.TC != "" {
		idList += "~" + patient.TC + "^^^MERNIS^NN"
	}
	dob := tsPtr(patient.BirthDate)
	if dob != "" {
		dob = dob[:8] // YYYYMMDD
	}
	fmt.Fprintf(&b, "PID|1||%s||%s^%s||%s|%s",
		idList,
		patient.LastName, patient.FirstName,
		dob, patient.Sex)
	b.WriteString(sep)

	// PV1 — Patient Visit.
	// PV1-2: patient class, PV1-3: assigned location (ward^room^bed^facility),
	// PV1-7: attending doctor, PV1-19: visit number (admission no),
	// PV1-44: admit datetime, PV1-45: discharge datetime, PV1-36: disposition.
	location := visit.WardCode + "^" + visit.BedCode + "^" + visit.WardName + "^" + facility.Facility
	fmt.Fprintf(&b, "PV1|1|%s|%s||||%s||||||||||||%s|||||||||||||||||||||||||%s|%s|%s",
		visit.PatientClass,
		location,
		escAttending(visit.AttendingDoctor),
		visit.AdmissionNo,
		visit.DischargeKind,
		tsPtr(visit.AdmissionAt),
		tsPtr(visit.DischargeAt))
	b.WriteString(sep)

	// DG1 — one per diagnosis.
	for i, dx := range diagnoses {
		dxType := "M"
		if dx.IsPrimary {
			dxType = "A"
		}
		fmt.Fprintf(&b, "DG1|%d|ICD-10|%s^%s|||%s",
			i+1, dx.ICD10Code, dx.Description, dxType)
		b.WriteString(sep)
	}

	return b.String()
}

// escAttending puts the doctor name in the HL7 XCN format
// (id^family^given^...). For free-text we tuck the whole label into the
// family-name slot, which most consumers display verbatim.
func escAttending(name string) string {
	if name == "" {
		return ""
	}
	parts := strings.SplitN(strings.TrimSpace(name), " ", 2)
	if len(parts) == 1 {
		return "^" + parts[0]
	}
	return "^" + parts[1] + "^" + parts[0]
}
