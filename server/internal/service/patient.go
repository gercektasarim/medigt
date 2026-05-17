package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/medigt/medigt/server/internal/util"
	"github.com/medigt/medigt/server/pkg/db/repo"
)

var (
	ErrInvalidTC          = errors.New("geçersiz TC kimlik no (11 haneli + checksum)")
	ErrPatientExists      = errors.New("bu kimlik bilgisiyle hasta zaten kayıtlı")
	ErrMissingPatientName = errors.New("ad ve soyad zorunlu")
)

type PatientService struct {
	patients *repo.PatientRepo
}

func NewPatientService(p *repo.PatientRepo) *PatientService {
	return &PatientService{patients: p}
}

type CreatePatientInput struct {
	OrganizationID    uuid.UUID
	FirstName         string
	LastName          string
	BirthDate         *time.Time
	Gender            string
	BloodType         string
	IdentifierKind    string
	IdentifierValue   string
	Phone             string
	Email             string
	Address           string
	NextOfKinName     string
	NextOfKinPhone    string
	Notes             string
}

// Create wraps repo.Create with validation + MRN generation. For TC kimlik
// identifiers, the checksum is verified locally; MERNIS sync is a follow-up
// (would set mernis_verified_at on success).
func (s *PatientService) Create(ctx context.Context, in CreatePatientInput) (*repo.Patient, error) {
	in.FirstName = strings.TrimSpace(in.FirstName)
	in.LastName = strings.TrimSpace(in.LastName)
	if in.FirstName == "" || in.LastName == "" {
		return nil, ErrMissingPatientName
	}

	ident := strings.TrimSpace(in.IdentifierValue)
	identKind := strings.TrimSpace(in.IdentifierKind)
	var identPtr, identKindPtr *string
	if ident != "" {
		if identKind == "tc" {
			if !util.ValidateTC(ident) {
				return nil, ErrInvalidTC
			}
		}
		identPtr = &ident
		identKindPtr = &identKind
		// Refuse duplicates upfront for a friendlier error than 23505.
		if existing, err := s.patients.GetByIdentifier(ctx, in.OrganizationID, identKind, ident); err == nil && existing != nil {
			return nil, ErrPatientExists
		}
	}

	mrnNum, err := s.patients.NextMRN(ctx)
	if err != nil {
		return nil, err
	}

	return s.patients.Create(ctx, repo.CreatePatientInput{
		OrganizationID:    in.OrganizationID,
		MRN:               util.FormatMRN(mrnNum),
		FirstName:         in.FirstName,
		LastName:          in.LastName,
		BirthDate:         in.BirthDate,
		Gender:            defaultStr(in.Gender, "unknown"),
		BloodType:         defaultStr(in.BloodType, "unknown"),
		IdentifierKind:    identKindPtr,
		IdentifierValue:   identPtr,
		Phone:             nilIfBlank(in.Phone),
		Email:             nilIfBlank(in.Email),
		Address:           nilIfBlank(in.Address),
		NextOfKinName:     nilIfBlank(in.NextOfKinName),
		NextOfKinPhone:    nilIfBlank(in.NextOfKinPhone),
		Notes:             nilIfBlank(in.Notes),
	})
}

func nilIfBlank(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func defaultStr(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
