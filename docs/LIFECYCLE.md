# The Circle lifecycle — design draft

**Status: draft for review.** Nothing in this document is built yet. The skills
it describes live in [`plugin/drafts/`](../plugin/drafts/), where the plugin
loader does not see them, and they call CLI commands listed under
[CLI surface](#cli-surface) that do not exist yet. Review the skills and this
document together; build the CLI once both are agreed.

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

`/circle:run`, `/circle:verify` and `/circle:brief` are unchanged and are not
redrafted.

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
| **Glossary / ADRs** | Shared language and hard-to-reverse decisions | `GLOSSARY.md`, `docs/adr/` — registered as knowledge | Vocabulary every skill uses |

### Item states

```
shaping ─▶ shaped ─▶ specified ─▶ planned ─▶ approved ─▶ building ─▶ reviewing ─▶ done
   ▲          │          │           │          │                        │
   └──────────┴──────────┴───────────┴── plan changes void approval ◀────┘ blocking finding
```

Each transition is a CLI command that checks its precondition and exits 2 if it
does not hold:

| Transition | Command | Refuses when |
|---|---|---|
| → `shaped` | `circle item shape <id> --from shape.md` | `shape.md` has an unanswered question in its frontier section |
| → `specified` | `circle spec check` | A required section is empty, or **a seam names no declared quality gate** |
| → `planned` | `circle task create` (×n), then `circle item plan` | Any task lacks goal, gate or paths; the graph has a cycle |
| → `approved` | `circle brief approve` (human) | Unchanged from today |
| → `building` | `circle task claim` | No approval, or approval is stale |
| → `reviewing` | automatic when the last task closes | — |
| → `done` | `circle review close` | A blocking finding is open, or `circle quality run` is red |

**This is the extensibility mechanism.** A skill declares the state it requires
and the state it produces. Its first injected command is
`circle item require --state <s>`, which exits 2 and aborts the skill when the
item is in the wrong state. A team's custom skill gets the same enforcement as a
built-in one by declaring two fields. See [Skill contract](#skill-contract).

### Session binding

Today every enforcing path hardcodes the item `"default"`
(`internal/cli/gate.go:145`, `status.go:35`, `timeline.go:273`,
`serve/server.go:74`). That makes two items in one repo impossible: they share
one plan hash, so changing one voids the other's approval. Binding fixes it.

A session's item resolves in this order:

1. `.circle/runtime/sessions/<session_id>.toml` — written by `circle session bind`. Hooks receive `session_id` on stdin.
2. The worktree name — `claude --worktree feat-1` binds to item `feat-1`; the `WorktreeCreate` hook already knows the name.
3. `$CIRCLE_ITEM`.
4. `default` — so a repo with one item behaves exactly as it does today.

The `PreToolUse` gate checks **the bound item's** approval and **the claimed
task's** paths, not the union of every task in the repo.

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

`circle skill validate` (Phase 10) lints exactly these five points. A
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

Everything here is **new**. Grouped by the skill that first needs it.

| Command | Needed by | Does |
|---|---|---|
| `circle session bind --item <id>` / `session current` | start | Binds this session; hooks resolve the item from it |
| `circle item new <id> --title <t> [--initiative <i>] [--kind feature\|fix\|refactor]` | shape, hunt | Creates `.circle/items/<id>/`, state `shaping` |
| `circle item show [<id>] [--json]` / `item list` | all | State, initiative, tasks, approval |
| `circle item require --state <s>` | all orchestrators | Exit 2 unless the bound item is in state `s` |
| `circle item shape --from <file>` | shape | → `shaped` |
| `circle item plan` | tasks | → `planned`; validates the graph |
| `circle knowledge add <p> --as glossary\|standards` | shape, retro | Two new knowledge classes; drift detection covers both |
| `circle spec check [--item <id>]` | spec | Sections present; every seam maps to a declared gate |
| `circle task create --item <id> ...` | tasks | Same contract as today; writes the item's own `tasks.jsonl`; generates a hash id when `--id` is omitted |
| `circle task claim` (changed) | implement | Claims through the shared, locked `claims.json`, so every worktree sees it |
| `circle task ready --item <id> --json` | implement | The ready set |
| `circle quality run <gate> --task <id> --expect red\|green` | tdd | Records the cycle; `--expect red` passes only when the gate fails |
| `circle review drift` | review | Alias of `circle timeline drift`, scoped to the item |
| `circle review record --axis standards\|spec\|trace --file <f>` | review | Stores findings; blocking ones hold the item in `reviewing` |
| `circle review close` | review | → `done` |
| `circle initiative new <id> --destination <d>` | chart | Creates the initiative |
| `circle initiative show [<id>]` | chart, next | The map at low resolution |
| `circle decision new --initiative <i> --type <t> --question <q> [--blocked-by <ids>]` | chart | Creates a decision |
| `circle decision frontier [--initiative <i>]` | chart, next | Open, unblocked, unclaimed decisions |
| `circle decision claim <id>` / `resolve <id> --answer <file>` / `scope-out <id> --why <w>` | chart | Claim, close with an answer, rule out of scope |
| `circle handoff write [--item <id>]` | handoff | Writes `handoff.md` from the timeline |
| `circle quality add <name> <cmd>` / `circle knowledge add` | retro | Retro's two kinds of output |
| `circle next [--json]` | next | State → suggested skill and phase-boundary move |
| `circle event record --type skill.run --skill <s>` | hooks | Skill telemetry |
| `circle event list --item <id> --types <t,...>` | retro | The event log, filtered |
| `circle sync github` | — | The projection above |

Flags added to commands that already exist:

| Flag | Needed by | Does |
|---|---|---|
| `circle knowledge list --as <class> --content` | domain, review | Prints the registered files' contents, not just their paths |
| `circle task show --current` | tdd | The task this session has claimed, with its spec seams |
| `circle item show --with shape\|spec` | spec, tasks | Inlines the named artifact |
| `circle item list --open` | start | Items not yet `done` |
| `circle item require --state a --state b` | implement | Repeatable: passes on any listed state |
| `circle session bind --decision <id>` | start, chart | Binds to a decision instead of an item |

---

## Open questions

1. **Glossary location.** Repo root `GLOSSARY.md` (Matt's convention, visible to
   every tool) or `.circle/glossary.md` (Circle's state directory)? Drafts assume
   the root, registered as knowledge.
2. **Decision granularity for `prototype`.** Matt keeps prototypes on a
   `prototype/<name>` branch. Does a prototype get its own worktree and item,
   or stay a branch the decision links to? Drafts assume a branch.
3. **Phase 7 barrier.** The PRD makes the cold-start trial a hard stop before
   v0.2. This lifecycle is v0.2 scope. Session binding (the `"default"` fix)
   arguably is not.
