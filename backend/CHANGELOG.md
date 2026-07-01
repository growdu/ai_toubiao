# Changelog

All notable changes to BidWriter will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial project setup
- Documentation structure (docs/ with mkdocs)
- GitHub Actions CI/CD
- GitHub Pages deployment
- AGENTS.md / CONTRIBUTING.md
- 7 ADR (Architecture Decision Records)
- BidWriter 框架设计文档
- **workflow-svc unit tests**: state machine (LinearPlan, CanTransition,
  Validate, StepForState, NextState), handler HTTP paths (create/get/list/
  transition/listSteps/listEvents) with fake backend + httptest, export
  contract (ooxmlBuilder zip validity, buildDocumentXML escape and heading
  clamping, defaultChapters, exportWordHandler, exportPDFHandler fallback,
  libreOfficeConverter Available semantics), and planner pure-function
  helpers (parseChapterOutline, defaultChapterOutline).
- **audit-svc unit tests**: per-rule compliance check coverage
  (expired certificate, repeated revenue, dark-bid, personnel spam),
  ChapterAuditor (empty/skipped/short content, dark-bid trigger),
  RejectionChecker (forbidden patterns, starred-clause suppression, nil
  content), CrossAuditor (duplicate detection, length guard) and
  similarStrings.
- **billing-svc unit tests**: service (GetCurrentBudget percent calc with
  limit=0 / limit>0 / spent=0 / 50% / store error propagation,
  SetBudget flows-through, GetTransactions limit normalization for 0 /
  negative / explicit) and api (envelope {data:...}, 401 on missing
  tenant, 400 on bad JSON, 500 on service error, 201 on add, query
  passthrough for ?limit=N).
- **notify-svc unit tests**: service (Send happy path with
  channel-dispatch seam, Send unknown channel + email/dingtalk/wecom
  "not implemented" all update log to failed, CreateLog error short-
  circuits, UpdateLog error logged not propagated, all 3 NotifyX
  "no enabled prefs" branches, preference CRUD pass-through) and api
  (send/preferences CRUD, multipart not required, 401/400/500 paths,
  healthz).
- **template-svc unit tests**: service (Upload happy path with
  IsDefault=true/false, ClearDefault failure rolls back storage,
  store.Create failure rolls back storage, storage.Put failure
  short-circuits; Download happy path + 3-tuple-on-error;
  Delete cleanup + storage.Delete-failure-swallowed; Update
  promotes-to-default clears defaults first) and api (list/get/
  update/delete 401+404+400+204+200 paths, multipart upload
  propagates name/kind/filename/size, no-file 400, download
  Content-Type + Content-Disposition headers, healthz).
- **project-svc unit tests**: auth.Verifier (round-trip, wrong secret,
  expired, malformed, bad tenant UUID, bad sub UUID, none-alg
  rejection, issuer/iat/exp claims) and api (full CRUD with real
  JWT middleware + auth.IssueToken; list returns {data,meta}, 400 on
  bad cursor, 401 without token, create 201 + owner_id from
  auth context + default currency CNY, validation 400, update 200
  / 409 version conflict / 404, delete 204 / 404, healthz 200).

### Changed
- **workflow-svc export**: extracted `DocBuilder` and `PDFConverter`
  interfaces so the default `ooxmlBuilder` can be swapped (e.g. for
  unioffice) and the LibreOffice path is testable. PDF endpoints now
  fall back to DOCX with an `X-Export-Fallback` warning header when
  soffice is unavailable, instead of silently returning a DOCX with a
  `.docx` extension. Added `LIBREOFFICE_BIN` env var + cmd wiring for
  binary override.
- **workflow-svc handlers**: introduced a consumer-defined
  `WorkflowBackend` interface (in `api/`) so handlers can be unit-tested
  with a fake without spinning up Postgres. `*store.Store` already
  satisfies it; `cmd/workflow-svc/main.go` wiring is unchanged.
- **billing-svc service + api**: declared `Store` interface in
  `service/` and `billingService` interface in `api/`; both let the
  layers be unit-tested with hand-rolled fakes. `*store.Store` and
  `*service.BillingService` still satisfy them; cmd wiring unchanged.
- **notify-svc service + api**: same pattern — `Store` interface in
  `service/`, `notifyService` interface in `api/`. Added a
  package-level `channelSenders` seam so tests can override the
  email/dingtalk/wecom dispatch without touching the real (unimplemented)
  senders. Fixed a latent nil-deref: `sendErr.Error()` was called even
  when the send succeeded; now guarded by an `errMsg == ""` check.
  Removed the unused `Handlers.Store` field; cmd/main.go updated.
- **template-svc service + api**: same pattern — `Store` interface in
  `service/`, `Service` interface in `api/`. Removed the unused
  `Handlers.Store` field; cmd/main.go updated.
- **project-svc api**: declared `ProjectStore` interface in `api/` so
  handlers can be unit-tested with a fake store. Auth (JWT) is exercised
  in tests via the real `auth.Verifier` + `auth.IssueToken` (HS256)
  against a fixed test secret — no live auth-svc needed.
- **API Gateway**: rewrote proxy route prefix from `/api/v1/workflows` to `/api/v1/bids` so the public surface matches the frontend `bidsApi`. Refreshed the package-level routing-table comment to reflect the actual proxy prefixes (`projects`, `documents`, `bids`) and explicitly note `knowledge` as not yet proxied. (api-gateway/cmd/api-gateway/main.go)
- **workflow-svc**: mounted export endpoints on the new `/api/v1/bids/{id}/...` mount, exposing `GET /export/word`, `GET /export/pdf`, and `POST /export` (with chapter payload). The first two accept an empty body and fill in default chapter stubs; PDF currently falls back to Word output pending a LibreOffice pipeline. (workflow-svc/internal/api/{handlers.go,export.go})
- **web ExportPage**: replaced `window.open(...)` (which silently dropped the JWT) with an axios `responseType: 'blob'` download driven by the existing auth interceptor. Added per-format loading state and inline error banner, plus a `download` filename parsed from `Content-Disposition` when present. (web/src/pages/bids/ExportPage.tsx, web/src/api/bids.ts)

## [0.1.0] - TBD

### Added
- Multi-tenant architecture (row-level tenant_id)
- API Gateway with JWT authentication
- Workflow Step01-06 state machine
- Multi-model AI router (OpenAI / Anthropic / DeepSeek / Ollama)
- 6 industry templates
- Consistency audit (normal mode)
- Word export (.docx)
- Real-time SSE progress
- Knowledge base (3 layers)

[Unreleased]: https://github.com/yourorg/bidwriter/compare/v0.1.0...HEAD