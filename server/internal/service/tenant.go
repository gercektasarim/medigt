package service

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/medigt/medigt/server/pkg/db/repo"
)

var (
	ErrInvalidSlug = errors.New("slug must be 2-40 chars, lowercase letters/digits/hyphens")
)

var slugRe = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,38}[a-z0-9])?$`)

// TenantService coordinates organization + branch creation, membership
// bootstrap, and tenant-level invariants (slug validation, reserved slugs).
type TenantService struct {
	orgs        *repo.OrganizationRepo
	branches    *repo.BranchRepo
	memberships *repo.MembershipRepo
}

type TenantDeps struct {
	Orgs        *repo.OrganizationRepo
	Branches    *repo.BranchRepo
	Memberships *repo.MembershipRepo
}

func NewTenantService(d TenantDeps) *TenantService {
	return &TenantService{orgs: d.Orgs, branches: d.Branches, memberships: d.Memberships}
}

type CreateOrgInput struct {
	OwnerUserID   uuid.UUID
	Slug          string
	Name          string
	Kind          string
	TaxID         string
	SGKEmployerNo string
	// Optional initial branch — when present, created in the same transaction
	// space (conceptually; we use sequential calls — pgx tx wrapping comes later).
	InitialBranch *CreateBranchInput
}

type CreateBranchInput struct {
	OrganizationID  uuid.UUID
	Slug            string
	Name            string
	Kind            string
	SGKFacilityCode string
}

func (s *TenantService) CreateOrganization(ctx context.Context, in CreateOrgInput) (*repo.Organization, *repo.Branch, error) {
	if err := ValidateSlug(in.Slug); err != nil {
		return nil, nil, err
	}

	taxID := nilIfEmpty(in.TaxID)
	sgkEmpl := nilIfEmpty(in.SGKEmployerNo)

	org, err := s.orgs.Create(ctx, repo.CreateOrgInput{
		Slug:          in.Slug,
		Name:          in.Name,
		Kind:          in.Kind,
		TaxID:         taxID,
		SGKEmployerNo: sgkEmpl,
	})
	if err != nil {
		return nil, nil, err
	}

	if _, err := s.memberships.Create(ctx, org.ID, in.OwnerUserID, "org_owner"); err != nil {
		return nil, nil, err
	}

	var branch *repo.Branch
	if in.InitialBranch != nil {
		in.InitialBranch.OrganizationID = org.ID
		branch, err = s.CreateBranch(ctx, *in.InitialBranch)
		if err != nil {
			return org, nil, err
		}
	}

	return org, branch, nil
}

func (s *TenantService) CreateBranch(ctx context.Context, in CreateBranchInput) (*repo.Branch, error) {
	if err := ValidateSlug(in.Slug); err != nil {
		return nil, err
	}
	sgkFacility := nilIfEmpty(in.SGKFacilityCode)
	return s.branches.Create(ctx, repo.CreateBranchInput{
		OrganizationID:  in.OrganizationID,
		Slug:            in.Slug,
		Name:            in.Name,
		Kind:            in.Kind,
		SGKFacilityCode: sgkFacility,
	})
}

func ValidateSlug(slug string) error {
	if !slugRe.MatchString(slug) {
		return ErrInvalidSlug
	}
	if isReservedOrgSlug(slug) {
		return ErrInvalidSlug
	}
	return nil
}

// isReservedOrgSlug mirrors the frontend reserved-slugs guard so backend
// rejects them even if the UI is bypassed.
func isReservedOrgSlug(slug string) bool {
	switch strings.ToLower(slug) {
	case "h", "login", "logout", "register", "signup", "onboarding",
		"davet", "invite", "sifre-sifirla", "password-reset", "auth", "api", "ws",
		"hastaneler", "hospitals", "admin", "settings", "ayarlar", "help", "yardim",
		"_next", "public", "static":
		return true
	}
	return false
}

func nilIfEmpty(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}
