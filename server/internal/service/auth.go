package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/medigt/medigt/server/internal/auth"
	"github.com/medigt/medigt/server/pkg/db/repo"
)

var (
	ErrSignupDisabled = errors.New("signup is disabled")
	ErrEmailNotAllowed = errors.New("email not in allow-list")
)

const (
	accessTTL  = 15 * time.Minute
	refreshTTL = 7 * 24 * time.Hour
)

type AuthService struct {
	users       *repo.UserRepo
	sessions    *repo.SessionRepo
	codes       *CodeService
	emailer     Emailer
	jwtSecret   string
	allowSignup bool
	emailDomains []string
	emailList   []string
}

type AuthDeps struct {
	Users        *repo.UserRepo
	Sessions     *repo.SessionRepo
	Codes        *CodeService
	Emailer      Emailer
	JWTSecret    string
	AllowSignup  bool
	EmailDomains []string
	EmailList    []string
}

func NewAuthService(d AuthDeps) *AuthService {
	return &AuthService{
		users: d.Users, sessions: d.Sessions, codes: d.Codes,
		emailer: d.Emailer, jwtSecret: d.JWTSecret,
		allowSignup: d.AllowSignup,
		emailDomains: d.EmailDomains, emailList: d.EmailList,
	}
}

// SendLoginCode issues a code and emails it. If signups are disabled and the
// user doesn't exist, returns ErrSignupDisabled (caller may still want to
// pretend success to prevent email enumeration — that's policy, decide in handler).
func (s *AuthService) SendLoginCode(ctx context.Context, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if !s.emailAllowed(email) {
		return ErrEmailNotAllowed
	}

	_, err := s.users.GetByEmail(ctx, email)
	if errors.Is(err, repo.ErrNotFound) && !s.allowSignup {
		return ErrSignupDisabled
	}

	code, err := s.codes.Issue(ctx, email)
	if err != nil {
		return err
	}
	return s.emailer.SendLoginCode(email, code)
}

// LoginResult is what VerifyLoginCode returns on success.
type LoginResult struct {
	AccessToken  string
	RefreshToken string
	User         *repo.User
	IsNewUser    bool
}

// VerifyLoginCode validates the code, creates the user if necessary (and
// signups are allowed), issues JWT + refresh token + session row.
func (s *AuthService) VerifyLoginCode(ctx context.Context, email, code, userAgent, ip string) (*LoginResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if err := s.codes.Verify(ctx, email, code); err != nil {
		return nil, err
	}

	user, err := s.users.GetByEmail(ctx, email)
	isNew := false
	if errors.Is(err, repo.ErrNotFound) {
		if !s.allowSignup {
			return nil, ErrSignupDisabled
		}
		name := defaultNameFromEmail(email)
		user, err = s.users.CreateForEmail(ctx, email, name)
		if err != nil {
			return nil, err
		}
		isNew = true
	} else if err != nil {
		return nil, err
	}

	refreshPlain, err := auth.GenerateToken(32)
	if err != nil {
		return nil, err
	}
	session, err := s.sessions.Create(ctx, user.ID, hashToken(refreshPlain), userAgent, ip, refreshTTL)
	if err != nil {
		return nil, err
	}

	accessToken, err := auth.IssueJWT(user.ID.String(), session.ID.String(), s.jwtSecret, accessTTL)
	if err != nil {
		return nil, err
	}

	_ = s.users.TouchLastLogin(ctx, user.ID)

	return &LoginResult{
		AccessToken:  accessToken,
		RefreshToken: refreshPlain,
		User:         user,
		IsNewUser:    isNew,
	}, nil
}

// Refresh validates the refresh token and issues a new access JWT.
func (s *AuthService) Refresh(ctx context.Context, refreshPlain string) (string, *repo.User, error) {
	session, err := s.sessions.GetByRefreshHash(ctx, hashToken(refreshPlain))
	if err != nil {
		return "", nil, err
	}
	user, err := s.users.GetByID(ctx, session.UserID)
	if err != nil {
		return "", nil, err
	}
	access, err := auth.IssueJWT(user.ID.String(), session.ID.String(), s.jwtSecret, accessTTL)
	if err != nil {
		return "", nil, err
	}
	return access, user, nil
}

// Logout revokes a session by ID (read from the access token by the handler).
func (s *AuthService) Logout(ctx context.Context, sessionID uuid.UUID) error {
	return s.sessions.Revoke(ctx, sessionID)
}

func (s *AuthService) emailAllowed(email string) bool {
	if len(s.emailList) > 0 {
		for _, e := range s.emailList {
			if strings.EqualFold(strings.TrimSpace(e), email) {
				return true
			}
		}
		return false
	}
	if len(s.emailDomains) > 0 {
		at := strings.LastIndex(email, "@")
		if at < 0 {
			return false
		}
		domain := strings.ToLower(email[at+1:])
		for _, d := range s.emailDomains {
			if strings.EqualFold(strings.TrimSpace(d), domain) {
				return true
			}
		}
		return false
	}
	return true
}

func defaultNameFromEmail(email string) string {
	at := strings.Index(email, "@")
	if at < 0 {
		return email
	}
	return email[:at]
}

func hashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}
