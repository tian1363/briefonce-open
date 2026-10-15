#!/usr/bin/env python3
"""Check public docs and tracked source without printing suspected secret values."""
from pathlib import Path
import re
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]
FORBIDDEN_PATHS = (
    "docs/admin-operations.md", "docs/admin-user-manual.md",
    "docs/image-provider-migration-requirements.md", "docs/wan3-video.md",
    "docs/proactive-agent.md", "docs/social/", "docs/assets/admin-",
    "private-docs/", "internal-operations/",
)
PRIVATE_DOC_TERMS = re.compile(
    r"毛利|利润|成本倍数|两倍成本|收入测算|供应商对账|供应商充值|"
    r"只读查看用户|只读用户工作台|只读视图|read-only user workspace|"
    r"gross profit|profit margin|cost multiplier", re.I
)
RETIRED_TERMS = re.compile(r"creatibi|wan.?3|视频字幕换版|换文案|主动建议", re.I)
CREDENTIALS = re.compile(
    r"sk-(?:proj-)?[A-Za-z0-9_-]{24,}|ghp_[A-Za-z0-9]{30,}|"
    r"github_pat_[A-Za-z0-9_]{40,}|AKIA[A-Z0-9]{16}|"
    r"-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----"
)
ASSIGNMENTS = re.compile(
    r'''(?i)(?:api_key|apikey|password|access_token|refresh_token)\s*[:=]\s*["']([A-Za-z0-9_+/=-]{16,})["']'''
)
PLACEHOLDER = re.compile(r"your[-_]|example|placeholder|dummy|mock|test[-_]|replace|^x+$", re.I)


def check():
    paths = subprocess.check_output(["git", "ls-files", "-z"], cwd=ROOT).decode().split("\0")
    issues = []
    for relative in paths:
        if not relative:
            continue
        path = ROOT / relative
        if not path.is_file():
            continue
        if any(relative == rule or relative.startswith(rule) for rule in FORBIDDEN_PATHS):
            issues.append((relative, "restricted file"))
        if path.stat().st_size > 4_000_000:
            continue
        try:
            text = path.read_text(encoding="utf-8")
        except UnicodeError:
            continue
        if relative == "scripts/check-public-content.py":
            continue
        if CREDENTIALS.search(text):
            issues.append((relative, "credential pattern"))
        public_doc = relative.endswith(".md") or (relative.startswith("docs/") and path.suffix in {".html", ".txt", ".csv"})
        public_site = relative.startswith("web/website/") and not relative.startswith("web/website/tests/") and path.suffix in {".html", ".jsx", ".js", ".json"}
        if public_doc or public_site:
            if PRIVATE_DOC_TERMS.search(text):
                issues.append((relative, "internal business or access detail"))
            if RETIRED_TERMS.search(text):
                issues.append((relative, "retired feature"))
            for match in ASSIGNMENTS.finditer(text):
                if not PLACEHOLDER.search(match.group(1)):
                    issues.append((relative, "non-placeholder credential assignment"))
                    break
        if relative.startswith(("docs/", "web/website/")) and re.search(r"/Users/[A-Za-z0-9_.-]+/|local-admin@scriptagent\.test", text):
            issues.append((relative, "private workstation or account identifier"))
        if relative in {"internal/web/pose_catalog.json", "internal/web/pose_collections_archive.json"} and re.search(r"(?:xsec_token|access_token|refresh_token)=", text):
            issues.append((relative, "session-bound source link"))
    for filename, category in sorted(set(issues)):
        print(f"FAIL {filename}: {category}", file=sys.stderr)
    if issues:
        return 1
    print("Public content checks passed. Historical Git objects and screenshots require separate review.")
    return 0


if __name__ == "__main__":
    raise SystemExit(check())
