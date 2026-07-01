#!/usr/bin/env python3
"""
Local CI validation script.

Validates:
1. YAML syntax (all *.yml / *.yaml files)
2. GitHub Actions workflow schema basics
3. mkdocs.yml structure
4. dependabot.yml structure
5. ISSUE_TEMPLATE files have required fields
6. PR_TEMPLATE has required sections
7. CODEOWNERS syntax
8. Markdown frontmatter in ISSUE_TEMPLATEs
9. Cross-references between docs (mkdocs nav vs actual files)
"""

import os
import sys
import yaml
import re
from pathlib import Path

ROOT = Path(os.environ.get("PROJECT_ROOT", Path(__file__).resolve().parents[2]))
ERRORS = []
WARNINGS = []
OK = []


def err(msg):
    ERRORS.append(msg)


def warn(msg):
    WARNINGS.append(msg)


def ok(msg):
    OK.append(msg)


# ----------------------------------------------------------------------
# 1. YAML syntax
# ----------------------------------------------------------------------
def check_yaml_syntax():
    print("\n=== 1. YAML Syntax ===")
    # Exclude mkdocs build output and vendored dirs
    SKIP_DIRS = {"site", ".git", "node_modules", "venv", ".venv"}
    yaml_files = []
    for f in list(ROOT.rglob("*.yml")) + list(ROOT.rglob("*.yaml")):
        rel = f.relative_to(ROOT)
        if any(part in SKIP_DIRS for part in rel.parts):
            continue
        yaml_files.append(f)

    # 注册 mkdocs / PyYAML 常见自定义 tag
    def construct_python_name(loader, suffix, node):
        return f"{suffix}.{node.value}"

    def construct_python_object(loader, suffix, node):
        return loader.construct_scalar(node)

    class MkdocsLoader(yaml.SafeLoader):
        pass

    MkdocsLoader.add_multi_constructor(
        "tag:yaml.org,2002:python/name:", construct_python_name
    )
    MkdocsLoader.add_multi_constructor(
        "tag:yaml.org,2002:python/object:", construct_python_object
    )

    for f in yaml_files:
        rel = f.relative_to(ROOT)
        loader_class = MkdocsLoader if rel.name == "mkdocs.yml" else yaml.SafeLoader
        try:
            data = yaml.load(f.read_text(), Loader=loader_class)
            ok(f"{rel} ✓ valid YAML")
        except yaml.YAMLError as e:
            err(f"{rel} ✗ YAML parse error: {e}")
            continue

        # 2. GitHub Actions workflow checks
        if ".github/workflows/" in str(rel) and not rel.name.startswith("dependabot"):
            check_workflow(rel, data)

        if rel.name == "dependabot.yml":
            check_dependabot(rel, data)

        if rel.name == "mkdocs.yml":
            check_mkdocs(rel, data)


# ----------------------------------------------------------------------
# 2. GitHub Actions workflow
# ----------------------------------------------------------------------
def check_workflow(rel, data):
    # top-level keys
    # NOTE: PyYAML parses bare `on:` as True. Some workflows use
    # `True:` (or quoted `"on":`). We accept both.
    triggers = data.get("on", data.get(True))
    required_keys = {"name", "jobs"}
    missing = required_keys - data.keys()
    if triggers is None:
        missing.add("on")
    if missing:
        err(f"{rel}: missing top-level keys {missing}")
    else:
        ok(f"{rel}: has required keys {required_keys} (+ on)")

    # jobs must be dict
    jobs = data.get("jobs", {})
    if not isinstance(jobs, dict) or not jobs:
        err(f"{rel}: 'jobs' must be non-empty dict")
        return

    for job_name, job in jobs.items():
        # runs-on
        if "runs-on" not in job:
            err(f"{rel}: job '{job_name}' missing 'runs-on'")
        # steps
        if "steps" not in job:
            err(f"{rel}: job '{job_name}' missing 'steps'")
            continue
        steps = job["steps"]
        if not isinstance(steps, list) or not steps:
            err(f"{rel}: job '{job_name}' 'steps' must be non-empty list")
            continue
        for i, step in enumerate(steps):
            if not isinstance(step, dict):
                err(f"{rel}: job '{job_name}' step {i} not a dict")
                continue
            if "uses" not in step and "run" not in step:
                err(f"{rel}: job '{job_name}' step {i} has no 'uses' or 'run'")


# ----------------------------------------------------------------------
# 3. dependabot.yml
# ----------------------------------------------------------------------
def check_dependabot(rel, data):
    if data.get("version") != 2:
        warn(f"{rel}: version should be 2")
    updates = data.get("updates", [])
    if not isinstance(updates, list) or not updates:
        err(f"{rel}: 'updates' must be non-empty list")
        return
    seen = set()
    for upd in updates:
        eco = upd.get("package-ecosystem")
        if not eco:
            err(f"{rel}: update entry missing package-ecosystem")
            continue
        key = (eco, upd.get("directory"))
        if key in seen:
            err(f"{rel}: duplicate ({eco}, {key[1]})")
        seen.add(key)
        if "directory" not in upd:
            err(f"{rel}: update '{eco}' missing directory")
        if "schedule" not in upd:
            warn(f"{rel}: update '{eco}' missing schedule")
    ok(f"{rel}: dependabot.yml valid")


# ----------------------------------------------------------------------
# 4. mkdocs.yml
# ----------------------------------------------------------------------
def check_mkdocs(rel, data):
    if "site_name" not in data:
        err(f"{rel}: missing site_name")
    if "nav" in data:
        nav = data["nav"]
        # validate nav file paths exist
        check_nav_paths(rel, nav, ROOT / "docs")
    if "theme" not in data:
        warn(f"{rel}: no theme configured")
    if "plugins" not in data:
        warn(f"{rel}: no plugins configured")
    ok(f"{rel}: mkdocs.yml structure valid")


def check_nav_paths(yaml_path, nav, base):
    """Walk nav tree and check referenced files exist."""
    if isinstance(nav, list):
        for item in nav:
            check_nav_paths(yaml_path, item, base)
    elif isinstance(nav, dict):
        for key, val in nav.items():
            check_nav_paths(yaml_path, val, base)
    elif isinstance(nav, str):
        # file path
        target = base / nav
        # nav files are typically without .md suffix in mkdocs
        if not target.exists():
            target_with_ext = base / f"{nav}.md"
            target_index = base / nav / "index.md"
            if target_with_ext.exists() or target_index.exists():
                return
            err(f"{yaml_path}: nav references missing file '{nav}'")


# ----------------------------------------------------------------------
# 5. ISSUE_TEMPLATE frontmatter
# ----------------------------------------------------------------------
def check_issue_templates():
    print("\n=== 5. ISSUE_TEMPLATE ===")
    tpl_dir = ROOT / ".github" / "ISSUE_TEMPLATE"
    if not tpl_dir.exists():
        err(".github/ISSUE_TEMPLATE missing")
        return
    for f in sorted(tpl_dir.glob("*.md")):
        rel = f.relative_to(ROOT)
        content = f.read_text()
        # YAML frontmatter
        m = re.match(r"^---\n(.*?)\n---\n", content, re.DOTALL)
        if not m:
            err(f"{rel}: missing YAML frontmatter")
            continue
        try:
            fm = yaml.safe_load(m.group(1))
        except yaml.YAMLError as e:
            err(f"{rel}: frontmatter YAML error: {e}")
            continue
        # required
        for k in ("name", "description", "title", "labels"):
            if k not in fm:
                warn(f"{rel}: frontmatter missing '{k}'")
        # labels must be list
        if "labels" in fm and not isinstance(fm["labels"], list):
            err(f"{rel}: 'labels' must be list")
        ok(f"{rel}: ISSUE_TEMPLATE valid ({fm.get('name')})")


# ----------------------------------------------------------------------
# 6. PR_TEMPLATE
# ----------------------------------------------------------------------
def check_pr_template():
    print("\n=== 6. PR_TEMPLATE ===")
    f = ROOT / ".github" / "PULL_REQUEST_TEMPLATE.md"
    if not f.exists():
        err(".github/PULL_REQUEST_TEMPLATE.md missing")
        return
    content = f.read_text()
    if "## 📋 描述" not in content and "## Description" not in content:
        warn(f"{f.name}: missing description section")
    if "## 🔗 关联" not in content and "## Related" not in content:
        warn(f"{f.name}: missing related section")
    ok("PULL_REQUEST_TEMPLATE.md present")


# ----------------------------------------------------------------------
# 7. CODEOWNERS
# ----------------------------------------------------------------------
def check_codeowners():
    print("\n=== 7. CODEOWNERS ===")
    f = ROOT / ".github" / "CODEOWNERS"
    if not f.exists():
        err(".github/CODEOWNERS missing")
        return
    content = f.read_text()
    lines = [l for l in content.splitlines() if l.strip() and not l.strip().startswith("#")]
    if not lines:
        err(".github/CODEOWNERS has no rules")
        return
    # very basic syntax
    bad = [l for l in lines if not re.match(r"^[/\*\.]", l.strip())]
    if bad:
        warn(f"CODEOWNERS: some lines don't start with / or *: {bad[:3]}")
    ok(f"CODEOWNERS: {len(lines)} rules present")


# ----------------------------------------------------------------------
# 8. Markdown cross-references
# ----------------------------------------------------------------------
def check_markdown_links():
    print("\n=== 8. Markdown cross-refs ===")
    md_files = list(ROOT.rglob("*.md"))
    md_files = [f for f in md_files if "/node_modules/" not in str(f) and "/.git/" not in str(f)]

    # Files that intentionally contain placeholder/example links
    SKIP = {"docs/development/documentation-style.md"}

    issues = 0
    for f in md_files:
        rel = f.relative_to(ROOT)
        if str(rel) in SKIP:
            ok(f"{rel}: skipped (intentional example links)")
            continue
        content = f.read_text()
        for m in re.finditer(r"\[([^\]]+)\]\(([^)]+)\)", content):
            link = m.group(2)
            if link.startswith(("http://", "https://", "#", "mailto:")):
                continue
            path = link.split("#")[0]
            if not path:
                continue
            target = (f.parent / path).resolve()
            if not target.exists():
                warn(f"{rel}: broken link '{link}'")
                issues += 1
    if issues == 0:
        ok("all markdown local links resolve")
    else:
        warn(f"{issues} broken markdown link(s)")


# ----------------------------------------------------------------------
# Main
# ----------------------------------------------------------------------
def main():
    print("=" * 60)
    print("BidWriter Local CI Validation")
    print("=" * 60)

    check_yaml_syntax()
    check_issue_templates()
    check_pr_template()
    check_codeowners()
    check_markdown_links()

    print("\n" + "=" * 60)
    print(f"OK: {len(OK)}")
    print(f"WARNINGS: {len(WARNINGS)}")
    print(f"ERRORS: {len(ERRORS)}")
    print("=" * 60)

    if WARNINGS:
        print("\n--- WARNINGS ---")
        for w in WARNINGS:
            print(f"  ⚠ {w}")

    if ERRORS:
        print("\n--- ERRORS ---")
        for e in ERRORS:
            print(f"  ✗ {e}")
        sys.exit(1)

    print("\n✅ All CI files validated successfully.")


if __name__ == "__main__":
    main()