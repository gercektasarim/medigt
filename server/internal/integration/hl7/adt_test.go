package hl7

import (
	"strings"
	"testing"
	"time"
)

func TestBuildADT_A01_HasAllRequiredSegments(t *testing.T) {
	now := time.Date(2026, 5, 17, 9, 30, 0, 0, time.UTC)
	dob := time.Date(1985, 3, 12, 0, 0, 0, 0, time.UTC)
	admit := time.Date(2026, 5, 17, 9, 0, 0, 0, time.UTC)
	msg := BuildADT(EventAdmit,
		SendingFacility{Application: "MEDIGT", Facility: "HOSP1", Receiver: "PACS", ReceiverFac: "PACS1"},
		PatientInfo{
			MRN: "00012345", TC: "10000000146",
			LastName: "Demir", FirstName: "Hasan",
			BirthDate: &dob, Sex: "M",
		},
		VisitInfo{
			AdmissionNo: "ADM-001", PatientClass: "I",
			WardCode: "DAH", WardName: "Dahiliye", BedCode: "101A",
			AttendingDoctor: "Uzm. Dr. Selin Yıldız",
			AdmissionAt:     &admit,
		},
		[]Diagnosis{{ICD10Code: "I10", Description: "Hipertansiyon", IsPrimary: true}},
		"MSGCTRL-1",
		now,
	)
	// Expected segments.
	segs := strings.Split(msg, "\r")
	want := []string{"MSH", "EVN", "PID", "PV1", "DG1"}
	for i, w := range want {
		if i >= len(segs) {
			t.Fatalf("missing segment %s (got %d segments)", w, len(segs))
		}
		if !strings.HasPrefix(segs[i], w+"|") {
			t.Fatalf("segment %d: want %s, got %q", i, w, segs[i])
		}
	}
	// Spot-checks on key fields.
	if !strings.Contains(msg, "ADT^A01") {
		t.Fatal("expected ADT^A01 trigger")
	}
	if !strings.Contains(msg, "MSGCTRL-1") {
		t.Fatal("expected control id in MSH-10")
	}
	if !strings.Contains(msg, "10000000146^^^MERNIS^NN") {
		t.Fatalf("expected TC in PID-3 MERNIS authority; got:\n%s", msg)
	}
	if !strings.Contains(msg, "Demir^Hasan") {
		t.Fatal("expected name in PID-5")
	}
	if !strings.Contains(msg, "I10^Hipertansiyon") {
		t.Fatal("expected ICD-10 in DG1")
	}
}

func TestBuildADT_A03_HasDischargeFields(t *testing.T) {
	now := time.Now()
	admit := now.Add(-3 * 24 * time.Hour)
	discharge := now
	msg := BuildADT(EventDischarge,
		SendingFacility{Application: "MEDIGT", Facility: "HOSP1", Receiver: "HIE", ReceiverFac: "MOH"},
		PatientInfo{MRN: "00012345", LastName: "Demir", FirstName: "Hasan", Sex: "M"},
		VisitInfo{
			AdmissionNo: "ADM-001", PatientClass: "I",
			WardCode: "DAH", BedCode: "101A",
			AdmissionAt: &admit, DischargeAt: &discharge,
			DischargeKind: "home",
		},
		nil,
		"MSGCTRL-X",
		now,
	)
	if !strings.Contains(msg, "ADT^A03") {
		t.Fatal("expected A03 trigger")
	}
	if !strings.Contains(msg, "home") {
		t.Fatalf("expected discharge kind 'home' in PV1; got:\n%s", msg)
	}
	if !strings.HasSuffix(strings.Split(msg, "\r")[3], discharge.UTC().Format("20060102150405")) {
		// PV1 last field is discharge timestamp.
		t.Fatalf("expected discharge ts at end of PV1; got %q", strings.Split(msg, "\r")[3])
	}
}

func TestBuildADT_A02_TransferOnlyEmitsCurrentLocation(t *testing.T) {
	now := time.Now()
	msg := BuildADT(EventTransfer,
		SendingFacility{Application: "MEDIGT", Facility: "HOSP1"},
		PatientInfo{MRN: "00012345", LastName: "X", FirstName: "Y", Sex: "U"},
		VisitInfo{
			AdmissionNo: "ADM-001", PatientClass: "I",
			WardCode: "YBU", WardName: "Yoğun Bakım", BedCode: "Y3",
		},
		nil,
		"MSGCTRL-T",
		now,
	)
	if !strings.Contains(msg, "ADT^A02") {
		t.Fatal("expected A02 trigger")
	}
	if !strings.Contains(msg, "YBU^Y3^Yoğun Bakım^HOSP1") {
		t.Fatalf("expected new location encoded in PV1-3; got:\n%s", msg)
	}
}
