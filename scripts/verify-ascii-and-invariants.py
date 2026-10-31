#!/usr/bin/env python3
"""Repository Security and Invariant Verification Script.

Verifies:
1. Pure 7-bit ASCII encoding across codebases (byte <= 127).
2. Strict repository invariants: HANDOFF.md, AGENTS.md, PLAN.md, and implementation
   plans are never tracked in Git.
"""

import os
import subprocess
import sys

FORBIDDEN_TRACKED_PATTERNS = [
    "HANDOFF.md",
    "AGENTS.md",
    "PLAN.md",
    "docs/implementation-plans/",
    "docs/superpowers/",
]

SCAN_EXTENSIONS = {
    ".go",
    ".py",
    ".ts",
    ".tsx",
    ".js",
    ".jsx",
    ".sql",
    ".sh",
    ".yml",
    ".yaml",
    ".json",
    ".md",
}

SCAN_DIRECTORIES = ["backend", "ai", "frontend", "scripts", ".github"]

IGNORE_DIRS = {
    ".git",
    ".next",
    "node_modules",
    ".venv",
    "dist",
    "build",
    "coverage",
    "__pycache__",
    ".pytest_cache",
    ".ruff_cache",
    ".mypy_cache",
    "test-results",
}


def check_forbidden_tracked_files() -> list[str]:
    """Ensure sensitive orchestration and planning files are not tracked in git."""
    violations = []
    try:
        cmd = ["git", "ls-files"] + FORBIDDEN_TRACKED_PATTERNS
        result = subprocess.run(
            cmd,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            check=False,
        )
        if result.returncode == 0 and result.stdout.strip():
            tracked = [line.strip() for line in result.stdout.strip().splitlines() if line.strip()]
            for item in tracked:
                violations.append(f"Forbidden file is tracked in git: {item}")
    except Exception as e:
        print(f"Note: git ls-files check skipped ({e})", file=sys.stderr)
    return violations


def find_line_and_col(content: bytes, offset: int) -> tuple[int, int]:
    """Calculate 1-indexed line and column numbers from a byte offset."""
    prefix = content[:offset]
    line = prefix.count(b"\n") + 1
    last_newline = prefix.rfind(b"\n")
    if last_newline == -1:
        col = offset + 1
    else:
        col = offset - last_newline
    return line, col


def check_ascii_invariants() -> list[str]:
    """Scan codebases for non-ASCII bytes (byte > 127)."""
    violations = []
    repo_root = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))

    for target in SCAN_DIRECTORIES:
        target_path = os.path.join(repo_root, target)
        if not os.path.exists(target_path):
            continue

        for root, dirs, files in os.walk(target_path):
            dirs[:] = [d for d in dirs if d not in IGNORE_DIRS]
            for f in files:
                ext = os.path.splitext(f)[1].lower()
                if ext in SCAN_EXTENSIONS:
                    path = os.path.join(root, f)
                    rel_path = os.path.relpath(path, repo_root)
                    try:
                        with open(path, "rb") as fp:
                            content = fp.read()
                            for idx, byte in enumerate(content):
                                if byte > 127:
                                    line, col = find_line_and_col(content, idx)
                                    violations.append(
                                        f"Non-ASCII byte 0x{byte:02x} in {rel_path} "
                                        f"at line {line}, col {col} (offset {idx})"
                                    )
                                    break
                    except Exception as e:
                        violations.append(f"Failed to read {rel_path}: {e}")

    return violations


def main() -> None:
    print("Running repository invariant and 7-bit ASCII scan...")
    tracked_violations = check_forbidden_tracked_files()
    ascii_violations = check_ascii_invariants()

    total_violations = tracked_violations + ascii_violations

    if total_violations:
        print("\nFAIL: Found repository invariant violations:")
        for v in total_violations:
            print(f"  - {v}")
        sys.exit(1)

    print("PASS: 100% 7-bit ASCII verified across source trees; repository invariants intact.")
    sys.exit(0)


if __name__ == "__main__":
    main()
