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

### Changed
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