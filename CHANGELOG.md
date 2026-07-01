# Changelog

All notable changes to this project will be recorded here. The format is
loosely based on [Keep a Changelog](https://keepachangelog.com/); dates
in `YYYY-MM-DD`.

## [Unreleased]

### Added
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

### Test counts
- Backend: **264** Test functions + **10** Benchmark functions
  across **36** `_test.go` files (was 227 / 28).
- Web: **23** vitest cases across **4** `*.test.{ts,tsx}` files
  (was 13 / 2).
- E2E: 11/11 cross-service smoke checks green
  (`scripts/smoke-e2e.sh`).

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
ratelimit  BenchmarkAllowConcurrent   170 ns/op      8 B/op   1 allocs/op
ooxml      BenchmarkOoxmlBuilder_DefaultOutline  218797 ns/op  36102 B/op  97 allocs/op
ooxml      BenchmarkOoxmlBuilder_LargeBid       233327 ns/op  57224 B/op 107 allocs/op
```
