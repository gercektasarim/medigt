// Slug values that cannot be used for organization or branch URL segments.
// They collide with global routes or with the /h/ tenant prefix conventions.

export const RESERVED_ORG_SLUGS = new Set<string>([
  "h",
  "login",
  "logout",
  "register",
  "signup",
  "onboarding",
  "davet",
  "invite",
  "sifre-sifirla",
  "password-reset",
  "auth",
  "api",
  "ws",
  "hastaneler",
  "hospitals",
  "admin",
  "settings",
  "ayarlar",
  "help",
  "yardim",
  "_next",
  "public",
  "static",
]);

export const RESERVED_BRANCH_SLUGS = new Set<string>([
  "ayarlar",
  "settings",
  "yeni",
  "new",
  "admin",
]);

export function isReservedOrgSlug(slug: string): boolean {
  return RESERVED_ORG_SLUGS.has(slug.toLowerCase());
}

export function isReservedBranchSlug(slug: string): boolean {
  return RESERVED_BRANCH_SLUGS.has(slug.toLowerCase());
}
