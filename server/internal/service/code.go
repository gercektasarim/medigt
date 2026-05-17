package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/medigt/medigt/server/internal/cache"
)

const (
	codeTTL          = 5 * time.Minute
	maxAttempts      = 5
	attemptsTTL      = 15 * time.Minute
	resendCooldown   = 60 * time.Second
	// DevMasterCode bypasses real codes when APP_ENV != "production".
	DevMasterCode = "888888"
)

var (
	ErrInvalidCode    = errors.New("invalid code")
	ErrCodeExpired    = errors.New("code expired")
	ErrTooManyAttempts = errors.New("too many attempts, try again later")
	ErrResendCooldown = errors.New("please wait before requesting a new code")
)

type CodeService struct {
	cache  cache.Client
	devMode bool
}

func NewCodeService(c cache.Client, devMode bool) *CodeService {
	return &CodeService{cache: c, devMode: devMode}
}

// Issue creates a 6-digit code, stores it in cache, and returns it.
// Caller is responsible for delivering the code (email, SMS, etc.).
func (s *CodeService) Issue(ctx context.Context, email string) (string, error) {
	email = normalizeEmail(email)
	cooldownKey := "auth:cooldown:" + email
	if _, err := s.cache.Get(ctx, cooldownKey); err == nil {
		return "", ErrResendCooldown
	}

	code, err := randomDigits(6)
	if err != nil {
		return "", err
	}

	if err := s.cache.Set(ctx, codeKey(email), code, codeTTL); err != nil {
		return "", err
	}
	_ = s.cache.Del(ctx, attemptsKey(email))
	_ = s.cache.Set(ctx, cooldownKey, "1", resendCooldown)
	return code, nil
}

// Verify returns nil when the supplied code matches. Wrong codes increment
// the attempts counter; after maxAttempts the code is destroyed.
func (s *CodeService) Verify(ctx context.Context, email, supplied string) error {
	email = normalizeEmail(email)
	supplied = strings.TrimSpace(supplied)

	if s.devMode && supplied == DevMasterCode {
		_ = s.cache.Del(ctx, codeKey(email))
		_ = s.cache.Del(ctx, attemptsKey(email))
		return nil
	}

	attempts, _ := s.cache.Incr(ctx, attemptsKey(email))
	if attempts == 1 {
		_ = s.cache.Expire(ctx, attemptsKey(email), attemptsTTL)
	}
	if attempts > maxAttempts {
		_ = s.cache.Del(ctx, codeKey(email))
		return ErrTooManyAttempts
	}

	stored, err := s.cache.Get(ctx, codeKey(email))
	if errors.Is(err, cache.ErrMiss) {
		return ErrCodeExpired
	}
	if err != nil {
		return err
	}
	if stored != supplied {
		return ErrInvalidCode
	}

	_ = s.cache.Del(ctx, codeKey(email))
	_ = s.cache.Del(ctx, attemptsKey(email))
	return nil
}

func codeKey(email string) string     { return "auth:code:" + email }
func attemptsKey(email string) string { return "auth:attempts:" + email }
func normalizeEmail(s string) string  { return strings.ToLower(strings.TrimSpace(s)) }

func randomDigits(n int) (string, error) {
	const digits = "0123456789"
	b := make([]byte, n)
	for i := range b {
		k, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", fmt.Errorf("random: %w", err)
		}
		b[i] = digits[k.Int64()]
	}
	return string(b), nil
}
