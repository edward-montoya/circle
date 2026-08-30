# Plan: Phase 0 — Paper Validation

## Summary

Test Circle AI's central hypothesis **before writing any Rust**, by hand-authoring the three contracts, three skills, and one enforcement hook on two real repositories, then running a cold-start trial. If a hand-written `project.toml` plus three markdown files lets a stranger clone, run, test, and land a correct PR, the CLI is an optimization worth building. If it doesn't, no amount of Rust rescues it.

## User Story

As **the author of Circle AI**,
I want **the replication hypothesis falsified or confirmed on two real repos using only hand-written files**,
So that **I commit a month to the CLI only after the premise survives contact with a stranger**.

## Problem → Solution

**Current state:** The PRD's own Evidence section concedes *"no evidence yet that non-requester developers experience this as their top pain"* and *"targets are proposed, not validated"* — yet the original plan scheduled all validation for Phase 11, after eleven phases of work. The `≥70% of goal criteria machine-checkable` target has no evidence behind it at all.

**Desired state:** A one-week, zero-code experiment produces a go/no-go signal plus the first real measurements of three assumptions the whole plan rests on: the machine-checkable ratio, the time-to-first-preflight-pass, and whether hook enforcement is tolerable or infuriating.

## Metadata

- **Complexity**: Medium — many small files, no compiled code, but spread across three repositories
- **Source PRD**: `circle-ai.prd.md`
- **PRD Phase**: Phase 0 — Paper validation *(no dependencies; hard gate for Phases 1–7)*
- **Estimated Files**: 23 created, 2 modified
- **Design refs**: A-1 screens 01/02/03, A-3 flows F1 and F3

---

## ⚠ Read this before anything else

**The `circle` repository contains no code.** It holds seven markdown/HTML documents and nothing else — no `src/`, no tests, no build. There is no existing implementation to mirror, and there will not be one until Phase 1.

This inverts the usual "Patterns to Mirror" section. The patterns this plan must follow come from three places instead:

1. **Claude Code's own file formats** — `SKILL.md` frontmatter, `settings.json` hooks, exit-code semantics. These are hard external contracts, not preferences. Documented under *External Documentation*.
2. **The two target repositories** — their real test commands, compose services, and conventions. Captured verbatim below so no exploration is needed during implementation.
3. **The design artifacts** — A-1 (terminal output shapes), A-3 (which surface does what).

Every code snippet in this plan is either copied from a target repo or from the Claude Code documentation. None is invented.

---

## UX Design

### Before

```
┌──────────────────────────────────────────────────────────────┐
│ New dev clones bb-control                                    │
│                                                              │
│ $ claude                                                     │
│ > how do I run this?                                         │
│ ● Let me look around... I see a docker-compose.yml with      │
│   api and web. Try `docker compose up`?                      │
│ > that failed, it needs a data dir                           │
│ ● Let me check the README...                                 │
│ > how do I run the tests?                                    │
│ ● I see a tests/ folder and pytest config. Probably          │
│   `pytest`. Should I check pyproject.toml?                   │
│                                                              │
│ 4 turns spent re-deriving what the repo already knows.       │
│ Nothing is written down. Next session repeats it.            │
└──────────────────────────────────────────────────────────────┘
```

### After

```
┌──────────────────────────────────────────────────────────────┐
│ New dev clones bb-control                                    │
│                                                              │
│ $ ./scripts/circle-preflight.sh                              │
│   ✓ execution   2 services · 2 app · make dev-up             │
│   ✓ quality     2 gates · pytest, tsc                        │
│   ✗ knowledge   3 of 5 paths resolve                         │
│   PREFLIGHT FAILED · exit 2                                  │
│                                                              │
│ $ vim .circle/project.toml   # fix the two paths             │
│ $ ./scripts/circle-preflight.sh                              │
│   PREFLIGHT PASSED · exit 0                                  │
│                                                              │
│ $ claude                                                     │
│ > /circle-start                                              │
│ ● Contracts loaded. api:4000 web:4080. Gates: test:unit,     │
│   typecheck. Goal criteria: 4, 3 machine-checkable.          │
│   What are we building?                                      │
│                                                              │
│ 0 turns spent on setup. The repo told the agent.             │
└──────────────────────────────────────────────────────────────┘
```

### Interaction Changes

| Touchpoint | Before | After | Notes |
|---|---|---|---|
| Session opening | Free-form question | `/circle-start` loads contracts via injected `!` command | Skill aborts entirely if preflight fails |
| "How do I run it?" | Agent infers from README | Read from `[execution]` | Measured: turns-to-first-successful-run |
| "How do I test it?" | Agent guesses `pytest` | Read from `[quality]` | Measured: turns-to-first-green-gate |
| Writing a file | Unconditional | `PreToolUse` denies until preflight passes | **The finding that matters: is this helpful or infuriating?** |
| Next session | Re-derives everything | Reads the same committed contract | This is the replication claim |

---

## Mandatory Reading

| Priority | File | Lines | Why |
|---|---|---|---|
| **P0** | `circle-ai.prd.md` | "Phase 0" row + "Phase Details → Phase 0" | The success signal this plan must produce |
| **P0** | `circle-ai.prd.md` | "The Three Contracts" section | Exact TOML shape for `[knowledge]`, `[quality]`, `[execution]` |
| **P0** | `circle-ai.prd.md` | "The Task Contract" (D-35) | Per-task goal + verification, needed by `goal.md` |
| **P0** | `~/code/binarybridges/bb-control/docker-compose.yml` | all (34) | Target A execution contract — verbatim below |
| **P0** | `~/code/binarybridges/bb-control/pyproject.toml` | `[project.optional-dependencies]`, `[tool.pytest.ini_options]` | Target A quality contract |
| **P0** | `~/code/surebets/docker-compose.yml` | service keys | Target B — 18 services, the app/infra heuristic's real test |
| **P1** | `~/code/surebets/CLAUDE.md` | all (430) | **The competing hypothesis.** If this already solves cold start, Circle's premise is weaker than claimed |
| **P1** | `~/code/binarybridges/bb-control/frontend/package.json` | `"scripts"` | Second quality gate for target A |
| **P1** | `~/code/surebets/.claude/settings.local.json` | all | Must not be clobbered when wiring hooks |
| **P2** | A-1 screens 01–03 | — | Output shapes for preflight and the denied write |
| **P2** | A-3 flows F1, F3 | — | Cold start and gate-denial sequences |

## External Documentation

| Topic | Source | Key Takeaway |
|---|---|---|
| SKILL.md frontmatter | [code.claude.com/docs/en/skills](https://code.claude.com/docs/en/skills) | Only `description` is recommended; `allowed-tools`, `disable-model-invocation`, `hooks` are the levers. Directory name becomes `/command-name` |
| Injected shell context | same, *Inject dynamic context* | `` !`cmd` `` runs **before** content reaches Claude. **A non-zero exit aborts the entire skill invocation** — this is the preflight gate |
| Skill content lifecycle | same | Rendered content persists across turns; re-attached after compaction with a 5k-per-skill / 25k-total budget. Keep bodies small |
| PreToolUse hook | [code.claude.com/docs/en/hooks](https://code.claude.com/docs/en/hooks) | Exit 2 blocks, **or** exit 0 with `hookSpecificOutput.permissionDecision: "deny"`. Reason goes to stderr |
| Hook input | same, *Common input fields* | JSON on stdin carries `cwd`, `tool_name`, `tool_input`, `permission_mode` |
| Workspace trust | same, *Workspace Trust & Security* | **Hooks in `.claude/settings.json` do not run until the trust dialog is accepted.** Expected friction in Phase 0; the reason Phase 1 ships a plugin (D-21) |
| Matcher syntax | same | `Edit\|Write` matches either. Alphanumeric + `\|` is an exact list, anything else is regex |

```
KEY_INSIGHT: An injected `!` command that exits non-zero aborts the skill before Claude sees any of it.
APPLIES_TO: Tasks 5–7 — every skill opens with !`./scripts/circle-preflight.sh --quiet`
GOTCHA: Exit code 1 from grep-like commands is treated as a normal result, not a failure. Use exit 2 for "contract invalid" so it is never mistaken for "no matches".

KEY_INSIGHT: PreToolUse hooks in project settings.json require workspace trust; plugin hooks do not.
APPLIES_TO: Task 8, and the Phase 0 → Phase 1 finding
GOTCHA: A second developer cloning the repo gets NO enforcement until they accept the dialog. Record how visible that friction is — it is the empirical case for D-21.

KEY_INSIGHT: ${CLAUDE_PROJECT_DIR} stays at the main checkout; only the hook payload's `cwd` follows into a worktree.
APPLIES_TO: Task 8 — the hook script must read `cwd` from stdin
GOTCHA: Phase 0 does not use worktrees, so this will not bite yet. Write it correctly anyway so Phase 3 inherits a working script.
```

---

## Patterns to Mirror

> **No patterns exist in `circle` — it has no code.** What follows is captured from the two target repositories and from Claude Code's file formats. Snippets are verbatim.

### TARGET_A_EXECUTION
```yaml
# SOURCE: ~/code/binarybridges/bb-control/docker-compose.yml:9-38
services:
  api:
    build: .                      # build: present ⇒ app candidate (D-10)
    container_name: bbcontrol-api
    environment:
      BBCONTROL_DATA_DIR: /datos
    volumes:
      - ./datos:/datos
    ports:
      - "4000:8000"               # FIXED host port — collides across worktrees
  web:
    build: ./frontend             # app candidate
    depends_on: [api]
    ports:
      - "4080:80"                 # FIXED host port
```
Two services, both `build:`, zero infrastructure. The heuristic marks both app with no ambiguity. **Both host ports are hard-coded**, which is precisely the collision Phase 3 exists to solve — note it, do not fix it here.

### TARGET_A_QUALITY
```toml
# SOURCE: ~/code/binarybridges/bb-control/pyproject.toml
[project.optional-dependencies]
dev = ["pytest>=8.0", "httpx>=0.27"]

[tool.pytest.ini_options]
testpaths = ["tests"]
pythonpath = ["src"]
```
```json
// SOURCE: ~/code/binarybridges/bb-control/frontend/package.json
"scripts": { "dev": "vite", "build": "tsc -b && vite build", "preview": "vite preview" }
```
**Only two gates are derivable: `pytest` and `tsc -b`.** There is no linter, no formatter, and no coverage tool configured. Do not invent them — an incomplete quality contract that preflight reports honestly is a finding, not a defect.

### TARGET_B_EXECUTION
```yaml
# SOURCE: ~/code/surebets/docker-compose.yml
services:
  redis:
    image: redis/redis-stack:7.2.0-v19   # image: only ⇒ infrastructure
  bootstrap:
    build: ./services/bootstrap          # app
  scraper_betplay:
    build: ...                           # app
  # … 15 more, all build:
```
18 services, 1 infrastructure, 17 app candidates. This is the case where the human-confirmation step in D-10 earns its keep — and where a wrong contract is expensive.

### CLAUDE_MD_INCUMBENT
```
# SOURCE: ~/code/surebets/CLAUDE.md  (430 lines)
# SOURCE: ~/code/surebets/AGENTS.md  (64 lines)
```
Target B already carries 494 lines of agent instructions. **This is the competing hypothesis and must be measured, not ignored.** If a stranger succeeds on surebets using CLAUDE.md alone, Circle's differentiator is enforcement, not knowledge capture — which changes the README and the positioning.

### SKILL_FILE_SHAPE
```markdown
<!-- SOURCE: code.claude.com/docs/en/skills — frontmatter reference -->
---
description: Opens a Circle session. Use at the start of any implementation work.
disable-model-invocation: true
allowed-tools: Bash(docker compose *) Bash(pytest *) Read Grep
---

## Contracts
!`./scripts/circle-preflight.sh --context`

## Your task
...
```
`disable-model-invocation: true` keeps these user-triggered. `allowed-tools` grants only for the invoking turn.

### HOOK_WIRING
```json
// SOURCE: code.claude.com/docs/en/hooks — configuration
{
  "hooks": {
    "PreToolUse": [
      { "matcher": "Edit|Write",
        "hooks": [ { "type": "command",
                     "command": "${CLAUDE_PROJECT_DIR}/.claude/hooks/circle-gate.sh",
                     "timeout": 10 } ] }
    ]
  }
}
```

### HOOK_DENY_SHAPE
```bash
# SOURCE: code.claude.com/docs/en/hooks — "Block Destructive Commands"
#!/usr/bin/env bash
INPUT=$(cat)
CWD=$(printf '%s' "$INPUT" | jq -r '.cwd')     # NOT CLAUDE_PROJECT_DIR
if ! "$CWD/scripts/circle-preflight.sh" --quiet; then
  jq -n '{hookSpecificOutput:{hookEventName:"PreToolUse",
          permissionDecision:"deny",
          permissionDecisionReason:"circle: preflight failing — run ./scripts/circle-preflight.sh"}}'
fi
exit 0
```

### TEST_STRUCTURE
```python
# SOURCE: ~/code/binarybridges/bb-control/tests/  (16 files, conftest.py present)
# tests/test_conciliacion.py, test_lots.py, test_tax_co.py …
# Convention: test_<module>.py mirroring src/bbcontrol/<module>.py
```
Phase 0 writes **no Python tests**. This is captured so the `[quality]` contract's `test:unit` gate points at the right thing.

---

## Files to Change

### In `~/code/circle` (this repo)

| File | Action | Justification |
|---|---|---|
| `trials/PROTOCOL.md` | CREATE | The trial script — identical wording for every participant, so results compare |
| `trials/SCORESHEET.md` | CREATE | Per-participant measurements |
| `trials/cold-start.sh` | CREATE | Scripted proxy: fresh container + `claude -p`, asserts exit codes |
| `trials/FINDINGS.md` | CREATE | The Phase 0 deliverable — go/no-go plus the three measurements |
| `templates/project.toml` | CREATE | Annotated reference contract, extracted from the two hand-written ones |
| `templates/SKILL-contract.md` | CREATE | The skill conventions Phase 4 will codify |
| `circle-ai.prd.md` | UPDATE | Phase 0 → `in-progress`; record findings on completion |

### In `~/code/binarybridges/bb-control` (Target A — easy case)

| File | Action | Justification |
|---|---|---|
| `.circle/project.toml` | CREATE | The three contracts, hand-written |
| `.circle/goal.md` | CREATE | Success definition for the trial task |
| `scripts/circle-preflight.sh` | CREATE | Validator — the Rust CLI's stand-in |
| `.claude/skills/circle-start/SKILL.md` | CREATE | Session opener |
| `.claude/skills/circle-run/SKILL.md` | CREATE | Bring up the stack from `[execution]` |
| `.claude/skills/circle-verify/SKILL.md` | CREATE | Run `[quality]` gates + goal criteria |
| `.claude/hooks/circle-gate.sh` | CREATE | `PreToolUse` deny |
| `.claude/settings.json` | CREATE | Hook wiring |
| `.gitignore` | UPDATE | `+ .circle/runtime/` |

### In `~/code/surebets` (Target B — hard case)

Same nine files. **`.claude/settings.json` must be created without touching the existing `.claude/settings.local.json` or `.claude/memory.md`.**

## NOT Building

- **No Rust.** No `circle` binary, no crate, no `cargo` anything. The whole point is that this phase is falsifiable without it.
- **No brief, no approval gate, no diagrams.** That is Phase 5. The Phase 0 hook gates on *preflight*, not on approval.
- **No timeline, no scoring, no status.** Phases 6.
- **No plugin, no marketplace.** Phase 1. Phase 0 deliberately uses `.claude/skills/` and `settings.json` so the workspace-trust friction is *felt* and recorded.
- **No worktrees, no Compose namespacing.** Phase 3. Record the fixed-port collision; do not solve it.
- **No task graph, no `circle task` commands.** Phase 4. `goal.md` carries criteria; nothing enforces per-task verification yet.
- **No multi-repo, no `workspace.toml`.** Two repos are tested *independently*, not as a workspace.
- **No generalisation.** These files are hand-written for two specific repos. Resist every urge to write the generator — that urge is Phase 1 and it will eat the week.

---

## Step-by-Step Tasks

### Task 1: Scaffold the trial harness in `circle`
- **ACTION**: Create `trials/` and `templates/` in `~/code/circle`.
- **IMPLEMENT**: `trials/PROTOCOL.md` with the verbatim participant brief: *"Clone this repo. Land a PR that [task]. You may use Claude Code. You may not ask anyone how to run or test the project — if you get stuck, stop and say so."* Add a start/stop timestamp table and the four measurements below.
- **MIRROR**: `TARGET_A_QUALITY` — the trial task must be verifiable by an existing gate, so pick one covered by `pytest`.
- **GOTCHA**: Identical wording for every participant. A protocol improved between participants produces uncomparable results.
- **VALIDATE**: `test -f trials/PROTOCOL.md && grep -c "may not ask" trials/PROTOCOL.md`

### Task 2: Write `project.toml` for Target A (bb-control)
- **ACTION**: Create `~/code/binarybridges/bb-control/.circle/project.toml`.
- **IMPLEMENT**:
```toml
[project]
name = "bb-control"
contract_version = 1

[knowledge]
docs        = ["README.md", "docs/"]
definitions = []                                  # none exist — leave empty, do not invent
validations = ["tests/"]

[execution]
compose      = "docker-compose.yml"
app_services = ["api", "web"]
up           = "docker compose up --build -d"
down         = "docker compose down"
bootstrap    = "mkdir -p datos"
url          = "http://localhost:4080"

[quality]
typecheck = "cd frontend && npx tsc -b"
[quality.test]
unit = "pytest"
```
- **MIRROR**: `TARGET_A_EXECUTION` and `TARGET_A_QUALITY` exactly. Ports 4000/4080 are hard-coded upstream.
- **GOTCHA**: There is **no linter, formatter, or coverage tool** in this repo. Leaving `format`, `lint`, and `coverage` absent is correct. Preflight must report the gap; inventing `ruff` would fabricate a contract the repo cannot honour.
- **GOTCHA**: `definitions = []` is an honest empty, and it will drag the machine-checkable ratio down. That is a measurement, not a bug.
- **VALIDATE**: `python3 -c "import tomllib,pathlib;tomllib.loads(pathlib.Path('.circle/project.toml').read_text())"`

### Task 3: Write `project.toml` for Target B (surebets)
- **ACTION**: Create `~/code/surebets/.circle/project.toml`.
- **IMPLEMENT**: Same shape. `app_services` lists the 17 `build:` services; `redis` is excluded as infrastructure. `docs` registers `CLAUDE.md`, `AGENTS.md`, `docs/`, and the root `*.md` files.
- **MIRROR**: `TARGET_B_EXECUTION`.
- **GOTCHA**: **Time this task with a stopwatch.** Enumerating 17 services by hand is the single strongest argument for `circle init` auto-detection, and the number belongs in FINDINGS.md.
- **GOTCHA**: Do not let the contract restate what CLAUDE.md says. Overlap here is the finding — record it.
- **VALIDATE**: `python3 -c "import tomllib,pathlib;d=tomllib.loads(pathlib.Path('.circle/project.toml').read_text());assert len(d['execution']['app_services'])==17"`

### Task 4: Write `scripts/circle-preflight.sh` (both repos)
- **ACTION**: Create the validator. Bash + `python3 -c` for TOML; no third-party dependency.
- **IMPLEMENT**: Reads `.circle/project.toml`; for each `[knowledge]` path, glob and count matches; for each `[quality]` gate, check the binary resolves; verify `compose` exists and every `app_services` entry is a real service key. Flags: `--quiet` (exit code only), `--context` (compact block for skill injection), `--explain` (full report per A-1 screen 02).
- **MIRROR**: A-1 screen 02 for output shape — every failing line names its cause and the command that fixes it.
- **IMPORTS**: `bash`, `python3` ≥3.11 (`tomllib` is stdlib), `jq`.
- **GOTCHA**: **Exit 2 on contract failure, never 1.** Claude Code treats exit 1 from search-like commands as a normal result; exit 2 is unambiguously blocking for both the hook and the skill injection.
- **GOTCHA**: `--context` output is injected into every skill body and counts against the 5k-token budget. Keep it under 25 lines.
- **VALIDATE**: `./scripts/circle-preflight.sh --explain; echo "exit=$?"` — expect 2 with a fix list on the first run, 0 after paths are corrected.

### Task 5: Write `circle-start` skill (both repos)
- **ACTION**: Create `.claude/skills/circle-start/SKILL.md`.
- **IMPLEMENT**: Frontmatter `description`, `disable-model-invocation: true`, `allowed-tools: Read Grep Bash(docker compose *)`. Body opens with `` !`./scripts/circle-preflight.sh --context` `` then states the contracts, the gates, and asks what is being built.
- **MIRROR**: `SKILL_FILE_SHAPE`.
- **GOTCHA**: The injected command **is** the gate — if preflight exits 2 the skill never loads, and Claude sees nothing. Verify this by breaking a knowledge path and invoking `/circle-start`.
- **GOTCHA**: Body under ~150 lines. Skill content is re-attached after compaction under a 5k-token cap per skill.
- **VALIDATE**: In Claude Code, `/circle-start` with a broken path → `Shell command failed for pattern`. Repair → skill loads with contracts inline.

### Task 6: Write `circle-run` skill (both repos)
- **ACTION**: Create `.claude/skills/circle-run/SKILL.md`.
- **IMPLEMENT**: Injects `[execution]`, instructs Claude to run `bootstrap` then `up`, wait for the URL, and report the mapped ports. Ends with the copyable `down` command.
- **MIRROR**: `SKILL_FILE_SHAPE`; A-1 screen 01 for tone.
- **GOTCHA**: Per D-11 **Claude executes; the contract only records**. The skill must not wrap compose in a shell script — that becomes a second execution path.
- **VALIDATE**: In a clean clone: `/circle-run` → `http://localhost:4080` serves, in one turn, with no follow-up question.

### Task 7: Write `circle-verify` skill (both repos)
- **ACTION**: Create `.claude/skills/circle-verify/SKILL.md`.
- **IMPLEMENT**: Injects `[quality]` and `goal.md`; runs each gate, reports pass/fail per gate with exit codes, then evaluates goal criteria, separating machine-checked from human-judgement.
- **MIRROR**: A-1 screen 06 for the pass/fail block.
- **GOTCHA**: Report the ratio honestly. bb-control has two gates and no coverage — the skill must say *"2 gates, no coverage floor configured"*, not imply completeness.
- **VALIDATE**: `/circle-verify` on an unmodified clone → all gates green, ratio printed.

### Task 8: Wire the `PreToolUse` gate (both repos)
- **ACTION**: Create `.claude/hooks/circle-gate.sh` and `.claude/settings.json`.
- **IMPLEMENT**: Exactly `HOOK_DENY_SHAPE` and `HOOK_WIRING`. Script reads `cwd` from stdin JSON, runs preflight `--quiet`, emits the deny JSON on failure, exits 0 either way.
- **MIRROR**: `HOOK_DENY_SHAPE`, `HOOK_WIRING`.
- **IMPORTS**: `jq` — verify with `command -v jq` and fail loudly in the script if missing.
- **GOTCHA**: **On surebets, `.claude/settings.local.json` and `.claude/memory.md` already exist.** Create `settings.json` alongside; do not merge into or overwrite either.
- **GOTCHA**: Read `cwd` from stdin, **never** `${CLAUDE_PROJECT_DIR}` — they diverge inside a worktree, and Phase 3 inherits this script.
- **GOTCHA**: The hook will not fire until the workspace-trust dialog is accepted. **Record how many participants hit this and whether they understood it** — it is the evidence for D-21.
- **VALIDATE**: Break a knowledge path, ask Claude to edit any file → `Denied by hook` with the preflight reason. Repair → the same edit proceeds.

### Task 9: Author `goal.md` for the trial task (both repos)
- **ACTION**: Create `.circle/goal.md` with 4–5 criteria for the trial task, each marked verified-by-command or human-judgement.
- **IMPLEMENT**: Follow the D-35 shape — one sentence per criterion plus the command that proves it.
- **MIRROR**: A-1 screen 04, *Goal · what done means*.
- **GOTCHA**: **Count the ratio before choosing the task.** If a natural task yields under 70% machine-checkable on a repo with a real test suite, that is the first evidence the target is wrong — record it rather than picking an artificially testable task.
- **VALIDATE**: `grep -c '^- ' .circle/goal.md` and the machine-checkable count recorded in SCORESHEET.md.

### Task 10: Build the scripted cold-start proxy
- **ACTION**: Create `trials/cold-start.sh` in `circle`.
- **IMPLEMENT**: Fresh container or clean clone → `./scripts/circle-preflight.sh` (assert exit 0) → `claude -p "/circle-run"` → curl the URL (assert 200) → `claude -p "/circle-verify"` (assert gates green). Print a pass/fail line per step.
- **GOTCHA**: `-p` skips workspace trust, so **this proxy cannot detect the trust-dialog friction** that a human hits. It is necessary, not sufficient. State that in the script's header comment so no one mistakes a green run for a passed trial.
- **VALIDATE**: `bash trials/cold-start.sh bb-control` → all steps pass on a clean clone.

### Task 11: Run the trial — 3 participants × 2 repos
- **ACTION**: Execute `trials/PROTOCOL.md` with three developers new to each repo.
- **IMPLEMENT**: Record per participant: turns-to-first-successful-run, turns-to-first-green-gate, out-of-band questions asked (target: 0), whether a correct PR landed, whether the gate helped or obstructed, and — on surebets — **whether they used CLAUDE.md or the contract**.
- **GOTCHA**: Do not coach. The instant you explain something, that participant's run is spent.
- **GOTCHA**: Run bb-control first for all three. Ordering effects are real and consistent ordering at least makes them uniform.
- **VALIDATE**: SCORESHEET.md complete for 6 runs.

### Task 12: Write FINDINGS.md and decide
- **ACTION**: Create `trials/FINDINGS.md` in `circle` and update the PRD.
- **IMPLEMENT**: Go/no-go against the success signal (**2 of 3 land a correct PR**), plus the four measurements: machine-checkable ratio observed, time to hand-author each contract, trust-dialog friction count, and CLAUDE.md-vs-contract usage on surebets.
- **GOTCHA**: A failed trial is a successful Phase 0. The PRD says *"If this fails, stop."* Write the finding that is true, not the one that unblocks Phase 1.
- **VALIDATE**: FINDINGS.md states an explicit GO or NO-GO with the numbers behind it; PRD Phase 0 row updated.

---

## Testing Strategy

The trial **is** the test. There is no unit-testable code beyond the two shell scripts.

### Shell script tests

| Test | Input | Expected Output | Edge Case? |
|---|---|---|---|
| preflight, valid contract | complete `project.toml` | exit 0, "PASSED" | no |
| preflight, dead knowledge path | path pointing at nothing | exit 2, names the path | yes |
| preflight, missing compose | `compose` renamed | exit 2, names the file | yes |
| preflight, undeclared service | `app_services` has a typo | exit 2, lists real service keys | yes |
| preflight, missing gate binary | `pytest` not installed | exit 2, names the binary | yes |
| preflight `--context` size | valid contract | ≤ 25 lines | yes |
| gate hook, preflight failing | any Edit/Write | deny JSON on stdout, exit 0 | no |
| gate hook, preflight passing | any Edit/Write | empty stdout, exit 0 | no |
| gate hook, `jq` absent | — | loud failure, not silent allow | yes |
| gate hook, `cwd` ≠ project dir | payload with different `cwd` | validates the `cwd` repo | yes |

### Edge Cases Checklist
- [ ] Empty `[knowledge]` section — must pass with a warning, not crash
- [ ] `project.toml` malformed — clear parse error, exit 2, never a stack trace
- [ ] `.circle/` absent entirely — exit 3, "run circle init" (which does not exist yet: say so)
- [ ] Docker not running — `circle-run` fails legibly, not with a raw daemon error
- [ ] Ports 4000/4080 already occupied — **expected on the second concurrent trial.** Record it; do not fix it
- [ ] Participant declines the trust dialog — hook silently absent. **Does anyone notice?**
- [ ] surebets: 17 app services, `docker compose up` slow or OOM — cap the trial task to a subset if so

---

## Validation Commands

### TOML validity
```bash
cd ~/code/binarybridges/bb-control && python3 -c "import tomllib,pathlib;tomllib.loads(pathlib.Path('.circle/project.toml').read_text());print('ok')"
cd ~/code/surebets           && python3 -c "import tomllib,pathlib;tomllib.loads(pathlib.Path('.circle/project.toml').read_text());print('ok')"
```
EXPECT: `ok` for both

### Shell correctness
```bash
shellcheck scripts/circle-preflight.sh .claude/hooks/circle-gate.sh
bash -n scripts/circle-preflight.sh
```
EXPECT: no errors (install `shellcheck` if absent; it is the only new tool this phase needs)

### Preflight, both states
```bash
./scripts/circle-preflight.sh --explain; echo "exit=$?"
```
EXPECT: exit 2 with a fix list before repair, exit 0 after

### Hook contract
```bash
echo '{"cwd":"'"$PWD"'","tool_name":"Write","tool_input":{"file_path":"x.py"}}' \
  | .claude/hooks/circle-gate.sh | jq -e '.hookSpecificOutput.permissionDecision'
```
EXPECT: `"deny"` while preflight fails; empty output once it passes

### Skill abort
```bash
# with a knowledge path broken, in Claude Code:
/circle-start
```
EXPECT: `Shell command failed for pattern "./scripts/circle-preflight.sh --context"` and **no skill content** in the transcript

### Scripted cold start
```bash
bash trials/cold-start.sh bb-control
bash trials/cold-start.sh surebets
```
EXPECT: every step passes on a clean clone

### Manual Validation
- [ ] Clean clone of bb-control: `docker compose up` reachable at :4080 following only `[execution]`
- [ ] `pytest` green from `[quality]` alone
- [ ] `/circle-start` prints the contracts without reading the README
- [ ] Write denied while preflight fails; allowed after repair
- [ ] Repeat all of the above on surebets
- [ ] Both `.circle/` directories commit cleanly with no absolute paths and no secrets

---

## Acceptance Criteria

- [ ] `.circle/project.toml` + `goal.md` hand-written and committed in **both** repos
- [ ] Three skills and one gate hook working in **both** repos
- [ ] `circle-preflight.sh` exits 2 on every failure mode in the test table, 0 when valid
- [ ] The `PreToolUse` gate demonstrably denies a write and demonstrably stops denying after repair
- [ ] `trials/cold-start.sh` passes unattended on both repos
- [ ] **6 trial runs completed** — 3 participants × 2 repos — with SCORESHEET.md filled in
- [ ] `trials/FINDINGS.md` records an explicit **GO** or **NO-GO** with the four measurements
- [ ] `templates/project.toml` and `templates/SKILL-contract.md` extracted for Phase 1
- [ ] PRD Phase 0 row updated with the outcome

**The phase succeeds when the question is answered, not when the answer is yes.** A NO-GO that stops a month of Rust is the highest-value outcome available here.

## Completion Checklist
- [ ] Contracts describe only what the repos actually have — no invented linters, no aspirational gates
- [ ] No Rust, no plugin, no generator written
- [ ] Hook reads `cwd` from stdin, not `${CLAUDE_PROJECT_DIR}`
- [ ] Preflight exits 2, never 1, on contract failure
- [ ] Skill bodies under the compaction budget
- [ ] surebets' existing `.claude/` files untouched
- [ ] No absolute paths or secrets in anything committed
- [ ] FINDINGS.md written before any Phase 1 work begins

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| **Cannot recruit 3 developers** | **H** | Phase 0 stalls; the temptation is to skip to Phase 1 | Scripted proxy proves the mechanics; the human trial proves the thesis. **Do not treat a green script as a passed trial.** If recruitment fails, say so in FINDINGS.md and mark the hypothesis untested rather than assumed |
| **CLAUDE.md alone succeeds on surebets** | **M** | Undercuts the knowledge-capture half of the premise | This is a finding, not a failure. It would narrow Circle's claim to *enforcement + execution*, which is defensible — and the README must then change |
| Author writes contracts for repos he knows well | **H** | Contracts encode tacit knowledge; strangers still fail | Write both contracts using **only** what is in the repo. If a value cannot be sourced from a file, leave it out and let preflight report the gap |
| Machine-checkable ratio lands far under 70% | **M** | A headline PRD target is wrong | Expected and worth knowing now. Record the real number; D-37 already anticipates lowering the target rather than gaming it |
| Fixed ports block concurrent trials | **M** | Two participants cannot run bb-control at once | Stagger runs, or `COMPOSE_PROJECT_NAME` + a port override by hand. **Do not build Phase 3's allocator** |
| Trust dialog silently disables the gate | **M** | Enforcement appears to work but does not | Explicit checklist item; count occurrences. It is the empirical case for shipping a plugin in Phase 1 |
| Phase 0 grows into Phase 1 | **H** | The week is lost writing a generator | Every task above says hand-write. `templates/` is extraction *after* the fact, never a code path |

## Notes

- **Why these two repos.** bb-control is the easy case: two services, both `build:`, a real pytest suite, no `.claude/` skills. surebets is the hard case: 18 services with mixed infra and app, 494 lines of existing agent instructions, and no root test directory. Passing one and failing the other is a more useful result than passing both.
- **Why three skills, not five.** Cold start needs run, test, and a session opener. The brief and handoff are about comprehension and continuity, not first-PR success, and Phase 5 owns the brief properly.
- **Why the hook gates on preflight, not approval.** There is no brief in Phase 0, but the enforcement mechanism — a `PreToolUse` deny backed by a non-zero exit — is identical. This tests the single largest correction from the review at zero extra cost.
- **bb-control has no linter or formatter.** Its `[quality]` block will have two gates and no coverage floor. That is a faithful contract for that repo, and a preflight that reports the gap plainly is the behaviour the PRD promises.
- **Prerequisite, unresolved:** three participants. Flagged as the top risk rather than blocking this plan, because Tasks 1–10 can all be completed before anyone is recruited.
