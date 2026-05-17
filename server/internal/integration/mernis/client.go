// Package mernis is the adapter for NVI's KPSPublicV2 SOAP service.
//
// In non-production (AppEnv != "production") we run a deterministic
// simulation: TC numbers ending in '0' are rejected; everything else
// passes if first_name+last_name+birth_year aren't empty. When the SGK
// production cert arrives we drop in a real client that hits
// https://tckimlik.nvi.gov.tr/Service/KPSPublic.asmx — the Service
// interface below stays the same.
package mernis

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Service is the surface other packages consume; swap implementations
// via dependency injection in main.go.
type Service interface {
	Verify(ctx context.Context, in VerifyInput) (*VerifyResult, error)
}

type VerifyInput struct {
	TCKimlikNo string // 11 digits; checksum validated in the verifier wrapper
	FirstName  string
	LastName   string
	BirthYear  int
}

type VerifyResult struct {
	Verified     bool
	ResponseCode string // simulated or NVI-returned code
	Latency      time.Duration
}

// MockClient — deterministic local simulation. Production swap-in lives
// behind the same Service interface.
type MockClient struct{}

func NewMockClient() *MockClient { return &MockClient{} }

var ErrIncompleteInput = errors.New("ad, soyad ve doğum yılı zorunlu")

// Verify implements Service in mock mode:
//   - TC must already pass checksum (caller validates)
//   - first_name+last_name+birth_year non-empty
//   - last digit of TC == '0' → simulated rejection (test rejection codes)
//   - otherwise verified=true with code "MERNIS:OK_SIM"
func (m *MockClient) Verify(ctx context.Context, in VerifyInput) (*VerifyResult, error) {
	start := time.Now()
	// Pretend a 50ms round-trip so the UI feels real.
	select {
	case <-time.After(50 * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if strings.TrimSpace(in.FirstName) == "" || strings.TrimSpace(in.LastName) == "" || in.BirthYear == 0 {
		return nil, ErrIncompleteInput
	}
	if len(in.TCKimlikNo) != 11 {
		return &VerifyResult{Verified: false, ResponseCode: "MERNIS:BAD_TC_FORMAT", Latency: time.Since(start)}, nil
	}
	// Deterministic rejection rule for testing the failure path.
	if strings.HasSuffix(in.TCKimlikNo, "0") {
		return &VerifyResult{Verified: false, ResponseCode: "MERNIS:NOT_FOUND_SIM", Latency: time.Since(start)}, nil
	}
	return &VerifyResult{Verified: true, ResponseCode: "MERNIS:OK_SIM", Latency: time.Since(start)}, nil
}
