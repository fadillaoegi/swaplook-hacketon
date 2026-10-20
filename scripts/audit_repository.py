#!/usr/bin/env python3
"""Check Git inclusion and known local credentials without modifying this project's Git metadata."""

from pathlib import Path
import re
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[1]


def git(args, **kwargs):
    return subprocess.run(["git", *args], capture_output=True, check=True, **kwargs).stdout


excluded = [
    ".env", ".env.production", "swaplook_FE/.env.local", "server.env",
    ".data/youcam/person.jpg", ".data/models/models.json", ".data/probe/receipt.json",
    "tmp/swaplook-api", "swaplook_FE/node_modules/pkg/index.js", ".pnpm-store/cache.bin",
    "swaplook_FE/dist/index.html", "swaplook_FE/test-results/run/trace.zip",
    "swaplook_FE/playwright-report/index.html", "blob-report/report.zip", "traces/session.json",
    "network.har", "trace-session.zip", "coverage/lcov.info", "coverage.out",
    "secret.pem", "private.key", "credentials.json", ".codex/session.json", ".aws/credentials", ".npmrc",
    "scripts/__pycache__/generate_brand.cpython-313.pyc", "swaplook_FE/tsconfig.tsbuildinfo",
]
included = [
    ".env.example", ".github/workflows/ci.yml", ".gitignore", ".dockerignore",
    "README.md", "LICENSE", "AGENTS.md", "Dockerfile", "compose.yaml", "Makefile",
    "swaplook_BE/go.mod", "swaplook_BE/go.sum", "swaplook_BE/migrations/005_settings.sql",
    "swaplook_FE/pnpm-lock.yaml", "swaplook_FE/src/presentation/App.tsx",
    "swaplook_FE/public/brand/swaplook-logo.svg", "swaplook_FE/public/catalog/SL-001.jpg",
    "docs/CATALOG-CREDITS.md", "docs/DEVPOST.md", "docs/screenshots/01-home.png",
    "demo-assets/models/models.json", "demo-assets/models/model-a.png",
]
failures = []
with tempfile.TemporaryDirectory(prefix="swaplook-repo-audit-") as tmp:
    # Real Git pattern matching, independent of machine-specific global excludes.
    git(["init", "--quiet", tmp])
    options = ["--git-dir=" + str(Path(tmp) / ".git"), "--work-tree=" + str(ROOT), "-c", "core.excludesFile=/dev/null"]
    checks = subprocess.run(["git", *options, "check-ignore", "--no-index", "-z", "--stdin"], input="\0".join(excluded + included).encode() + b"\0", capture_output=True, cwd=ROOT)
    assert checks.returncode in (0, 1), "Git ignore verification failed"
    ignored = set(checks.stdout.decode().strip("\0").split("\0"))
    failures += ["Sensitive/generated path is not ignored: " + p for p in excluded if p not in ignored]
    failures += ["Required project file is ignored: " + p for p in included if p in ignored]
    failures += ["Required project file is missing: " + p for p in included if not (ROOT / p).is_file()]
    paths = set(git([*options, "ls-files", "--others", "--exclude-standard", "-z"], cwd=ROOT).decode().strip("\0").split("\0")) - {""}

    actual_repo = subprocess.run(["git", "-C", str(ROOT), "rev-parse", "--show-toplevel"], capture_output=True)
    has_repo = actual_repo.returncode == 0 and Path(actual_repo.stdout.decode().strip()).resolve() == ROOT
    tracked = set()
    if has_repo:
        tracked = set(git(["-C", str(ROOT), "ls-files", "-z"]).decode().strip("\0").split("\0")) - {""}
        for path in tracked:
            check = subprocess.run(["git", *options, "check-ignore", "--no-index", "-q", path], cwd=ROOT)
            if check.returncode == 0:
                failures.append("Already tracked despite ignore rules: " + path)
        paths |= tracked

    values = []
    env_file = ROOT / ".env"
    if env_file.is_file():
        for line in env_file.read_text().splitlines():
            key, sep, value = line.strip().removeprefix("export ").partition("=")
            value = value.strip().strip('"').strip("'")
            if sep and re.search(r"(?:API_KEY|TOKEN|SECRET|PASSWORD)$", key.strip()) and len(value) >= 16:
                values.append(value.encode())
    signed_url = re.compile(rb"[?&](?:X-Amz-Signature|X-Amz-Credential|X-Goog-Signature)=[^&\s\"<>]+", re.I)
    private_key = re.compile(rb"-----BEGIN (?:RSA |EC |OPENSSH |ENCRYPTED )?PRIVATE KEY-----")
    total = 0
    for name in sorted(paths):
        path = ROOT / name
        if path.is_symlink():
            failures.append("Review symlink before publication: " + name)
            continue
        if not path.is_file():
            continue
        data = path.read_bytes()
        total += len(data)
        if any(value in data for value in values):
            failures.append("Known local credential found in candidate file: " + name)
        if signed_url.search(data) or private_key.search(data):
            failures.append("Potential signed URL/private key found in candidate file: " + name)

print(f"Repository initialized: {'yes' if has_repo else 'no (candidate-file audit only; no commit history exists here)'}")
print(f"Audited {len(paths)} candidate/tracked files ({total:,} bytes); {len(excluded)} exclusion and {len(included)} inclusion cases.")
if failures:
    for message in failures:
        print("FAIL: " + message)
    raise SystemExit(1)
print("PASS: required files retained; private/generated files excluded; no known local credentials, signed URLs or private keys found in candidate contents.")
print("This is not a full Git-history or third-party license audit. No staging, commit, push or provider request was made.")
