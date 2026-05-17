# CLAUDE.md

Mimari kurallar ve geliştirme rehberi. Bu dosya Claude Code'a (claude.ai/code) bu repoda çalışırken yön gösterir.

## Project Context

MediGt — modern bir hastane bilgi yönetim sistemi (HBYS). Eski Medivizyon HBYS (VB.NET + DevExpress) sisteminin yeniden inşası.

- Hedef: Türkiye'deki orta ölçekli hastaneler için tam fonksiyonel HBYS
- Multi-tenant: organization (hastane grubu) + branch (şube)
- Türkçe varsayılan, KVKK-uyumlu, Medula SGK entegre

## Architecture

**Go backend + monorepo frontend (pnpm workspaces + Turborepo) with shared packages.**

- `server/` — Go backend (Chi router, sqlc for DB, gorilla/websocket for realtime)
- `apps/web/` — Next.js frontend (App Router)
- `apps/desktop/` — (sonra) Electron desktop app
- `packages/core/` — Headless business logic (zero react-dom, all-platform reuse)
- `packages/ui/` — Atomic UI components (zero business logic)
- `packages/views/` — Shared business pages/components (zero next/* imports)
- `packages/tsconfig/` — Shared TypeScript configuration
- `packages/eslint-config/` — Shared ESLint configuration

### Key Architectural Decisions

**Internal Packages pattern** — all shared packages export raw `.ts`/`.tsx` files (no pre-compilation). The consuming app's bundler compiles them directly. Zero-config HMR, instant go-to-definition.

**Dependency direction:** `views/ → core/ + ui/`. Core and UI are independent of each other. No package imports from `next/*`, `react-router-dom`, or app-specific code.

**Platform bridge:** `packages/core/platform/` provides `CoreProvider` — initializes API client, mounts QueryClient, opens WebSocket. Each app wraps its root with `<CoreProvider>` and provides its own `NavigationAdapter`.

**pnpm catalog** — `pnpm-workspace.yaml` defines `catalog:` for version pinning. All shared deps use `catalog:` references.

### State Management

Strict split between server state and client state. Mixing them is the most common way to break it.

- **TanStack Query owns all server state.** Patients, appointments, lab orders — anything fetched from the API. WS events keep it fresh via invalidation; no polling.
- **Zustand owns all client state.** UI selections, filters, drafts, modal state. Stores live in `packages/core/` so both apps share them.
- **React Context** is reserved for cross-cutting platform plumbing — `I18nProvider`, `NavigationProvider`.

**Hard rules:**

- **Never duplicate server data into Zustand.**
- **Branch-scoped queries must key on `branchId`.** Branch switching = cache key change = right data appears, no manual invalidation.
- **Mutations are optimistic by default.**
- **WS events invalidate queries — they never write to stores directly.**

### Multi-Tenancy

All queries filter by `organization_id` and (where applicable) `branch_id`. Membership checks gate access. `X-Organization-ID` and `X-Branch-ID` headers route requests.

URL pattern: `/h/:org_slug/:branch_slug/...`

We learned from Caliptic — workspace not being in the URL was a P1 bug there. We start with URL-driven tenancy from day one.

### WebSocket half-open mitigation (BUILT-IN FROM DAY ONE)

Caliptic had a P0 cache-staleness bug because browsers don't expose WS ping/pong to JS. We mitigate with three layers:

1. Server app-level heartbeat (30s) — see `server/internal/realtime/hub.go`
2. Client `lastMessageTime` tracking + force-close on stale (60s timeout) — see `packages/core/api/ws-client.ts`
3. Page Visibility API invalidation — see `packages/core/platform/core-provider.tsx`

Do not remove any layer.

## Commands

```bash
# One-command dev (auto-setup + start everything)
make dev              # Creates .env, installs deps, starts DB, migrates, launches app

# Explicit setup & run
make setup            # First-time: install + DB up + migrate
make start            # Start backend + frontend together
make stop             # Stop app processes
make db-up            # Start PostgreSQL container
make db-down          # Stop PostgreSQL container
make db-reset         # Drop + recreate DB, re-run migrations

# Frontend
pnpm install
pnpm dev:web          # Next.js dev server (port 3008)
pnpm build            # Build all frontend apps
pnpm typecheck        # TypeScript check (all packages + apps via turbo)
pnpm lint             # ESLint
pnpm test             # TS tests (Vitest, all packages)

# Backend (Go)
make server           # Run Go server only (port 8088)
make build            # Build server + CLI + migrate binaries to server/bin/
make test             # Go tests
make sqlc             # Regenerate sqlc code after editing SQL in server/pkg/db/queries/
make migrate-up       # Run database migrations
make migrate-down     # Rollback migrations
```

### Coding Rules

- TypeScript strict mode; keep types explicit.
- Go code follows standard Go conventions (gofmt, go vet).
- Keep comments in code **English only**. User-facing strings can be Turkish.
- Prefer existing patterns/components over introducing parallel abstractions.
- No backwards-compatibility shims unless explicitly requested — the product is not live yet.
- No broad refactors unless required by the task.

### Package Boundary Rules

Hard constraints. Violating them breaks the cross-platform architecture:

- `packages/core/` — zero react-dom, zero localStorage (use StorageAdapter), zero process.env, zero UI libraries
- `packages/ui/` — zero `@medigt/core` imports (pure UI, no business logic)
- `packages/views/` — zero `next/*` imports, zero `react-router-dom` imports
- `apps/web/platform/` — the only place for Next.js APIs (`next/navigation`)
- `apps/desktop/src/renderer/src/platform/` — the only place for react-router-dom

### The No-Duplication Rule

If the same logic exists in both apps, it must be extracted to a shared package. Decision tree:

1. Depends on Next.js or Electron APIs? → Keep in the app
2. Depends on `react-router-dom` or `next/navigation`? → Keep in the app's `platform/` layer
3. Everything else → `packages/core/` (headless) or `packages/views/` (UI)

### CSS Architecture

Both apps share the same CSS foundation from `packages/ui/styles/`.

- **Design tokens** → use semantic tokens (`bg-background`, `text-muted-foreground`). Never use hardcoded Tailwind colors.
- **Shared styles** → `packages/ui/styles/`. Never duplicate base layer rules in app CSS.
- **Palettes** → eight available (`teal` default for health). Switch via `<html data-palette="...">`.

## KVKK Compliance

- Audit log retention: 10 years (3650 days, env: `AUDIT_RETENTION_DAYS`)
- Personal data fields (TC, password, TOTP secret) stored encrypted (env: `FIELD_ENCRYPTION_KEY`)
- TC kimlik no in audit details: last 4 digits only (use `util.MaskTC`)
- All access to patient records → audit_log entry

## Medula SGK Integration

- SOAP calls via the outbox pattern (`medula_outgoing_message` table)
- Synchronous user requests get 202 Accepted; worker processes outbox
- Realtime event `medula:provision:completed` invalidates the UI when worker finishes
- Test environment first; production cert applied for in Sprint 1 (~3-6 month process)

## i18n

Default locale: `tr`. Date format: `dd.MM.yyyy`. Currency: TRY (₺). Phone: `+90 5XX XXX XX XX`. TC: 11 digits with checksum validation.

## Commit Rules

- Atomic commits grouped by logical intent
- Conventional format: `feat(scope)`, `fix(scope)`, `refactor(scope)`, `docs`, `test(scope)`, `chore(scope)`

## Minimum Pre-Push Checks

```bash
make check    # typecheck, unit tests, Go tests, E2E
```
