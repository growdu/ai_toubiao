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
