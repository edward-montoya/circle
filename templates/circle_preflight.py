#!/usr/bin/env python3
"""circle-preflight — Phase 0 hand-written stand-in for `circle preflight`.

Validates the three contracts in .circle/project.toml and exits with a code that
both Claude Code hooks and skill-body injection understand.

    exit 0  contracts valid
    exit 2  contract invalid -> PreToolUse denies the write, skill invocation aborts
    exit 3  precondition missing (no .circle/, no python3)

EXIT 2, NEVER 1. Claude Code treats exit 1 from search-like commands as a normal
result. Exit 2 is unambiguously blocking on both surfaces.

Modes:
    --explain   full report with fixes        (humans)
    --context   compact block, <=25 lines     (skill injection)
    --quiet     exit code only                (hooks)

Why Python and not bash: macOS ships bash 3.2, which mis-parses heredocs inside
command substitution when they contain backticks or apostrophes. A stranger's
machine is the target, so the logic lives here and the .sh is a one-line shim.
This was found by running it, not by reasoning about it.

Deliberately NOT the Go binary. This is the falsifiable paper version.
"""

from __future__ import annotations

import glob
import os
import re
import shutil
import sys
import tomllib
from pathlib import Path

OK, WARN, FAIL = "ok", "warn", "fail"


class Report:
    def __init__(self) -> None:
        self.rows: list[tuple[str, str, str, str, str]] = []

    def add(self, status: str, section: str, label: str, detail: str = "", fix: str = "") -> None:
        self.rows.append((status, section, label, detail, fix))

    def count(self, status: str) -> int:
        return sum(1 for r in self.rows if r[0] == status)


def compose_services(path: Path) -> list[str]:
    """Service keys under `services:`. Two-space indent only.

    Deliberately not pyyaml: a stranger must be able to run this with nothing
    installed but python3. Good enough for Phase 0; the Go binary parses properly.
    """
    names, inside = [], False
    for line in path.read_text().splitlines():
        if re.match(r"^services:\s*$", line):
            inside = True
            continue
        if inside:
            if re.match(r"^\S", line):
                break
            m = re.match(r"^  ([A-Za-z0-9._-]+):\s*$", line)
            if m:
                names.append(m.group(1))
    return names


def check_execution(c: dict, root: Path, r: Report) -> None:
    ex = c.get("execution", {})
    services: list[str] = []

    rel = ex.get("compose")
    if not rel:
        r.add(FAIL, "execution", "compose", "no compose file declared", "add execution.compose")
    elif not (root / rel).exists():
        r.add(FAIL, "execution", "compose", f"{rel} does not exist",
              "point execution.compose at a real file")
    else:
        services = compose_services(root / rel)
        r.add(OK, "execution", "compose", f"{rel} · {len(services)} services")

    declared = ex.get("app_services", [])
    unknown = [s for s in declared if services and s not in services]
    if not declared:
        r.add(FAIL, "execution", "app_services", "none declared",
              "list the containers that hold your code")
    elif unknown:
        r.add(FAIL, "execution", "app_services", "not in compose: " + ", ".join(unknown),
              "real services: " + ", ".join(sorted(services)))
    else:
        shown = ", ".join(declared[:4]) + (f" +{len(declared) - 4}" if len(declared) > 4 else "")
        r.add(OK, "execution", "app_services", shown)

    for key in ("up", "down"):
        if ex.get(key):
            r.add(OK, "execution", key, ex[key])
        else:
            r.add(FAIL, "execution", key, "not declared", f"add execution.{key}")


def check_quality(c: dict, r: Report) -> None:
    q = c.get("quality", {})
    # `bootstrap` is how you make the gates runnable, not a gate itself.
    gates = {k: v for k, v in q.items() if isinstance(v, str) and k != "bootstrap"}
    gates.update({f"test:{k}": v for k, v in q.get("test", {}).items()})

    if not gates:
        r.add(FAIL, "quality", "gates", "no gates declared", "add at least one runnable gate")

    # A gate whose binary is absent is not runnable *yet*. That is different from
    # a gate that was never declared, and different again from one that fails.
    # Phase 0 finding: [quality] needs a bootstrap step exactly as [execution]
    # does — on a fresh clone `pytest` lives behind `pip install -e .[dev]`, so
    # without it preflight would report a green contract a stranger cannot run.
    bootstrap = q.get("bootstrap")
    missing = []
    for name, cmd in sorted(gates.items()):
        # Strip leading `cd <dir> &&` and env assignments to find the binary.
        probe = re.sub(r"^(cd\s+\S+\s*&&\s*)+", "", cmd).split()
        binary = next((t for t in probe if "=" not in t), "")
        if binary and shutil.which(binary):
            r.add(OK, "quality", name, cmd)
        else:
            missing.append(binary or name)
            r.add(WARN, "quality", name, f"{binary} not runnable yet",
                  f"run: {bootstrap}" if bootstrap else
                  "declare quality.bootstrap so a fresh clone can install it")

    if missing and not bootstrap:
        # No way to make the gates runnable => the contract cannot be honoured
        # on a fresh clone, which is the entire promise. This blocks.
        r.add(FAIL, "quality", "bootstrap", f"{len(missing)} gate(s) unrunnable, no bootstrap",
              "add quality.bootstrap with the install command")

    cov = q.get("coverage", {}).get("min")
    r.add(OK if cov else WARN, "quality", "coverage",
          f"floor {cov}%" if cov else "no floor configured",
          "" if cov else "the status score cannot use what is not measured")


def check_knowledge(c: dict, root: Path, r: Report) -> tuple[int, int]:
    kn = c.get("knowledge", {})
    total = hit = 0
    for cls in ("docs", "definitions", "validations"):
        paths = kn.get(cls, [])
        if not paths:
            r.add(WARN, "knowledge", cls, "empty — nothing registered",
                  f"circle knowledge add <path> --as {cls}")
            continue
        for p in paths:
            total += 1
            matches = glob.glob(str(root / p), recursive=True)
            if not matches and (root / p).exists():
                matches = [str(root / p)]
            if matches:
                hit += 1
                r.add(OK, "knowledge", cls, f"{p} → {len(matches)}")
            else:
                r.add(FAIL, "knowledge", cls, f"{p} → 0 matches", "repoint it or remove it")
    if total:
        r.add(OK if hit == total else FAIL, "knowledge", "paths", f"{hit} of {total} resolve")
    return hit, total


def check_goal(root: Path, r: Report) -> None:
    """Advisory only. Never blocks."""
    goal = root / ".circle" / "goal.md"
    if not goal.exists():
        r.add(WARN, "goal", "goal.md", "absent", "advisory — does not block")
        return
    # A criterion is a bullet plus every continuation line under it. Detecting on
    # first lines alone under-counts badly: a criterion whose command sits on the
    # wrapped second line reads as unverifiable, which is the opposite of true.
    crit: list[str] = []
    for line in goal.read_text().splitlines():
        stripped = line.strip()
        if stripped.startswith("- "):
            crit.append(stripped)
        elif crit and stripped and line.startswith((" ", "\t")):
            crit[-1] += " " + stripped
    checkable = [b for b in crit if chr(96) in b]  # carries a command in backticks
    pct = round(100 * len(checkable) / len(crit)) if crit else 0
    r.add(WARN if pct < 70 else OK, "goal", "criteria",
          f"{len(checkable)} of {len(crit)} machine-checkable ({pct}%, target 70%)",
          "" if pct >= 70 else "attach a command to more criteria")


def build(root: Path) -> Report:
    r = Report()
    toml_path = root / ".circle" / "project.toml"
    try:
        c = tomllib.loads(toml_path.read_text())
    except Exception as e:
        r.add(FAIL, "contract", "project.toml", f"{type(e).__name__}: {e}", "fix the TOML syntax")
        return r
    check_execution(c, root, r)
    check_quality(c, r)
    check_knowledge(c, root, r)
    check_goal(root, r)
    return r


# --------------------------------------------------------------------------- render

def _palette(stream) -> dict[str, str]:
    if not stream.isatty() or os.environ.get("NO_COLOR"):
        return dict.fromkeys(("g", "r", "y", "d", "b", "n"), "")
    return {"g": "\033[32m", "r": "\033[31m", "y": "\033[33m",
            "d": "\033[2m", "b": "\033[1m", "n": "\033[0m"}


def _mark(status: str, p: dict[str, str]) -> str:
    return {OK: f"{p['g']}✓{p['n']}", WARN: f"{p['y']}⚠{p['n']}"}.get(status, f"{p['r']}✗{p['n']}")


def render_explain(r: Report, root: Path, p: dict[str, str]) -> None:
    print(f"\n{p['b']}PREFLIGHT{p['n']}  {root}\n")
    section = None
    for status, sec, label, detail, fix in r.rows:
        if sec != section:
            print(f"{p['b']}{sec.upper()}{p['n']}")
            section = sec
        print(f"  {_mark(status, p)} {label:<13} {detail}")
        if fix:
            print(f"     {p['d']}└ {fix}{p['n']}")
    fails, warns = r.count(FAIL), r.count(WARN)
    print()
    if not fails:
        print(f"{p['g']}PREFLIGHT PASSED{p['n']}  {p['d']}{warns} advisory{p['n']}\n")
        return
    print(f"{p['r']}PREFLIGHT FAILED{p['n']}  {p['d']}{fails} blocking · {warns} advisory{p['n']}")
    print(f"{p['d']}Writes are blocked until these pass.{p['n']}\n")


def render_context(r: Report, root: Path, p: dict[str, str]) -> None:
    """Injected into every skill body. Must stay small — see the 5k/25k budget."""
    print(f"CONTRACTS  {root.name}")
    for status, sec, label, detail, _ in r.rows:
        if status == OK and sec == "knowledge" and label != "paths":
            continue  # collapse the per-path noise; keep the ratio line
        print(f"  {_mark(status, p)} {sec:<9} {label:<12} {detail}")
    if r.count(FAIL):
        print("\nPREFLIGHT FAILED — run ./scripts/circle-preflight.sh --explain")


def main(argv: list[str]) -> int:
    mode = "explain"
    if len(argv) > 1:
        flag = argv[1]
        if flag not in ("--explain", "--context", "--quiet"):
            print("usage: circle-preflight.sh [--explain|--context|--quiet]", file=sys.stderr)
            return 3
        mode = flag[2:]

    root = Path(__file__).resolve().parent.parent
    if not (root / ".circle" / "project.toml").exists():
        if mode != "quiet":
            print(f"circle: no .circle/project.toml at {root}", file=sys.stderr)
        return 3

    r = build(root)
    failed = r.count(FAIL) > 0

    if mode == "quiet":
        return 2 if failed else 0

    p = _palette(sys.stdout)
    (render_context if mode == "context" else render_explain)(r, root, p)
    return 2 if failed else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
