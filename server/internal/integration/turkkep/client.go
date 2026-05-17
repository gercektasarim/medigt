// Package turkkep is the adapter for TURKKEP / E-Tugra cloud-based e-imza
// providers. Real TURKKEP API has 3 SOAP/REST calls:
//
//	Init  → create session, return sessionId + challengeCode that's pushed
//	         to the user's mobile app (or shown if SMS provider)
//	Poll  → check status; returns signed envelope when user approves
//	Cancel→ abort session
//
// The mock is deterministic for development:
//
//   - Init: returns "MOCK-SESSION-" + 6 random chars + a 6-digit numeric
//     challenge_code derived from the document hash (so tests can predict it).
//   - Poll: first call after Init returns 'in_progress'; subsequent calls
//     (≥2s after Init) return 'signed' with a fake PKCS#7 envelope built
//     from the document hash. TC ending in '0' deterministically fails
//     (consistent with our other mock policy).
package turkkep

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type Client interface {
	Init(ctx context.Context, in InitInput) (*InitResponse, error)
	Poll(ctx context.Context, sessionID string) (*PollResponse, error)
	Cancel(ctx context.Context, sessionID string) error
}

type InitInput struct {
	SignerTC     string
	SignerName   string
	DocumentHash string // SHA-256 hex
	DocumentKind string // e.g. "prescription"
}

type InitResponse struct {
	SessionID     string
	ChallengeCode string
	ExpiresAt     time.Time
}

type PollResponse struct {
	Status             string // pending | in_progress | signed | failed | cancelled | expired
	SignedEnvelope     []byte
	CertificateSerial  string
	CertificateSubject string
	ErrorMessage       string
}

var ErrInvalidInput = errors.New("e-imza girdisi geçersiz")
var ErrSessionNotFound = errors.New("e-imza oturumu bulunamadı")

// MockClient — deterministic stateful simulation. session info kept in
// memory; suitable only for development & tests.
type MockClient struct {
	mu       sync.Mutex
	sessions map[string]*mockSession
}

type mockSession struct {
	signerTC     string
	signerName   string
	docHash      string
	docKind      string
	initiatedAt  time.Time
	expiresAt    time.Time
	pollCount    int
	cancelled    bool
}

func NewMockClient() *MockClient {
	return &MockClient{sessions: make(map[string]*mockSession)}
}

func (m *MockClient) Init(ctx context.Context, in InitInput) (*InitResponse, error) {
	select {
	case <-time.After(150 * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if len(in.SignerTC) != 11 || in.DocumentHash == "" {
		return nil, ErrInvalidInput
	}

	// Session id deterministic from doc hash + signer + time (so reruns
	// during same second produce same id — useful for tests).
	seed := sha256.Sum256([]byte(in.SignerTC + in.DocumentHash + time.Now().Format(time.RFC3339)))
	sessionID := "MOCK-" + strings.ToUpper(hex.EncodeToString(seed[:6]))
	// Challenge: 6 numeric digits from the doc hash so flow is reproducible.
	challenge := challengeFromHash(in.DocumentHash)

	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[sessionID] = &mockSession{
		signerTC:    in.SignerTC,
		signerName:  in.SignerName,
		docHash:     in.DocumentHash,
		docKind:     in.DocumentKind,
		initiatedAt: time.Now(),
		expiresAt:   time.Now().Add(15 * time.Minute),
	}
	return &InitResponse{
		SessionID:     sessionID,
		ChallengeCode: challenge,
		ExpiresAt:     m.sessions[sessionID].expiresAt,
	}, nil
}

func (m *MockClient) Poll(ctx context.Context, sessionID string) (*PollResponse, error) {
	select {
	case <-time.After(80 * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[sessionID]
	if !ok {
		return nil, ErrSessionNotFound
	}

	if s.cancelled {
		return &PollResponse{Status: "cancelled"}, nil
	}
	if time.Now().After(s.expiresAt) {
		return &PollResponse{Status: "expired", ErrorMessage: "süre doldu"}, nil
	}

	// Deterministic failure rule consistent with other mocks: signer TC
	// ending in '0' fails immediately.
	if strings.HasSuffix(s.signerTC, "0") {
		return &PollResponse{Status: "failed", ErrorMessage: "TURKKEP_AUTH_REJECTED"}, nil
	}

	s.pollCount++
	// First poll: still in_progress; subsequent calls (after at least 2s
	// or 2 polls) return signed.
	elapsed := time.Since(s.initiatedAt)
	if s.pollCount < 2 && elapsed < 2*time.Second {
		return &PollResponse{Status: "in_progress"}, nil
	}

	envelope := buildMockEnvelope(s.signerTC, s.docHash)
	return &PollResponse{
		Status:             "signed",
		SignedEnvelope:     envelope,
		CertificateSerial:  fmt.Sprintf("MOCK-CERT-%s", s.signerTC[len(s.signerTC)-4:]),
		CertificateSubject: fmt.Sprintf("CN=%s, TC=%s, OU=MediGt Mock CA", s.signerName, s.signerTC),
	}, nil
}

func (m *MockClient) Cancel(ctx context.Context, sessionID string) error {
	select {
	case <-time.After(60 * time.Millisecond):
	case <-ctx.Done():
		return ctx.Err()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[sessionID]
	if !ok {
		return ErrSessionNotFound
	}
	s.cancelled = true
	return nil
}

// challengeFromHash produces a deterministic 6-digit numeric code from the
// document hash. Same hash → same code, useful for E2E.
func challengeFromHash(hash string) string {
	if len(hash) < 6 {
		return "000000"
	}
	// Take first 6 hex chars, mod into 6-digit decimal.
	var n int64
	for _, c := range hash[:8] {
		v := hexVal(c)
		if v < 0 {
			continue
		}
		n = (n*16 + int64(v)) % 1_000_000
	}
	return fmt.Sprintf("%06d", n)
}

func hexVal(c rune) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c - 'a' + 10)
	case c >= 'A' && c <= 'F':
		return int(c - 'A' + 10)
	}
	return -1
}

// buildMockEnvelope synthesises a PKCS#7-ish blob from the document hash
// so callers can store SOMETHING in signed_envelope. Real provider returns
// CMS/CAdES bytes. Format: "MOCK-CMS-v1\n" + hash + "\n" + signerTC.
func buildMockEnvelope(signerTC, docHash string) []byte {
	body := "MOCK-CMS-v1\n" + docHash + "\n" + signerTC
	return []byte(body)
}
