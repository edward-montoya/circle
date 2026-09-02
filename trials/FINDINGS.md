# Phase 0 — Findings

*Running log. Started 2026-08-29. Iteration 1 of the implementation loop.*

Status: **Tasks 2–9 done on both targets. Human trial (Tasks 11–12) not started.**
No GO/NO-GO yet — that verdict needs the trial.

Both targets live on branch `circle/phase-0`, built in git worktrees so neither
repo's working tree was disturbed. bb-control had 10 uncommitted files on
`feat/retiros`; they were never touched.

| Target | Preflight | Gate deny/allow | Committed |
|---|---|---|---|
| bb-control | exit 0 · 3 advisory | ✓ both states | `fe84831` |
| surebets | exit 0 · 4 advisory | ✓ both states | `f7dec22` |

---

## F-1 · `.claude/` is gitignored in surebets — the skills cannot travel

**The finding that matters most so far.** `surebets/.gitignore:190` ignores
`.claude/`, so the three skills and the enforcement hook **cannot be committed
to that repo**. A stranger cloning it gets `.circle/project.toml` and the
validator, and no skills or gate at all.

This is not unusual — ignoring `.claude/` is common practice. It means the
"share the workflow via the repo" premise fails on any repo that does it, unless
the skills and hooks live somewhere else.

**They already can:** a Claude Code plugin's skills and hooks do not live in the
repo's `.claude/` directory. This is direct empirical support for **D-21**, which
was argued from documentation alone. The contract stays in the repo where it
belongs; the machinery ships with the plugin.

**Action:** raise D-21 from a distribution convenience to a correctness
requirement. Note it in the PRD.

## F-2 · `[quality]` needs a `bootstrap` step — schema gap

Preflight initially **passed** on bb-control while `pytest` was not installed.
The contract was green and a stranger could not have run the tests, which makes
the pass a lie.

The cause is a missing concept: `[execution]` has a `bootstrap` and `[quality]`
does not, yet on a fresh clone `pytest` sits behind `pip install -e ".[dev]"`.

**Fixed** by adding `quality.bootstrap`. Preflight now distinguishes three
states: gate not declared (**fail**), gate declared but not runnable yet with a
bootstrap available (**warn**, quoting the command), and gate unrunnable with no
bootstrap at all (**fail** — the contract cannot be honoured on a fresh clone).

**Action:** add `quality.bootstrap` to the PRD's contract schema.

## F-3 · macOS bash 3.2 forced the logic out of the shell

The validator was first written as bash with a Python heredoc inside `$( )`.
It would not parse: macOS ships bash 3.2, which mis-handles heredocs inside
command substitution when they contain a backtick or an apostrophe.

Two failures, then the restructure: all logic moved to `circle_preflight.py`,
and `circle-preflight.sh` became a one-line `exec` shim.

Worth keeping in mind for the Go binary: **the target is a stranger's machine,
not the author's.** Found by running it, not by reasoning about it.

## F-4 · Enumerating 18 services by hand is not viable

surebets has 19 compose services. Listing the 18 application containers was done
with a script, because typing them correctly by hand is error-prone busywork.

That is the argument for `circle init` auto-detection stated as a measurement
rather than an assumption. The `build:`-present heuristic (**D-10**) classified
all 19 correctly on the first pass: 1 infrastructure, 18 app.

## F-5 · Preflight caught the author's own wrong assumption

The surebets contract registered `README.md` under `[knowledge] docs`. **There is
no README.md at the root of that repo.** Preflight rejected the contract.

Small, but it is the mechanism working on the person best placed to get it
right. A contract asserting a file that does not exist is caught before a
session starts, not halfway through one.

## F-6 · The goal ratio can be inflated by lint and format gates

surebets reaches 80% machine-checkable partly because `ruff check` and
`ruff format --check` supply two criteria for free — neither of which says
anything about whether the feature works.

The 70% target is gameable as specified.

**Open question for the PRD:** should lint and format criteria count toward the
goal ratio, or only test criteria? Current opinion: only tests, with lint and
format living in `[quality]` where they already are and staying out of `goal.md`.

## F-7 · Two self-inflicted bugs, both caught only by running the thing

Recorded because they bear on how the Go port should be tested, not as
self-flagellation.

1. **The gate hook had an inverted test** — `[[ -x $PREFLIGHT ]] && exit 0`
   allowed every write instead of proceeding to validate. A gate that silently
   stops gating is worse than no gate: it manufactures confidence exactly when
   someone is trusting it. **Every gate must be tested in both states, always.**
2. **The goal parser under-counted 4 of 5 as 1 of 5**, because criteria wrap
   onto a second line and it only inspected first lines. A criterion whose
   command sits on the wrapped line read as unverifiable — the opposite of true.

## F-8 · Contrast between the two targets is holding

| | bb-control | surebets |
|---|---|---|
| Services | 2, both app | 19, 1 infra + 18 app |
| Linter / formatter | **none** | ruff, both |
| Coverage | none | measured, no floor declared |
| `definitions` | **empty — nothing to read** | docs/ + 2 templates |
| Existing agent instructions | none | **494 lines of CLAUDE.md + AGENTS.md** |
| `.claude/` committable | yes | **no — gitignored** |

The pairing is doing its job: every finding above came from one repo and not the
other. bb-control's empty `definitions` and missing tooling stress the contract's
honesty; surebets stresses scale, distribution, and the competing hypothesis.

---

## Still open

- **The trial itself.** Tasks 11–12 need three people. Nothing above tests
  whether a *human stranger* succeeds — only that the mechanism works.
- Whether surebets' 494 lines of CLAUDE.md already solve cold start, which is
  the single most important measurement left.

---

# Iteration 2 — the Go core

*Same day. Phases 1, 2, 4 and 6 built; Phase 3 and 5 not started.*

The hand-written Phase 0 validator is now superseded by a real binary. Both
targets were re-verified against it, and both behave identically to the paper
version — which is the cheapest possible confirmation that the paper version
encoded the right rules.

| Built | Verified against |
|---|---|
| `circle init\|doctor\|preflight\|project validate` | both targets |
| `circle gate check --hook` (PreToolUse deny/allow) | both targets |
| `circle quality run\|list` | bb-control |
| `circle task create\|validate\|ready\|claim\|close\|list\|show` | bb-control |
| `circle timeline open\|sync\|show\|drift` | bb-control, incl. an amend |
| `circle status`, `circle score explain` | bb-control |
| `circle serve` — SSE app | bb-control, live |
| `circle` plugin | `claude plugin validate` ✔ |

## F-9 · The task contract holds under test

The full loop was exercised end to end on bb-control:

1. `task create` with no goal → **exit 2**, task not created.
2. `task create` naming an undeclared gate → **exit 2**, and the error lists the
   gates that *are* declared.
3. `verify-kind manual` without justification or reviewer → **exit 2**.
4. `task close` before any gate run → **exit 5**, naming the gate and the
   command to run.
5. `quality run test:unit` → event recorded.
6. `task close` → succeeds, and records `evidence: test:unit@2026-08-30T04:29:19Z`.

Step 4 is the one that matters. Task closure is now a measurement rather than an
assertion, which was the last of the four status components resting on the
agent's word.

## F-10 · Auto-detection settles F-4

F-4 recorded that enumerating surebets' 18 application containers by hand was
error-prone busywork, and that this was the argument for `circle init`.

`circle init --detect-only` now classifies all 19 services correctly in one
command — 18 `build:` app candidates, `redis` as infrastructure — matching the
hand-built list exactly. The D-10 heuristic did not misclassify anything on
either target.

## F-11 · A bug a unit test could not have caught

`git()` applied `TrimSpace` to the whole of `git status --porcelain`. That output
is column-oriented: an unstaged modification is ` M path`, with a leading space.
Trimming shifted the line left by one, so **every unstaged path lost its first
character** — `.circle/x` rendered as `circle/x`, which reads as a plausible path
rather than an error.

The parser was correct and its tests passed. The corruption happened upstream of
the parser, in the helper that fetched the bytes. Fixed with a `gitRaw` variant,
and the table-driven test now uses dotfiles precisely because the truncation is
invisible on an ordinary path.

Worth generalising: **the boundary where output is normalised is as much a
parsing surface as the parser.**

## F-12 · What is not built

Honest ledger, so the phase table is not read as more than it is.

- **Phase 3 — Compose isolation.** No `WorktreeCreate`/`WorktreeRemove` hooks
  yet. bb-control's fixed ports (4000, 4080) still collide across checkouts.
- **Phase 5 — Brief and the human approval gate.** The `PreToolUse` gate
  currently enforces *preflight*, not approval. The blast-radius check is
  written and wired but inert until an `approved-radius` file exists.
- **Phase 7 — the trial.** Still the largest gap. Everything above proves the
  mechanism works; nothing proves a human stranger succeeds.
- The `Stop` hook is specified in the CLI doc but not yet registered.

---

# Iteration 3 — Phase 3, Compose isolation

## F-13 · The override file was appending, not replacing

The first working version generated a `docker-compose.override.yml` remapping
each published port, and it looked correct. `docker compose config` disagreed:

```
published: "8000"     <- the original, still there
published: "42000"    <- the remap
```

**Compose's default merge strategy for a sequence is APPEND.** The original host
port stayed published alongside the new one, so two worktrees would still have
collided on 8000 — while every Circle command reported the stack as isolated.

Fixed with the `!override` tag (Compose v2.24+):

```yaml
services:
  api:
    ports: !override
      - "42000:8000"
```

This is the third bug in the same family — after the bash heredoc and the
`TrimSpace` on porcelain output. All three produced **plausible-looking wrong
output rather than an error**, and all three were found by running the thing
against a real repository rather than by reasoning about it. The pattern is
worth naming: *a wrong result that resembles a right one is the expensive kind,
and the only reliable detector is executing against the real substrate.*

## F-14 · The port ledger cannot live in `.circle/runtime/`

Each worktree has its own `.circle/`, so a per-worktree ledger cannot see the
other worktrees' allocations — which is precisely the collision it exists to
prevent.

The ledger now lives beside `git rev-parse --git-common-dir`, the one path every
worktree of a repository agrees on. Allocation holds a real `flock`, because
three agents provisioning at once is the designed case and a check-then-write
lets two of them pass the free-port probe simultaneously.

Both checks are needed and neither alone suffices: the ledger knows what Circle
handed out, and `net.Listen` knows what anything else on the machine is holding.

## F-15 · Phase 3 exit criterion met

Three worktrees of bb-control, provisioned **in parallel**:

| Worktree | Project | api | web |
|---|---|---|---|
| task-118 | `circle-task-118` | 42000 | 42001 |
| task-119 | `circle-task-119` | 42002 | 42003 |
| task-120 | `circle-task-120` | 42004 | 42005 |

Six distinct ports, one shared ledger, zero collisions. `docker compose config`
confirms each worktree publishes exactly two ports and carries its own project
name, so networks and volumes namespace themselves. `release` runs
`compose down -v` and frees the range; a killed session's lease is pruned on the
next provision, so the range self-heals rather than leaking.

Unparseable port entries — ranges, udp — are reported loudly rather than
skipped. A stack advertised as isolated with one port still colliding is worse
than an honest refusal.

## Still not built

- **Phase 5** — the brief and the human approval gate. The `PreToolUse` gate
  still enforces preflight, not approval.
- **Phase 7** — the trial. Unchanged and still the largest gap.

---

# Iteration 4 — Phase 5, the human gate

The headline claim is now true. Until this phase the `PreToolUse` hook enforced
*preflight*; it now enforces *approval*, checked on every single write rather
than trusted once at session start.

Verified end to end on bb-control:

| Step | Result |
|---|---|
| Write with no brief | **deny** — "no approved brief" |
| Write after generating, before approving | **deny** — generating is not approving |
| Self-approve without `--allow-self` | **refused**, and the error names the flag |
| Self-approve with `--allow-self` | approved, recorded as `self_approved` |
| Write inside the radius | allow |
| Write to `infra/terraform/main.tf` | **deny**, and the reason lists the approved patterns |
| Add a task, then write again | **deny** — "the plan changed since approval" |

## F-16 · The blast radius belongs to the task, not the brief

A-3 left this open. It is now decided: `--paths` is a field on the task.

A four-task item cannot otherwise express that task 1 touches `src/mw/` and
task 4 touches `Makefile`. With a single radius per brief the write-block would
have to approve the union of everything, which is looser than anything a human
actually agreed to — and drift could not be attributed to the task that caused
it.

## F-17 · Approval is bound to a plan hash, and the hash had to be chosen carefully

The hash covers every task's id, title, goal, verification kind, gate, command,
paths and dependencies. It deliberately **excludes** state, claim and closure.

That distinction is the whole design. If the hash covered task state, then
*implementing* an approved plan would revoke its own approval on the first
closure — the gate would fight the work it just authorised. If it excluded
paths, a task could quietly widen its own write permissions after approval.

A table test pins both halves: seven mutations that must invalidate, and two
that must not.

## F-18 · Two gates that had to stay open

Enforcement that blocks its own prerequisites is unusable, so:

- **`.circle/` is always writable.** Recording an event or closing a task must
  never be denied by the gate those actions serve.
- **A repo with no tasks is not gated.** Otherwise adopting Circle would block
  the very edits needed to configure it — the first thing a new user does is
  the thing they could not do.

Both are narrow carve-outs with a stated reason, not general escape hatches.

## Still not built

- **Phase 7** — the trial. Unchanged, and now the only thing between here and a
  v0.1 verdict. Every mechanism the PRD promised is built and demonstrated;
  none of it proves a human stranger succeeds.

---

# Iteration 5 — greenfield

Prompted by a question rather than a plan: what does a project with no compose
file and no tests actually experience? The answer was bad enough to be worth
recording.

## F-19 · Day zero was a dead end

Running the real sequence on an empty repository with only a `REQUIREMENTS.md`:

| Step | Before |
|---|---|
| `circle init` | **exit 0**, a green tick, and `✓ quality 0 gate(s) detected` |
| Contract written | `compose = ""` and `up = "docker compose up --build -d"` |
| `circle preflight` | exit 2, three blocking failures |
| Writing `main.go` | **denied** |
| `REQUIREMENTS.md` | ignored entirely |

A new user installs Circle, sees a checkmark, and then cannot write a single
file. Their only escape is `--force` from the first minute, which teaches them
the gate is noise before it has ever been useful.

Four defects, none of which needed the scope question answering:

1. **`init` invented tooling.** It wrote `docker compose up` into a repository
   with no compose file — the exact thing the contract's own header promises not
   to do.
2. **A tick on zero.** `✓ quality 0 gate(s) detected` reported the total absence
   of gates as a success.
3. **The fix hints assumed brownfield.** "list the containers that hold your
   code" means nothing when there is no code.
4. **The one artifact present was ignored.** `REQUIREMENTS.md` is precisely what
   the definitions registry exists for.

## F-20 · Incubating: a third state between valid and broken

A new project and a broken contract look identical to a naive check and deserve
opposite treatment. **Incubating** is now explicit: nothing declared, and nothing
on disk to declare.

In that state the gaps are still listed, but preflight exits 0 and the gate stays
open. The state is deliberately narrow, and three things end it:

- a compose file appears on disk → declare it, or block with the exact line to paste
- a gate is declared → no longer incubating
- **the contract names a file that does not exist → never incubating.** A
  contract claiming an execution model it cannot honour is a lie, not a young
  project, and laundering that through incubation would make the gate decorative

One thing still blocks once a project runs: having something to run and no way to
prove it works. Without a declared gate no task can name a verification, so no
task can be created, so the approval gate never engages — the framework would be
installed and inert. The message now says that rather than "add a gate".

## F-21 · The scope boundary held

The question that prompted this was whether the CLI should propose a
`docker-compose.yml` from a requirements sheet. Decided: **no.** Circle reads
what exists and writes it down; it does not choose a stack.

Every spec-driven framework already generates scaffolding, and the review found
their common weakness is brownfield. Moving into greenfield generation would put
Circle in a crowded field where it has no advantage and give up the one it has.

What it does instead is cheap and in scope: register `REQUIREMENTS.md`, `PRD.md`,
`SPEC.md` and the usual `docs/` subdirectories as definitions, so the only
artifact a new project has is not wasted.
