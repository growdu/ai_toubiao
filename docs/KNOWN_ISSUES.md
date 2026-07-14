# Known Issues

Documentation entries for open problems that are tracked but not yet fixed.
Each item should be a single, actionable path forward — not a dump.

---

## CI workflow red on `main` — predates 2026-07-01

**Status:** `CI` workflow has been `failure` on the last 14 of 16 pushes.
The test code itself (264 backend + 23 web + 11/11 e2e + bench-guard)
is green; only the docs lint jobs are red.

**Affected jobs** (see `.github/workflows/ci.yml`):

1. `Markdown lint` (job `markdown-lint`)
2. `Mermaid lint` (job `mermaid-lint`)

Both are gated by the final `CI Status` job, so any single failure
flips the whole workflow to `failure`.

### 1. Markdown lint — glob negation ignored

The action is invoked with:

```yaml
- uses: DavidAnson/markdownlint-cli2-action@v19
  with:
    globs: |
      **/*.md
      !node_modules/**/*.md
```

On the runner this expansion still includes `web/node_modules/**/*.md`,
producing ~32 000 false-positive violations (Vite / pnpm cached
licenses with trailing spaces, inline HTML in third-party READMEs,
etc). Ten real project files are also flagged — those need fixing
too, see `./MARKDOWNLINT_BACKLOG.md` once generated.

**Fix sketch.** Two changes:

- Replace the inline `globs:` with the project config file
  (`.markdownlint-cli2.jsonc`), which already declares both
  `**/*.md` and `!node_modules/**`. The action picks this up
  automatically when no `globs:` is provided.
- If inline globs are kept, prepend the absolute path: have the
  action run from the repo root with the negation rewritten as
  `!**/node_modules/**` so `web/node_modules` and any other
  nested `node_modules` are excluded. Validate with the local
  `npx markdownlint-cli2` (no flags) before pushing.

### 2. Mermaid lint — headless render failure

Job `mermaid-lint` runs `node tools/mermaid-lint.mjs`, which
spawns headless Chrome via `puppeteer-core` to render every
` ```mermaid ` block in docs. The failure signature is
"Render mermaid blocks" timing out or returning non-zero,
most likely because the CI image lacks a Chrome binary at
the candidates the script probes, or because `npm ci` over
the cached `package-lock.json` fails to install `puppeteer-core`.

**Fix sketch.** In rough order of preference:

- Pre-bake Chrome into the runner by adding
  `browser-actions/setup-chrome` (or a `sudo apt-get install -y
  chromium-browser`) before the `Render mermaid blocks` step.
- Pin `puppeteer-core` to a version known to work on GitHub-hosted
  `ubuntu-latest` and add `if: hashFiles('web/package-lock.json')`
  to the cache step so the dependency install doesn't get a stale
  tree.
- As a last resort, swap the headless render for a static
  syntax check (`mermaid-cli --validate`) which doesn't need a
  browser at all.

### What is **not** in scope for this entry

- Fixing the actual markdown violations in
  `backend/CONTRIBUTING.md`, `backend/docs/**` etc. — those need
  a separate pass to also re-evaluate the project's style.
- Replacing `tools/mermaid-lint.mjs` with a different validator.
- Changing the `CI Status` job to a softer gate (e.g. `continue-on-error`)
  — that would mask the real failures.

---

**Filed:** 2026-07-01 by the test-coverage session, after observing
the run history via `GET /repos/:owner/:repo/actions/runs?per_page=20`.

---

## Login returns `500 Internal Server Error` despite valid credentials

**Symptom.** POST `/api/v1/auth/login` returns
`{"error":{"code":"INTERNAL_ERROR","message":"服务器内部错误"}}` for users
that exist in the DB. Wrong passwords correctly return `401 UNAUTHORIZED`
(so the route is wired and the handler differentiates correctly); only the
"valid creds" path explodes, because that's the one that calls into PG.

**Confirmed root cause (2026-07-14).** The `postgres` role in
`bidwriter-pg-test` was set to `NOLOGIN`. Because every Go service connects
to PG as `postgres:postgres@…`, the connection is refused with
`FATAL: role "postgres" is not permitted to log in`, the auth service
returns the bare error to the handler, and `loginHandler` maps any non-`ErrInvalidCredentials`
error to `httperr.InternalError` → HTTP 500.

The first-init guarantee is in
`backend/migrations/initdb.d/zz_998_ensure_login.sql` (mounted at
`/tmp/bidwriter-initdb/zz_998_ensure_login.sql` for the manual
`bidwriter-pg-test` container). That script only runs once per fresh
cluster. Any subsequent `ALTER ROLE postgres NOLOGIN` (from an older
PG container, a stray migration, or a manual `psql`) sticks and is not
self-healed by restart.

**Reproduction recipe.** From the host:

```sh
docker exec -u postgres bidwriter-pg-test psql \
  -d bidwriter -c "ALTER ROLE postgres NOLOGIN;"
# now any /auth/login with valid creds returns 500
```

**Fix that ships in this repo (2026-07-14).** The Go stack now self-heals:

1. `backend/scripts/start-stack.sh` calls a new `ensure_pg_login`
   preflight before launching the 11-service container.
2. `ensure_pg_login` first tries to connect via TCP with the same DSN
   the Go services use. If `rolcanlogin=false`, it first tries an
   online `ALTER ROLE … WITH LOGIN SUPERUSER PASSWORD 'postgres'`.
3. If online ALTER is itself refused (because the cluster is rejecting
   logins entirely), `ensure_pg_login` stops `bidwriter-pg-test`, runs
   `postgres --single` inside a `postgres:16` sidecar against the same
   data dir, runs the `ALTER ROLE`, and restarts the container.
4. `ensure_pg_login` is idempotent and silent on the happy path; only
   when a fix is actually applied does it print the `+` repair line.

**Operator's quick check.**

```sh
docker exec -u postgres bidwriter-pg-test psql -d bidwriter \
  -tAc "SELECT rolname, rolcanlogin FROM pg_authid WHERE rolname='postgres';"
```

If `rolcanlogin=f`, run `backend/scripts/start-stack.sh start` once;
it will repair and continue. The same script ships in
`docs/user-guide.md` § "8.2" with the documented manual workaround for
operators who don't want to bring the stack down.

**Related tests:** `backend/services/api-gateway/internal/auth/auth_test.go`
covers `ErrInvalidCredentials` but not the new preflight (the preflight is
host-side, not in the Go binary, so no Go test reaches it). End-to-end
verification is the curl sequence listed in `docs/user-guide.md` §
"8.1 Smoke test the API".

**Filed:** 2026-07-14 by the auth-recovery session.
