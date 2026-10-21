# The Circle lifecycle — design draft

**Status: built.** The skills ship in [`plugin/skills/`](../plugin/skills/), every
command they call exists, and a test fails the build if one ever does not
(`TestShippedSkillsConform`). The one deferred piece is the GitHub projection —
see [CLI surface](#cli-surface).

The ideas are adapted from Matt Pocock's skill set
([mattpocock/skills](https://github.com/mattpocock/skills)). The adaptation rule
is one sentence:

> **The method stays in prose. State moves into `.circle/`. Enforcement moves
> into the CLI and the hooks.**

A Matt skill asks the model to behave. A Circle skill asks the model to behave
*and* hands every claim that matters to a command whose exit code decides it.

---

## The flow

```
                       ┌──────────── on-ramps ────────────┐
                       │ /circle:hunt   (something broke) │
                       │ /circle:chart  (too big, foggy)  │
                       └───────────────┬──────────────────┘
                                       ▼
 /circle:start ─▶ /circle:shape ─▶ /circle:spec ─▶ /circle:tasks ─▶ /circle:brief ─▶ /circle:implement ─▶ /circle:review ─▶ /circle:handoff
   contracts       grill + domain    seams→gates     tracer bullets    HUMAN GATE       tdd per task        3 axes            from git
                                                                                                                  │
                                                         /circle:retro ◀──────────────────────────────────────────┘
                                                      repo learns: new gates,
                                                      new knowledge entries
                         /circle:next — at any point: where am I, what next, continue / clear / handoff / subagent / compact
```

| Step | Skill | Item state after | Who can invoke | Adapted from |
|---|---|---|---|---|
| Open | `/circle:start` | — (binds the session) | user | — (upgraded) |
| Route | `/circle:next` | — | user | `ask-matt` + phase boundaries |
| Chart | `/circle:chart` | initiative `charted` | user | `wayfinder` |
| Shape | `/circle:shape` | `shaped` | user | `grill-with-docs` |
| Spec | `/circle:spec` | `specified` | user | `to-spec` |
| Plan | `/circle:tasks` | `planned` | user | `to-tickets` (upgraded) |
| Gate | `/circle:brief` | `approved` | user (human approves) | — (unchanged) |
| Build | `/circle:implement` | `building` → `reviewing` | user | `implement` + `implement-spec` |
| Review | `/circle:review` | `done` or back to `building` | user | `code-review` |
| Hunt | `/circle:hunt` | new `fix` item, `specified` | user | `diagnosing-bugs` |
| Hand off | `/circle:handoff` | — | user | `handoff` (upgraded) |
| Retro | `/circle:retro` | — (edits the contract) | user | `retro` |
| *practice* | `circle:grill` | — | model | `grilling` |
| *practice* | `circle:domain` | — | model | `domain-modeling` |
| *practice* | `circle:tdd` | — | model | `tdd` |

`/circle:run`, `/circle:verify` and `/circle:brief` keep their bodies; they gained
the metadata block, and `brief` the gate injection.

### Two kinds of skill

Kept from Matt, because it is the right split:

- **Orchestrators** are user-invoked (`disable-model-invocation: true`). They
  move an item from one state to the next. An orchestrator may call practices;
  it never calls another orchestrator. It *suggests* the next one.
- **Practices** are model-invoked. They hold a reusable discipline — how to
  interview, how to name things, how to write a test — and are called by
  orchestrators or reached for by the model when the task fits.

Circle adds one rule: **a practice never changes item state.** Only
orchestrators do, and only through the CLI.

---

## State model

```
Initiative                  .circle/initiatives/<id>/       multi-session, foggy
  ├─ Decision               .circle/initiatives/<id>/decisions/<nn>-<slug>.md
  └─ Item ───────────────── .circle/items/<id>/             one brief, one approval
       └─ Task ──────────── .circle/items/<id>/tasks.jsonl  goal + gate + paths + deps
Session ── bound to ──▶ Item | Decision                     .circle/runtime/sessions/<id>.toml
```

| Thing | Is | Lives in | Source of truth for |
|---|---|---|---|
| **Initiative** | A destination too big for one session, charted as decisions | `.circle/initiatives/<id>/initiative.toml` + `map.md` | Destination, notes, decisions so far, not yet specified, out of scope |
| **Decision** | One question whose answer is a decision, not a deliverable. Typed `research`, `prototype`, `grill` or `task` | `.circle/initiatives/<id>/decisions/` | The question, its blockers, its claim, its answer |
| **Item** | One unit a human approves: a feature, a fix, a refactor. Optionally belongs to an initiative | `.circle/items/<id>/` | `item.toml` (state), `shape.md`, `spec.md`, `brief.md`, approval, `review.md`, `handoff.md`, `timeline.jsonl` |
| **Task** | One tracer-bullet slice of an item | `.circle/items/<id>/tasks.jsonl` (committed) + claims in the shared git dir | Goal, gate, paths, dependencies, closure |
| **Session** | One Claude Code session, bound to exactly one item or decision | `.circle/runtime/` (gitignored) | Which item the hooks enforce against |
| **Domain model** | Shared language per bounded context, hard-to-reverse decisions, and which domain owns each module | `DOMAIN.md` files (one per context, in the directory it covers), `adr/` directories, `ARCHITECTURE.md`; indexed into `.circle/DOMAIN-INDEX.md` | Vocabulary every skill uses — see [The domain model](#the-domain-model) |

### Item states

```
shaping ─▶ shaped ─▶ specified ─▶ planned ─▶ approved ─▶ building ─▶ reviewing ─▶ done
   ▲          │          │           │          │                        │
   └──────────┴──────────┴───────────┴── plan changes void approval ◀────┘ blocking finding
```

**The state is derived, never stored.** Every state is a fact already on disk, so
storing it as well would be a second copy that can disagree with the first.
`item.Derive` checks from the end of the lifecycle backwards and the first fact
that holds wins:

| State | Holds when | Produced by |
|---|---|---|
| `done` | `.circle/items/<id>/done.json` exists | `circle review close` |
| `reviewing` | approval valid, every task closed | the last `circle task close` |
| `building` | approval valid, some task closed or claimed | `circle task claim` |
| `approved` | approval valid for the current plan hash | `circle brief approve` |
| `planned` | tasks exist, no valid approval — including a **stale** one | `circle task create` |
| `specified` | `spec.md` exists | `/circle:spec`, checked by `circle spec check` |
| `shaped` | `shape.md` exists | `/circle:shape` |
| `shaping` | none of the above | `circle item new` |

A stale approval sends a half-built item back to `planned`: the plan it was
building against is no longer the one a human signed.

**This is the extensibility mechanism.** A skill declares the state it requires
and the state it produces. Its first injected command is
`circle item require --state <s>`, which exits 2 and aborts the skill when the
item is in the wrong state. A team's custom skill gets the same enforcement as a
built-in one by declaring two fields. See [Skill contract](#skill-contract).

### Session binding

Every enforcing path used to read the item `"default"`, so two items in one repo
shared a plan: planning one voided the other's approval, and a session on one
could write into the other. A session now resolves to exactly one item, first
match wins:

1. an explicit `--item` flag;
2. the session's binding — `circle session bind --item <id>`, stored in the
   shared git directory. The `SessionStart` hook exports the session id as
   `$CIRCLE_SESSION` through `$CLAUDE_ENV_FILE`, which is how the agent's own
   `circle` commands find it; the write hook reads it from its payload;
3. the worktree's name — `claude --worktree alpha` works on item `alpha`, and a
   worktree named after a task works on that task's item;
4. `$CIRCLE_ITEM`;
5. `default`, so a repository with one item behaves exactly as before.

The write hook then enforces **that item's** approval and radius:

| Situation | Writes allowed |
|---|---|
| Item approved | Inside its approved radius, plus any `DOMAIN.md` or ADR |
| Item with no plan yet (being shaped) | `DOMAIN.md` files, ADRs, architecture docs, and `.circle/` |
| Session bound to nothing, while other items have plans | Nothing outside `.circle/` — bind first |
| Repository with no plans at all | Everything (unchanged: adopting Circle must not block configuring it) |
| `approval.json`, `approved-radius`, `tasks.jsonl`, `done.json`, `DOMAIN-INDEX.md` | Never by hand — only through `circle` commands |

### The task graph stays in Circle

D-4 and D-24 stand: **Circle's CLI owns the task graph, and tasks live in
`.circle/`.** Beads was considered for dependency tracking and rejected again.
It would replace the easy part (a DAG, a ready set, claim, close), which Circle
already has. It would also move tasks out of git into Dolt, put a database
subprocess on every write-hook call, and give `bd close` a way past the closure
gate. The `TaskGraphPort` stays, so a beads adapter remains possible later if
many agents across machines ever need claims coordinated in real time.

Two changes to today's store, both needed for parallel work:

| Problem | Today | Change |
|---|---|---|
| **Claims are invisible across worktrees.** `tasks.jsonl` sits in each worktree's own `.circle/`, so two implementers in `/circle:implement --all` can both claim one task | Claim is a line in the per-worktree `tasks.jsonl` | **Claims move to the shared git directory**, `$(git rev-parse --git-common-dir)/circle/claims.json`, under `flock` — the same mechanism the port ledger already uses (`internal/worktree/ports.go:39-87`). Claims are runtime state, so they are not committed |
| **Ids collide across branches.** Two branches can each create `feat-3` | Human-chosen ids, repo-wide file | **Per-item files** (`.circle/items/<id>/tasks.jsonl`), and an omitted `--id` generates `<item>-<4 hex>` from a hash of title and time. A human-chosen id is still allowed and is checked for uniqueness within the item |

What stays committed and what does not:

| State | Where | Committed |
|---|---|---|
| Task definition: goal, gate, kind, paths, dependencies | `.circle/items/<id>/tasks.jsonl`, append-only | yes — it is what the plan hash and the brief cover |
| Closure: `closed_at`, `closed_by` (the gate-run event) | same file | yes — closure is a durable fact |
| Claim: who holds a task right now | `<git-common-dir>/circle/claims.json` | no — visible to every worktree on the machine |
| Cross-machine visibility | GitHub issue assignee, via the projection below | — |

`circle task close` stays the only way to close a task, so the gate rule holds.
The existing repo-level `.circle/tasks.jsonl` is read as the `default` item, so
current repos keep working without a migration.

### GitHub issues

A one-way **projection**, never a source of truth: `circle sync github` writes
an item as an issue (body = `brief.md`, which already renders mermaid), its tasks
as sub-issues with native blocking links, and an initiative as an issue
labelled `circle:initiative`. State is read from `.circle/`, never back from
GitHub. A claim also sets the task's issue assignee, which is how a teammate on
another machine sees who holds what. Phase 9's `TrackerPort` owns it.

### The domain model

The domain model is **kept current by Circle, not by memory.**

- **`DOMAIN.md`, one per bounded context**, in the directory it covers. Terms are
  `**Term**:` lines, banned synonyms an `_Avoid_:` line under them, owned modules
  backticked paths under `## Modules`. Format:
  [`plugin/skills/domain/DOMAIN-FORMAT.md`](../plugin/skills/domain/DOMAIN-FORMAT.md).
- **ADRs** are any `.md` under an `adr/` directory; **architecture documents** are
  `ARCHITECTURE.md` or anything under `architecture/`.
- **`.circle/DOMAIN-INDEX.md`** is generated from all of them plus every source
  module: domains and terms, decisions, architecture docs, and which domain
  documents each module. Deterministic, so "current?" is a byte comparison.

What keeps it current, with nobody remembering to:

| When | What runs | Effect |
|---|---|---|
| Every Edit / Write | `PostToolUse` → `circle domain sync --hook` | Regenerates the index. If the write added a type the file did not declare at HEAD with no term in any `DOMAIN.md`, used a banned synonym, or started a module nothing describes, the agent is told at once which `DOMAIN.md` it belongs in |
| Session start, `circle init` | `circle domain index` | Catches edits made outside Claude Code |
| Review, CI | `circle domain check --since <base>`, `circle domain index --check` | Judges a whole branch; fails CI on a stale index |
| Preflight | domain line in KNOWLEDGE | Domains, ADRs, stale index, undocumented modules — advisory |
| `circle knowledge verify` | drift detection | Checks every `DOMAIN.md`, ADR and architecture doc against the code, no registration needed |

Only what is **new** is flagged: a type already in the file at HEAD, or a module
already tracked, is not — so touching a legacy file does not block on old debt,
while every new concept is recorded from then on. Technical words (Service,
Repository, Handler, Config, Props …) and plurals are normalised away, so
`OrderRepository` is covered by **Order**.

```toml
[domain]
enforce = "block"            # default; "advise" turns the feedback into context
ignore  = ["*Options", "Mock*"]  # plumbing that is not a concept
```

---

## Skill contract

Every Circle skill, built-in or team-authored, declares itself in frontmatter
`metadata`. The fields are real Claude Code frontmatter, not a parallel schema:

```yaml
---
description: <leading word first; one trigger per branch>
disable-model-invocation: true          # orchestrators only
allowed-tools: Read Grep Bash(circle *)
metadata:
  circle:
    kind: orchestrator | practice
    requires: specified                  # item state; omitted for practices
    produces: planned
    reads:  [knowledge, quality, spec]
    writes: [tasks]
    calls:  [circle:grill, circle:domain]  # practices this skill may invoke
---
```

And every orchestrator body follows one shape:

1. **Gate injection.** First `!` block is `circle item require --state <requires>`.
   Exit 2 aborts the skill before the model sees it.
2. **Context injection.** The state the skill works on, from the CLI — never
   re-derived by the model.
3. **Steps,** each ending on a checkable completion criterion.
4. **Record.** State changes go through a CLI command that records the event.
5. **Next.** End by naming the next skill and the phase-boundary move.

`circle skill validate` checks the contract: the metadata block, that an
orchestrator is user-only and a practice is not, that `requires`/`produces` are
real states, that the first injected block is the gate covering every required
state, that every `circle` command named anywhere in the skill exists, and that
linked sibling files are there. `circle skill new` scaffolds a skill that already
passes. A
`PreToolUse` hook on the `Skill` tool records `skill.run` events, so `retro` and
the timeline can see which skills actually ran on an item.

### Lifecycle slots

Practices are swappable per repo, so a team can keep Circle's enforcement and
bring its own discipline:

```toml
[lifecycle]
grill  = "circle:grill"
domain = "circle:domain"
tdd    = "circle:tdd"         # e.g. "acme:property-tests"
```

An orchestrator never names a practice directly. It reads the slot from
`circle preflight --context` and calls whatever is there.

---

## CLI surface

✅ built · ⏸ deferred to Phase 9 (the tracker port). Grouped by the skill that first needs it.

| | Command | Needed by | Does |
|---|---|---|---|
| ✅ | `circle session start --hook` | SessionStart | Exports `$CIRCLE_SESSION`, opens the item's timeline once, refreshes the domain index |
| ✅ | `circle session bind --item <id> \| --decision <id>` / `session current [--why]` | start | Binds this session; prints the resolved item |
| ✅ | `circle item new <id> --title <t> [--kind] [--initiative] [--bind]` | shape, hunt | Creates `.circle/items/<id>/` |
| ✅ | `circle item list [--open] [--initiative] [--json]` / `item show [<id>] [--json]` | all | Items with their derived state |
| ✅ | `circle item require --state <s> [--state <s>]` | all orchestrators | Exit 2 unless the item is in one of the states |
| ✅ | `circle task create --item <id> ...` | tasks | Writes the item's own `tasks.jsonl`; hash id when `--id` is omitted |
| ✅ | `circle task ready [--item <id>] [--all] [--json]` | implement | The ready set, minus tasks claimed in any worktree |
| ✅ | `circle task claim <id> [--steal]` / `task release <id>` | implement | Through the shared, locked claim ledger |
| ✅ | `circle task show --current` | tdd | The task this session holds |
| ✅ | `circle domain index [--check] [--print]` | domain, CI | Regenerates / checks / prints `.circle/DOMAIN-INDEX.md` |
| ✅ | `circle domain check [--since <rev>] [--json]` | review, CI | New concepts and modules the model does not cover |
| ✅ | `circle domain sync --hook` | PostToolUse | Refreshes the index; feeds new concepts back to the agent |
| ✅ | `circle spec check [--item <id>]` | spec | Sections present; every seam maps to a declared gate |
| ✅ | `circle item show --with shape\|spec` | spec, tasks | Inlines the named artifact |
| ✅ | `circle quality run <gate> --task <id> --expect red\|green` | tdd | Records the cycle; `--expect red` passes only when the gate fails |
| ✅ | `circle review drift` / `review record --axis spec\|standards` / `review show` / `review close` | review | Computed trace axis; judged axes; → `done` |
| ✅ | `circle knowledge add <p> --as standards` / `knowledge list --as <c> --content` | review, retro | A standards class and its contents |
| ✅ | `circle initiative new\|list\|show`, `circle decision new\|link\|frontier\|claim\|show\|resolve\|scope-out` | chart, next | Initiatives and decisions |
| ✅ | `circle skill validate [paths]` / `skill new <name>` | extending | Check a skill against the contract; scaffold one that passes |
| ✅ | `circle task update <id>` | tasks | Change an open task's goal, gate, paths or dependencies; voids the approval |
| ✅ | `circle local set\|list\|export` | gates | Machine-local variables (a proxy CA, a mirror) for every gate and agent shell, shared by worktrees, never committed |
| ✅ | `circle preflight --context --advisory` | shape, chart, retro | Report a failing contract without aborting planning |
| ✅ | `circle quality run --task <id>` | tdd, implement | Runs the task's own verify command when it has one; the bootstrap runs first unless `--no-bootstrap` |
| ✅ | `circle handoff write [--item <id>]` | handoff | Writes `handoff.md` from the timeline |
| ✅ | `circle quality add <name> <cmd>` | retro | Adds a gate |
| ✅ | `circle next [--json]` | next | State → suggested skill and phase-boundary move |
| ✅ | `circle event list --item <id> --types <t,...>` / `event record --type skill.run` | retro, hooks | Filtered events; skill telemetry |
| ⏸ | `circle sync github` | — | The GitHub projection |

---

## Open questions

1. **Phase 7 barrier.** The PRD makes the cold-start trial a hard stop before
   v0.2. This lifecycle is v0.2 scope. Recommendation: record a decision moving the
   barrier after the foundation now built — a trial of one item at a time would
   not exercise what Circle now is.

## Decided

- **Domain model:** `DOMAIN.md` per bounded context, indexed automatically into
  `.circle/DOMAIN-INDEX.md`; new concepts and modules are caught at the write
  that adds them. No manual registration.
- **Prototypes:** a `prototype/<name>` branch the decision links to — never an
  item, since a prototype is thrown away and needs no approval.
- **Tasks:** stay in `.circle/`, owned by the Circle CLI; beads is not used.
- **Parallel initiatives:** one item per session, resolved as above; per-item
  plans and approvals; claims shared across worktrees.
- **Approval:** agents may approve; unchanged for now.
