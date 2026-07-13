# Changelog

All notable changes to this project will be recorded here. The format is
loosely based on [Keep a Changelog](https://keepachangelog.com/); dates
in `YYYY-MM-DD`.

## [Unreleased]

### Added
- **Async document parser** (document-svc): the `POST /api/v1/documents/{id}/parse`
  endpoint now enqueues onto a new `parser-q` Asynq queue when `async=true`,
  removing the `TODO` in `internal/service/parser.go:62`. The HTTP handler
  updates the document row to `StatusParsing` and returns 202 immediately,
  the worker picks the task up, runs the same `doParse` logic, and writes
  the result back to `document.metadata`. Wired through a small
  `ParseEnqueuer` interface so dev mode (no `REDIS_ADDR`) keeps the old
  inline path with a no-op enqueuer. New env vars: `REDIS_ADDR`,
  `ASYNQ_CONCURRENCY` (default 4). New `internal/workers/` package with
  the task type, payload, asynq client + server wiring, and a slog
  adapter that emits structured logs.
- **PDF export from docgen-svc**: the `POST /api/v1/docgen/assemble`
  endpoint now accepts `format: "pdf"` and converts the freshly
  assembled `.docx` via a new `internal/pdfexport` package that wraps
  LibreOffice (`soffice --headless --convert-to pdf`). Returns 503 with
  an actionable hint when LibreOffice is not installed; the
  `/api/v1/docgen/download/:id` endpoint now also sets the right
  `Content-Type` for `.pdf` vs `.docx`. Configure with `PDF_SOFFICE_BIN`
  (default: PATH lookup).
- **Graceful tenant-missing errors in billing-svc**: `GetTenantPlan`
  and `UpdateTenantPlan` used to call `mustTenantID(ctx)` which would
  panic and crash the process if the auth middleware ever let a
  request through without a tenant. Both now return a typed Go error
  (`fmt.Errorf("billing: tenant missing from context: %w", err)`) so
  the HTTP layer can translate it to 401 cleanly. The dead-code
  `mustTenantID` helper has been deleted. New tests
  (`internal/store/billing_store_test.go`) lock in the no-panic
  contract.

### Fixed
- **CI markdown lint glob negation**: removed the inline `globs:`
  block from `.github/workflows/ci.yml` so the action picks up
  `.markdownlint-cli2.jsonc`, which already excludes both `node_modules/`
  and `.git/`. The previous inline pattern only negated top-level
  `node_modules` and pulled in ~32k Vite/pnpm-licensed MD files from
  `web/node_modules/`.
- **CI mermaid lint headless Chrome**: added an explicit
  `apt-get install -y chromium-browser chromium-codecs-ffmpeg-extra`
  step and pinned `MERMAID_LINT_CHROME=/usr/bin/chromium-browser` on
  the GitHub-hosted runner, which doesn't ship with Chrome by default.
- **notify-svc dead-code `mustEnv` panic helper**: the function was
  declared but never called; renamed to `requireEnvOrLog` and changed
  to return `(value, error)` so a future caller can decide between
  fail-fast and degraded-mode without taking the whole process down.
  New `cmd/notify-svc/main_test.go` pins the no-panic contract.

### Test counts
- Backend: **433** Test functions across **62** `_test.go` files
  (was 425 / 59).
- Web: **109/109** vitest pass, tsc clean.

### Notes
- `document-svc` now depends on `github.com/hibiken/asynq v0.24.1`
  (matches `workflow-svc`). The `start-stack.sh` already injects
  `REDIS_ADDR` for `workflow-svc`; `document-svc` reads the same env
  var so no additional wiring is needed on the supervisor.
- `docgen-svc` will start even without LibreOffice installed; only
  `format=pdf` requests are 503. word-only callers are unaffected.
- The known CI red (`docs/KNOWN_ISSUES.md`) should now flip green
  after these fixes propagate.

### Added
- **Hybrid KB retrieval** (vector + BM25 + RRF):
  - `kb_store.SearchChunksBM25` uses `plainto_tsquery('simple', ...)`
    so callers can pass natural-language strings; 'simple' config
    treats each CJK char as a token, which matches the trigger
    population strategy.
  - `kb_store.RRFuse` promoted from un-exported helper to public
    method so the service layer can fuse without leaking the helper.
  - `KBService.Search` dispatches on `req.Mode` (vector / bm25 /
    hybrid). Hybrid over-fetches 3x and runs RRF; if embed fails it
    degrades to BM25-only rather than returning empty.
  - New `/api/v1/kb/ingest` body has `{ material_id }`; the router
    previously had no path param so ingest couldn't be triggered
    end-to-end.
- **PostgreSQL integration tests** (real PG @ bidwriter-pg-test:5434):
  - `kb_store_integration_test.go`: UUID chunk insert against old
    BIGSERIAL schema (fails as expected), against migration 00013
    schema (passes), BM25 search with Chinese tokens, hybrid RRF
    fusion ranking.
  - Tests skip when `DATABASE_URL_TEST_PG` is unset so unit-only
    runs stay green.
- **MinIO integration tests** (real MinIO @ bidwriter-minio-test:9100):
  - `s3_minio_integration_test.go`: upload + Get + Delete round-trip
    plus a 16MB large-file upload case to exercise multipart paths.
    Skips when `MINIO_TEST_ENDPOINT` unset.
- **Single-container deployment stack**:
  - `scripts/build-services.sh`: docker golang:1.25-alpine builder
    that compiles all 10 services to `/tmp/bidwriter-bin` (host has
    no Go toolchain).
  - `scripts/start-stack.sh`: launches the 10 binaries in one
    `alpine:3.20` container via supervisor (`stack-entrypoint.sh`),
    exposing only api-gateway (7080) to the host.
  - `scripts/stack-entrypoint.sh`: per-service env (HTTP_ADDR,
    SERVICE_NAME, PORT) + derives REDIS_ADDR from REDIS_URL for
    workflow-svc since it reads REDIS_ADDR, not REDIS_URL.

### Fixed
- **Login-to-home flow** (the "clicked login but can't get into the
  home page" report): LoginPage pre-filled password was `admin123`
  (hard-coded) but the real bcrypt-hashed demo password is
  `password123`. Same typo in the test-account hint below the form,
  so submitting the default form 401'd every time. Both the default
  state and the hint now read `password123`.
- **Root route renders empty**: `App.tsx` mounted the protected
  layout under `path="/*"` with no index route, so visiting `/`
  rendered an empty `<Outlet />`. Changed to `path="/"` with an
  explicit `<Route index element={<Navigate to="/bids" replace />} />`
  so the root path now redirects into the bids list. Also avoids
  the React Router v7 splat-resolution warning.
- **workflow-svc redis disconnect**: `config.Load()` reads REDIS_ADDR
  while docker run injected REDIS_URL, so asynq workers tried to
  dial `[::1]:6379` (the default) and Dequeue error'd every 2s.
  `stack-entrypoint.sh` now derives REDIS_ADDR from REDIS_URL when
  launching workflow-svc, falling back to `host.docker.internal:6390`.
- **workflow-svc test cascade**: silentLogger redeclared,
  AuditClient httptest chunked-deadlock, fakeEnqueuer missing
  EnqueueAllTasks/EnqueueExport. All three now compile and pass.

### Changed
- **package-lock.json** synced with the dev deps that were added
  when the vitest + RTL test suite landed (LoginPage, Layout,
  toast, useHotkey). The deps were installed into node_modules
  during the previous session but the lockfile wasn't committed,
  so fresh installs on a clean host would have skipped them.

### Previous session entries

- **Web component tests** (vitest + @testing-library/react): `LoginPage`
  (5 cases) and `Layout` (5 cases). Covers render, submit, error path,
  loading state, navigation, role-based UI, and the "trailing path
  active" behaviour of `Layout`.  `src/test/setup.ts` now imports
  `@testing-library/jest-dom/vitest` for `toBeInTheDocument()` and
  calls `cleanup()` after each test.
- **Go benchmarks** for the hot paths so future regressions are
  visible:
  - `shared/pkg/httperr` — `BenchmarkWrite` / `BenchmarkWriteNoDetails`
    (error-envelope cost).
  - `shared/pkg/validator` — `BenchmarkValidate` (full struct) plus
    `BenchmarkHex64Only` / `BenchmarkMimeOnly` (single-rule cost).
  - `services/api-gateway/internal/ratelimit` — `BenchmarkAllowSingleKey`,
    `BenchmarkAllowManyKeys`, `BenchmarkAllowConcurrent` (mutex scaling).
  - `services/workflow-svc/internal/api` — `BenchmarkOoxmlBuilder_DefaultOutline`,
    `BenchmarkOoxmlBuilder_LargeBid` (DOCX generation cost — the
    regression gate for the planned unioffice / gooxml swap).
- **Export handler end-to-end tests** for the two endpoints that were
  still uncovered: `TestExportWordHandler_Success`,
  `TestExportDocumentHandler_Success`, `TestExportWordHandler_InvalidID`,
  `TestExportDocumentHandler_NotFound`, `TestExportDocumentHandler_InvalidJSON`.
  Uses the existing `fakeBackend` pattern and goes through the real
  route mux, so it pins the contract for the new POST `/export` path.
- **Toast notification system** (`web/src/lib/toast.ts` +
  `web/src/components/ToastContainer.tsx`): success / error / info /
  warning tones, auto-dismiss, top-right slide-in. Wired into every
  mutation in `BidsPage` / `BidWorkspace` / `KnowledgePage` /
  `ExportPage`.
- **Keyboard shortcut hook** `useHotkey` in
  `web/src/hooks/useHotkey.ts`, used in `BidWorkspace` to bind
  `Cmd/Ctrl+S` to "save current edit".
- **In-flight generation banner** in `BidWorkspace`: pulses while
  chapters are running and reports in-flight + failed counts.
- **New page** `web/src/pages/settings/SettingsPage.tsx` (account info,
  notification preferences, system info) — wired into the Layout nav
  so `/settings` no longer 404s.
- **Skeleton loaders** in `web/src/components/ui/Skeleton.tsx`:
  `Skeleton` / `SkeletonText` / `SkeletonCard` /
  `SkeletonStatCard` / `SkeletonTableRow`. Used in `BidsPage` and
  `KnowledgePage` for consistent loading states.
- **Reusable UI primitives** in `web/src/components/ui/`:
  `Badge` / `StatusBadge`, `Button`, `Card`, `Modal` (portal + ESC
  close + backdrop blur), `EmptyState`, `StatCard`, `ProgressBar`,
  `Field` / `TextInput` / `TextArea` / `Select`, `Logo`.
- **Tests** (vitest): `web/src/lib/toast.test.ts` (4 cases) and
  `web/src/hooks/useHotkey.test.ts` (4 cases).

### Changed
- Test setup (`src/test/setup.ts`): adds RTL `cleanup()` so each
  `render()` mounts into a fresh DOM, and registers `jest-dom`
  matchers.
- **Audit / Template / Billing / Notify handler tests** (cd3a64d): added
  handler-level tests for the previously uncovered services and the
  consumer-defined interfaces they need.
- **End-to-end infrastructure** (e277515): `infra/docker-compose.yml`
  to bring up the full local stack (Postgres, Redis, all 6 services,
  Caddy); `docs/INFRA.md` for the daily workflow; and a
  cross-service smoke test (`scripts/smoke-e2e.sh`, 11/11 green)
  that hits real HTTP endpoints through the gateway and verifies
  DB persistence end-to-end.
- **Benchmark regression guard** (538be05): a CI workflow
  (`.github/workflows/bench-guard.yml`) that runs every push/PR
  the eight hot-path benchmarks, stores the numbers in the
  `bench-results` branch, and fails the build when any benchmark
  regresses by more than the configured `BUDGET_PCT` (default 15%).
  Includes a stable `benchstat -split=package` style summary,
  machine-readable JSON artifacts, and a `bench/budgets.json`
  file that lets teams set per-benchmark thresholds. See
  `docs/BENCH_GUARD.md` for the full design and how to update
  baselines after an intentional perf change.
- **Frontend beautification** (2026-07-03):
  - Design system tokens (brand + ink palettes, Inter + JetBrains
    Mono fonts, soft/pop/inset shadows, fade/slide/scale/shimmer
    animations, custom scrollbar utilities).
  - Layout: dark sidebar with brand mark, multi-line nav, user
    avatar, polished logout button.
  - LoginPage: split hero (brand panel + feature grid) and form
    panel with inline error + loading state.
  - BidsPage: 4 stat cards, search + segmented status filter,
    stagger-animated cards with status badges and progress bars,
    branded modal for creation.
  - BidWorkspace: rewritten as 5 components (`WorkspaceHeader`,
    `MaterialPanel`, `ChapterTree`, `ChapterEditor`,
    `ChapterInspector`); top bar shows a 7-step workflow stepper;
    editor has word-count warning + meta row; inspector surfaces
    priority / word / style / prompt / config in distinct sections.
  - KnowledgePage: 4 stat cards, 7 category filter pills,
    color-coded cards with icons, drag-and-drop upload area.
  - ExportPage: hero header with breadcrumb, two format cards with
    feature lists, dedicated "未就绪" amber card.
  - index.html: Inter / JetBrains Mono preconnect, theme-color meta,
    `favicon.svg` moved to `/public` (fixes vite URI decode error).

### Test counts
- Backend: **264** Test functions + **10** Benchmark functions
  across **36** `_test.go` files (was 227 / 28).
- Web: **32** vitest cases across **6** `*.test.{ts,tsx}` files
  (was 23 / 4).
- E2E: 11/11 cross-service smoke checks green
  (`scripts/smoke-e2e.sh`).

### Notes
- The pre-existing `CI` workflow (docs lint + link check + mkdocs
  strict) is **not** exercised by this push and is **not** green on
  `main`. It has been failing on 14 of the last 16 pushes predating
  this session (markdownlint glob negation is ignored, and
  `tools/mermaid-lint.mjs` cannot find Chrome in CI). See
  [`docs/KNOWN_ISSUES.md`](docs/KNOWN_ISSUES.md) for the breakdown
  and fix sketches. Backend / Web / E2E / Bench all pass independently.

### How to run benchmarks
```bash
# Hot-path baselines (one-liner per package)
cd backend
( cd shared/pkg/httperr   && go test -bench . -benchmem -benchtime=1s )
( cd shared/pkg/validator && go test -bench . -benchmem -benchtime=1s )
( cd services/api-gateway && go test -bench . -benchmem -benchtime=1s ./internal/ratelimit/ )
( cd services/workflow-svc && go test -bench . -benchmem -benchtime=1s ./internal/api/ )
```

Sample output (Intel i5-8400, 6 cores):
```
httperr    BenchmarkWrite              862 ns/op    883 B/op   7 allocs/op
httperr    BenchmarkWriteNoDetails     456 ns/op    396 B/op   2 allocs/op
validator  BenchmarkValidate         1132 ns/op      0 B/op   0 allocs/op
validator  BenchmarkHex64Only         747 ns/op     16 B/op   1 allocs/op
validator  BenchmarkMimeOnly          486 ns/op     16 B/op   1 allocs/op
ratelimit  BenchmarkAllowSingleKey     75 ns/op      0 B/op   0 allocs/op
ratelimit  BenchmarkAllowManyKeys      89 ns/op      0 B/op   0 allocs/op
ratelimit  BenchmarkAllowConcurrent   170 ns/op      0 B/op   0 allocs/op
ooxml      BenchmarkDefaultOutline    219 µs/op   9.7 KB/op  40 allocs/op
ooxml      BenchmarkLargeBid          233 µs/op  10.1 KB/op  42 allocs/op
```