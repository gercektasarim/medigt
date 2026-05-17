package handler

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/medigt/medigt/server/internal/middleware"
	"github.com/medigt/medigt/server/pkg/db/repo"
)

type userPayload struct {
	ID          string     `json:"id"`
	Email       string     `json:"email"`
	Name        string     `json:"name"`
	Phone       *string    `json:"phone,omitempty"`
	AvatarURL   *string    `json:"avatar_url,omitempty"`
	TotpEnabled bool       `json:"totp_enabled"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type orgPayload struct {
	ID            string    `json:"id"`
	Slug          string    `json:"slug"`
	Name          string    `json:"name"`
	Kind          string    `json:"kind"`
	TaxID         *string   `json:"tax_id,omitempty"`
	SGKEmployerNo *string   `json:"sgk_employer_no,omitempty"`
	LogoURL       *string   `json:"logo_url,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type branchPayload struct {
	ID              string    `json:"id"`
	OrganizationID  string    `json:"organization_id"`
	Slug            string    `json:"slug"`
	Name            string    `json:"name"`
	Kind            string    `json:"kind"`
	Address         *string   `json:"address,omitempty"`
	Phone           *string   `json:"phone,omitempty"`
	SGKFacilityCode *string   `json:"sgk_facility_code,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type meResp struct {
	User          userPayload      `json:"user"`
	Organizations []orgPayload     `json:"organizations"`
	Branches      []branchPayload  `json:"branches"`
}

func toUserPayload(u *repo.User) userPayload {
	return userPayload{
		ID: u.ID.String(), Email: u.Email, Name: u.Name,
		Phone: u.Phone, AvatarURL: u.AvatarURL, TotpEnabled: u.TotpEnabled,
		LastLoginAt: u.LastLoginAt, CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt,
	}
}

func toOrgPayload(o *repo.Organization) orgPayload {
	return orgPayload{
		ID: o.ID.String(), Slug: o.Slug, Name: o.Name, Kind: o.Kind,
		TaxID: o.TaxID, SGKEmployerNo: o.SGKEmployerNo, LogoURL: o.LogoURL,
		CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt,
	}
}

func toBranchPayload(b *repo.Branch) branchPayload {
	return branchPayload{
		ID: b.ID.String(), OrganizationID: b.OrganizationID.String(),
		Slug: b.Slug, Name: b.Name, Kind: b.Kind,
		Address: b.Address, Phone: b.Phone, SGKFacilityCode: b.SGKFacilityCode,
		CreatedAt: b.CreatedAt, UpdatedAt: b.UpdatedAt,
	}
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	uidStr := middleware.UserIDFromContext(r.Context())
	uid, err := uuid.Parse(uidStr)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "no_user", "kimlik bulunamadı")
		return
	}

	user, err := h.deps.Users.GetByID(r.Context(), uid)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "no_user", "kullanıcı yok")
		return
	}

	orgs, err := h.deps.Orgs.ListForUser(r.Context(), uid)
	if err != nil {
		h.deps.Log.Error("list orgs failed", "err", err)
		writeError(w, http.StatusInternalServerError, "db_error", "veritabanı hatası")
		return
	}

	allBranches := []branchPayload{}
	for _, o := range orgs {
		bs, err := h.deps.Branches.ListAccessibleForUser(r.Context(), uid, o.ID)
		if err != nil {
			h.deps.Log.Error("list branches failed", "err", err)
			continue
		}
		for i := range bs {
			allBranches = append(allBranches, toBranchPayload(&bs[i]))
		}
	}

	resp := meResp{
		User:          toUserPayload(user),
		Organizations: make([]orgPayload, 0, len(orgs)),
		Branches:      allBranches,
	}
	for i := range orgs {
		resp.Organizations = append(resp.Organizations, toOrgPayload(&orgs[i]))
	}

	writeJSON(w, http.StatusOK, resp)
}
