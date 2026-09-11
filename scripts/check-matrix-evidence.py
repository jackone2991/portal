#!/usr/bin/env python3
"""Traceability-matrix evidence check (ADR-11). Blocking in CI, with check-links.sh.

docs/reference/TRACEABILITY-MATRIX.md grades each requirement row ✅ / ⚠ / ✖ on
the strength of an Evidence cell that names `_test.go` functions. This script
makes that cell load-bearing:

  * every `path_test.go: TestA, TestB` reference resolves — the file exists under
    backend/internal/ and each named Test function is defined in it;
  * every row marked ✅ has a non-empty Evidence cell that names at least one
    test function (a CI job or a document may also count, but only when the row
    says so in words — see ALLOWED_NON_TEST);
  * every named CI job exists in .github/workflows/ci.yml.

It does not judge whether the named test actually proves the row — that is the
reviewer's call. It only stops the matrix drifting back to a column of ✅ with
nothing behind it, which is exactly how the 2026-08-25 audit found it.

Usage: python3 scripts/check-matrix-evidence.py    (exit 1 on any failure)
"""
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
MATRIX = os.path.join(ROOT, "docs", "reference", "TRACEABILITY-MATRIX.md")
CI = os.path.join(ROOT, ".github", "workflows", "ci.yml")

# `modules/bank/bank_test.go: TestA, TestB` — also `modules/{movie,music}/*_test.go: TestX`
REF = re.compile(r"`((?:[\w{},./*-]+?)_test\.go)(?::\s*([^`]+))?`")
TESTNAME = re.compile(r"\bTest[A-Za-z0-9_]+\b")
JOB = re.compile(r"job `([a-z-]+)`")
ALLOWED_NON_TEST = ("document, not code", "structural:", "CI, not a test")

failures = []


def expand(path_glob):
    """`modules/{movie,music}/*_test.go` → concrete files under backend/internal/."""
    m = re.search(r"\{([^}]+)\}", path_glob)
    variants = [path_glob]
    if m:
        variants = [path_glob.replace(m.group(0), alt) for alt in m.group(1).split(",")]
    out = []
    for v in variants:
        base = os.path.join(ROOT, "backend", "internal", v)
        if "*" in v:
            d, pat = os.path.split(base)
            rx = re.compile("^" + re.escape(pat).replace(r"\*", ".*") + "$")
            if os.path.isdir(d):
                out += [os.path.join(d, f) for f in os.listdir(d) if rx.match(f)]
        else:
            out.append(base)
    return out


def funcs_in(path):
    with open(path, encoding="utf-8") as fh:
        return set(re.findall(r"^func (Test[A-Za-z0-9_]+)\(", fh.read(), re.M))


with open(MATRIX, encoding="utf-8") as fh:
    lines = fh.readlines()
with open(CI, encoding="utf-8") as fh:
    ci_jobs = set(re.findall(r"^  ([a-z-]+):$", fh.read(), re.M))

for n, line in enumerate(lines, 1):
    if not line.startswith("|"):
        continue
    cells = [c.strip() for c in line.strip().strip("|").split("|")]
    if len(cells) < 5 or cells[0] in ("Req", "CC", "Risk", "Spec") or set(cells[0]) <= {"-"}:
        continue
    cov = next((c for c in cells if c in ("✅", "⚠", "✖")), None)
    evidence = cells[-1]
    refs = REF.findall(evidence)
    named = []
    for path_glob, names in refs:
        files = expand(path_glob)
        if not files or not all(os.path.exists(f) for f in files):
            failures.append(f"{n}: file not found: {path_glob}")
            continue
        wanted = TESTNAME.findall(names or "")
        have = set().union(*(funcs_in(f) for f in files))
        for w in wanted:
            if w not in have:
                failures.append(f"{n}: {w} not defined in {path_glob}")
        named += wanted
    for job in JOB.findall(evidence):
        if job not in ci_jobs:
            failures.append(f"{n}: CI job `{job}` not in ci.yml")
    if cov == "✅" and not named and not any(k in evidence for k in ALLOWED_NON_TEST) and not JOB.search(evidence):
        failures.append(f"{n}: row marked ✅ but its Evidence names no test function")
    if cov == "✅" and evidence == "":
        failures.append(f"{n}: row marked ✅ with an empty Evidence cell")

if failures:
    print("\n".join(failures))
    print(f"{len(failures)} matrix evidence failure(s).")
    sys.exit(1)
print("every matrix evidence reference resolves; every ✅ names its proof.")
