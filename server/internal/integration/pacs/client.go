// Package pacs is the adapter for a DICOM PACS (Picture Archiving and
// Communication System). Plan calls for Orthanc as the open-source
// reference + OHIF Viewer as the front-end. Real client makes DICOM-Web
// (QIDO-RS / WADO-RS) HTTP calls to Orthanc; the mock today generates
// deterministic UIDs so the rest of the system develops end-to-end
// without a running PACS.
package pacs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Org root OID — used as the DICOM UID prefix per IHE / DICOM PS3.5.
// The full 25-byte OID below is a recognised "public root" stub; production
// deployments register their own root with the National Standards Body
// (Sağlık Bakanlığı for TR).
const OrgRootOID = "1.2.826.0.1.3680043.10.1234"

// Client is the surface the radiology service uses. Submit is called when
// an order is created so PACS can pre-schedule the study (modality
// worklist). Query reads back metadata when the radyolog views.
type Client interface {
	// ScheduleStudy reserves a study_instance_uid for a freshly-created
	// order (modality worklist equivalent). Real Orthanc accepts the
	// DICOM-MWL message; mock just returns a deterministic UID.
	ScheduleStudy(ctx context.Context, in ScheduleInput) (*ScheduleResponse, error)

	// QueryStudy reads study metadata + series count by UID.
	QueryStudy(ctx context.Context, studyUID string) (*StudyDetail, error)

	// ViewerURL returns the URL to embed in an iframe for the given study.
	// Real impl points to the org's OHIF deployment; mock points to the
	// public demo OHIF for hand-testing the workflow.
	ViewerURL(studyUID string) string
}

type ScheduleInput struct {
	OrderID     uuid.UUID
	OrderNo     string
	PatientID   uuid.UUID
	PatientMRN  string
	Modality    string    // CR / CT / MR / US / DX / MG / NM / OT
	Description string
	ScheduledAt time.Time
}

type ScheduleResponse struct {
	StudyInstanceUID string
	SeriesUIDs       []string // pre-allocated; empty if PACS allocates on acquisition
	PACSBaseURL      string
	ThumbnailURL     string
}

type StudyDetail struct {
	StudyInstanceUID string
	StudyDate        time.Time
	Modality         string
	Description      string
	SeriesCount      int
	InstanceCount    int
	Raw              map[string]any
}

var ErrInvalidInput = errors.New("PACS girdisi geçersiz")

// MockClient deterministic local sim. Useful in tests; UIDs derive from
// the order id so test assertions stay stable across runs.
type MockClient struct {
	// viewerBase: e.g. "https://viewer.ohif.org/viewer". UI ifade eder.
	viewerBase string
}

func NewMockClient() *MockClient {
	return &MockClient{viewerBase: "https://viewer.ohif.org/viewer"}
}

func (m *MockClient) ScheduleStudy(ctx context.Context, in ScheduleInput) (*ScheduleResponse, error) {
	select {
	case <-time.After(120 * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if in.Modality == "" {
		return nil, ErrInvalidInput
	}
	uid := buildStudyUID(in.OrderID)
	return &ScheduleResponse{
		StudyInstanceUID: uid,
		// Mock: single series, single instance — production adds these
		// as the modality acquires images.
		SeriesUIDs:   []string{uid + ".1"},
		PACSBaseURL:  "https://orthanc.example.test/dicom-web",
		ThumbnailURL: fmt.Sprintf("https://orthanc.example.test/studies/%s/thumbnail.jpg", uid),
	}, nil
}

func (m *MockClient) QueryStudy(ctx context.Context, studyUID string) (*StudyDetail, error) {
	select {
	case <-time.After(80 * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if studyUID == "" {
		return nil, ErrInvalidInput
	}
	return &StudyDetail{
		StudyInstanceUID: studyUID,
		StudyDate:        time.Now().Add(-1 * time.Hour),
		Modality:         "MOCK",
		Description:      "Simülasyon study",
		SeriesCount:      1,
		InstanceCount:    32,
		Raw:              map[string]any{"source": "mock"},
	}, nil
}

func (m *MockClient) ViewerURL(studyUID string) string {
	// OHIF public demo: ?StudyInstanceUIDs=<uid>. Mock UID won't be
	// resolvable on demo server; production replaces this with the
	// org's OHIF deployment URL.
	return fmt.Sprintf("%s?StudyInstanceUIDs=%s", m.viewerBase, studyUID)
}

// buildStudyUID returns a deterministic DICOM-compliant Study UID from the
// order's UUID. DICOM UIDs must be:
//   - ASCII, dotted-decimal (0-9, '.'), ≤ 64 chars
//   - rooted at a registered OID
//
// We pack the UUID's bytes into base-10 chunks under our OrgRootOID.
func buildStudyUID(orderID uuid.UUID) string {
	// Take last 16 hex chars (8 bytes), convert to decimal in 4-char chunks.
	hex := orderID.String()
	hex = strings.ReplaceAll(hex, "-", "")
	parts := make([]string, 0, 4)
	for i := 0; i+4 <= len(hex) && len(parts) < 4; i += 4 {
		// Each 4-hex-char chunk: 0..65535 → up to 5 decimal digits.
		var n int64
		for _, c := range hex[i : i+4] {
			n = n*16 + int64(hexVal(c))
		}
		parts = append(parts, fmt.Sprintf("%d", n))
	}
	return OrgRootOID + "." + strings.Join(parts, ".")
}

func hexVal(c rune) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return 0
}
