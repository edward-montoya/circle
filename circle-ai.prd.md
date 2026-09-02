# Circle AI — A Result-Focused SDD Framework for Claude Code

**Binary**: `circle` · **State dir**: `.circle/` · **Skills**: `circle:*` · **License**: OSS (license TBD)

## Problem Statement

Every developer using Claude Code on a real project re-teaches the agent the same things from scratch: how to run the app locally, how to test it, what "good" looks like, and where the authoritative knowledge lives. That knowledge exists in someone's head or scrollback, never in the repository — so it cannot be replicated by a teammate, reused next session, or trusted by the agent. The cost is a workflow that is complex, non-transferable, and re-derived on every single session.

## Evidence

- **Primary (requester, direct)**: the stated pain is *"simplification of the workflow… teach Claude how to test, how to keep quality, how to execute the app locally. It will be easy to replicate."* The operative word is **replicate** — the failure mode is non-transferability, not merely forgetting.
- **Corroborating (external)**: the `beads` project exists because markdown plans fail as agent memory for long-horizon work, and has attracted an ecosystem around that premise ([gastownhall/beads](https://github.com/gastownhall/beads), [Better Stack](https://betterstack.com/community/guides/ai/beads-issue-tracker-ai-agents/)). This validates the *durable state* half of the problem independently — but beads addresses task memory, not execution or quality contracts.
- **Gap**: no evidence yet that non-requester developers experience this as their top pain. For an OSS launch this matters. **Validation method**: publish the README's problem statement and measure whether the "re-teaching the agent" framing resonates before building the dashboard.

## Proposed Solution

Circle AI encodes a project's **execution, quality, and knowledge contracts** into the repository once, then drives Claude Code through implementation against those contracts.

Six pieces:

1. **`.circle/`** — a git-tracked layout holding project contracts, implementation artifacts, work items, and session state.
2. **`circle`** — a Go CLI owning all domain logic; the sole backend for the skills, the hooks, and the web app.
3. **A Claude Code plugin** (D-21) bundling the skills, the enforcement hooks, and the brief assets. Plugin hooks run the moment the plugin is enabled — project `settings.json` hooks do not run until a workspace-trust dialog is accepted, which would break enforcement at exactly the moment a teammate clones the repo.
4. **Hook-enforced gates** (D-22) — `PreToolUse` denies file writes without an approval event; a `` !`circle gate check` `` injection aborts any Circle skill whose contracts don't validate; a `Stop` hook refuses to end a turn on red gates. **The skills are the interface; the hooks are the enforcement.**
5. **Worktree-isolated local execution** — a `WorktreeCreate` hook layers Compose project namespacing and port allocation over Claude Code's native worktrees (D-23), so running and testing is safe and parallel.
6. **A human comprehension gate** — before any code is written, Circle renders a diagram-first brief (module + sequence diagrams, examples, blast radius, risks) that a human must explicitly approve.
7. **A local web app** served by the CLI — observation and configuration only, never execution. *(A minimal read-only page ships in v0.1 alongside the event layer, D-40; the full eleven-screen app is v0.2.)*

The framework covers **implementation only**. It consumes definition artifacts rather than producing them, and assumes an existing stack and structure — there is no greenfield/brownfield branch.

The core bet: *the reason agent workflows don't replicate is that the project's operating knowledge is never written down in a machine-usable form.* Circle AI's job is to make writing it down cheap, validate it, and then enforce it.

## Key Hypothesis

We believe that **encoding execution, quality, and knowledge-base contracts in the repo, enforced by a preflight gate**, will make agent-driven implementation workflows replicable by any developer on the team.

We'll know we're right when **a developer who has never worked on the repo can clone it, run `circle init` → `circle preflight` → `/circle:start`, and land a correct PR without asking anyone how to run or test the project.**

> Requester's answer to the success-signal question was *"One month work"*, which reads as a **delivery constraint for v0.1** rather than a metric. Interpreted as such throughout this document. **Confirm.**

## What We're NOT Building

| Not building | Why |
|---|---|
| Product definition tooling (PRD/UX authoring) | Explicit constraint — the framework consumes definition artifacts |
| Greenfield scaffolding or stack selection | Explicit constraint — existing structure and stack are preconditions |
| A hosted service or shared database | Collaboration happens through git; a server breaks the "share via repo" model |
| An agent runner in the web app | All interaction happens in Claude Code; the app suggests, never dispatches |
| A CI system | Circle runs quality gates locally and *reads* CI status; it does not replace it |
| A general-purpose project management tool | Scope is one team's implementation loop, not portfolio management |
| Non-Compose execution models | Compose is the assumed substrate. Bare-metal/k8s projects are out of scope for v1 |
| A replacement for plan mode | Plan mode is already a pre-code human gate. The brief gate differs on three axes only: **durable** (approval is a repo event surviving the session), **structured** (deterministically assembled diagrams, not prose), and **reused** (`brief.md` becomes the PR body). If it isn't clearly all three, it is plan mode with extra steps |
| A replacement for `/run` and `/verify` | The bundled skills record a launch recipe into the repo. Circle **consumes** that recipe as a contract detection source (D-25) rather than competing with it |

## Success Metrics

> **Gap in the original set:** every metric below the first two measures the *artifact* (contract completeness, skill conformance, gate integrity). They can all pass while the product is useless. The metric the hypothesis actually claims is **correction rate**, added at the top.

| Metric | Target | How Measured |
|---|---|---|
| **Correction rate** (primary) | Measurably fewer human interventions to reach a correct PR, with Circle vs. without | **Paired trials, N=5**: same task, same repo, with/without Circle. Count corrections, re-prompts, and manual fixes |
| **Cold-start replication** | A developer new to a repo lands a correct PR with zero out-of-band setup questions | **Scripted, not judged**: fresh container, `claude -p`, assert `circle preflight` exit 0 and all quality gates green with no human turn. N=3 |
| Time-to-first-preflight-pass | < 15 min on a repo with an existing compose file | Timed `circle init` → `circle preflight` runs |
| Quality gate determinism | 100% of status scores backed by executed gates, not inference | `circle score explain` audit |
| Contract completeness | ≥70% of `goal.md` criteria machine-checkable | `circle goal validate` ratio |
| Worktree isolation | N parallel work items run compose stacks with zero port/volume collisions | Concurrency test, N=3 |
| **Brief comprehension** | A reviewer understands scope, blast radius, and risk from `brief.html` alone in < 5 min, without opening source artifacts | Timed trial, N=5, comprehension questions |
| Gate integrity | 0 work items reach implementation without a recorded approval event | `circle status --audit` |
| Diagram accuracy | Module diagrams match the actual touched module set post-merge | ~~Manual comparison, N=5~~ → **`circle brief verify`**: declared blast radius vs. the change timeline's actual file set. Now automated (D-31) |
| **Timeline fidelity** | Timeline matches `git log` across amend, rebase, and squash-merge; 100% of agent-claimed commits verified against git | `circle timeline sync` reconciliation test; phantom-commit count |
| Skill conformance | 100% of forged skills pass `circle skill validate` | Lint on commit |
| OSS adoption (post-launch) | TBD | TBD |

> Targets are **proposed, not validated**.

## Open Questions

**Resolved by the platform/landscape review** *(see `circle-ai.review.md`)*

- [x] ~~Skills distribution~~ → **A Claude Code plugin.** `circle:*` namespacing exists only for plugins, and plugin hooks run without a workspace-trust dialog (D-21)
- [x] ~~beads storage backend~~ → **Dolt-only since early 2026.** Avoid the dependency for v0.1 (D-4 revised)
- [x] ~~Worktree base location~~ → **`.claude/worktrees/`, gitignored** — the native default (D-23)
- [x] ~~Brief rendering / committing `mermaid.min.js`~~ → **Ship it in the plugin** at `${CLAUDE_PLUGIN_ROOT}` (D-28)
- [x] ~~Does `circle:review` run as a subagent~~ → **Yes**: `context: fork` + `agent:` in skill frontmatter

**Still open**

- [ ] Is *"one month"* a delivery constraint (assumed) or the success metric? *Everything downstream depends on this — confirm first*
- [ ] **Name collision** with Circle (Circle Internet Financial / USDC) and CircleCI — rename now, or accept the discoverability and trademark cost?
- [ ] **What consumes a `--force` preflight bypass?** Currently nothing, which makes it a free bypass with paperwork. Proposal: degrade the Status score and block PR creation
- [ ] OSS license (MIT / Apache-2.0 / dual)
- [ ] Distribution: `go install`, GitHub release binaries, Homebrew tap, plugin `bin/` shim, or a combination. *Note: `bin/` is not permitted for plugins distributed through claude.ai org settings; Go cross-compiles a static binary for every platform from one machine, which is what makes the `bin/` shim viable at all (D-39)*
- [ ] Token/cost source: hook-captured only (shareable, approximate) vs. reconciled against `~/.claude/projects/` transcripts (precise, unshareable, format-unstable)
- [ ] Should brief approval be revocable mid-implementation, and what happens to in-flight work if the plan changes underneath it?
- [ ] Should forged skills be shareable *across* repos (a registry), or repo-local only? Repo-local is the v0.1 assumption

---

## Users & Context

**Primary User**

- **Who**: A working developer using Claude Code for substantial implementation on an existing codebase with an established stack. Not a prototyper, not a PM.
- **Current behavior**: Re-explains how to run and test the project at the start of every session; maintains ad-hoc markdown plans; validates progress by reading diffs and trusting the agent's summary.
- **Trigger**: Starting work that spans multiple sessions, or joining a repo someone else has been agent-driving.
- **Success state**: The project tells the agent how to run, test, and validate itself. Nobody has to explain it again.

**Job to Be Done**

When **I start implementation work on a project with Claude Code**, I want to **have the project's execution, quality, and knowledge contracts already encoded and enforced**, so I can **get correct results without re-teaching the agent, and so anyone else can do the same**.

**Non-Users**

- PMs and designers — the framework starts *after* definition
- Teams wanting a hosted board or portfolio view
- Projects with no established structure, stack, or Compose setup

**Constraints**

- CLI must be Go (D-39, reversing the original Rust constraint); web app plain JS or a very small framework, performance-first
- Web app is tracking + configuration only; all interaction happens in Claude Code
- State must be shareable via git
- Docker Compose is the execution substrate; git worktrees provide isolation
- **v0.1 must be deliverable in one month**
- Open source — must work on an arbitrary third-party repo with no configuration inherited from the author's environment

---

## Solution Detail

### The Three Contracts

This is the conceptual core of Circle AI, and it maps one-to-one onto the three indicators.

| Contract | Question it answers | Lives in | Feeds indicator |
|---|---|---|---|
| **Knowledge** | Where is the truth about this project? | `.circle/project.toml` → `[knowledge]` | Unknown |
| **Execution** | How do I run this, in isolation? | `.circle/project.toml` → `[execution]` + `workspace.md` | Complexity |
| **Quality** | How do I prove it's correct? | `.circle/project.toml` → `[quality]` + `goal.md` | Status |

**Repo-level contracts are stable and reusable** (`project.toml`) — this is what makes the workflow replicable. **Session-level contracts are per-work** (`workspace.md`, `goal.md`) and inherit from the repo level. This satisfies the "two documents per app session" requirement while keeping the durable knowledge where it can be shared.

**Knowledge base registry** — the requester's explicit requirement, split into three categories:

```toml
[knowledge]
docs        = ["docs/", "README.md", "services/*/README.md"]   # how the system works today
definitions = [".circle/artifacts/", "docs/adr/"]              # what should be built
validations = ["tests/", "e2e/", ".github/workflows/"]         # how correctness is proven
```

Skills consult the registry instead of grepping blindly. `circle preflight` verifies every path resolves. Coverage of `definitions` against a work item is a direct input to the **Unknown** score.

**Quality contract** — "teach Claude how to keep quality," made executable:

```toml
[quality]
format    = "pnpm prettier --check ."
lint      = "pnpm eslint ."
typecheck = "pnpm tsc --noEmit"
[quality.test]
unit        = "pnpm vitest run"
integration = "pnpm vitest run --config vitest.integration.ts"
e2e         = "pnpm playwright test"
[quality.coverage]
min = 80
```

`circle quality run [gate]` executes these and records results as events. Because gates are commands with exit codes, the **Status** score becomes deterministic rather than inferred — this is the single highest-leverage design decision in the framework.

**Execution contract** — "teach Claude how to run the app locally," with worktree isolation:

```toml
[execution]
compose      = "docker-compose.yml"
app_services = ["api", "web"]        # the containers holding real code (see D-10)
[execution.worktree]
base       = "../.circle-worktrees"
isolation  = "compose-project"       # COMPOSE_PROJECT_NAME=circle-<item-id>
port_range = [42000, 42999]
```

### Worktree-Isolated Execution

Requested explicitly, and non-trivial: parallel worktrees each running Compose will collide on project names, ports, and volumes unless managed.

> **Revised (D-23).** Claude Code shipped native worktrees in v2.1.49: `claude --worktree <name>`, worktrees under `.claude/worktrees/`, `EnterWorktree`/`ExitWorktree`, `.worktreeinclude` for carrying gitignored files like `.env`, `worktree.baseRef`, automatic cleanup with locking, and hard isolation enforcement at the tool layer. Circle must **not** build a parallel `circle worktree create|list|destroy` tree. It implements a `WorktreeCreate` hook that adds the one thing Claude Code does not do.

`WorktreeCreate` hook (fires on `claude --worktree <item-id>`; any non-zero exit aborts creation):
1. Lets the native flow create the worktree under `.claude/worktrees/<item-id>/` (gitignored)
2. Sets `COMPOSE_PROJECT_NAME=circle-<item-id>` — isolates networks, volumes, container names
3. Allocates free host ports from `port_range`, writes a `.env` override into the worktree
4. Records the mapping into `compose.map.toml` so `workspace.md` can reference real, live ports
5. `WorktreeRemove` hook runs `compose down -v` before the native cleanup removes the worktree

The hook **generates and records** this configuration; **Claude executes** the compose commands (D-11). Circle never runs the app itself.

> **Implementation trap.** Once Claude enters a worktree, `${CLAUDE_PROJECT_DIR}` still points at the **main checkout** — only the `cwd` field in the hook's stdin JSON follows. Every Circle hook script must read `cwd` from stdin. Assuming `CLAUDE_PROJECT_DIR` means reading and writing the wrong repo during exactly the parallel work this section exists to enable.

### Repo Impact Detection

Requester's requirement: *"the app should be able to identify the repos that needs to be modified per the epic."*

Same philosophy as scoring — **deterministic signals propose, the LLM classifies from a fixed list, a human confirms.**

| Signal | Source | Determinism |
|---|---|---|
| Explicit path/service references in the epic's artifacts | Artifact parse | Deterministic |
| Compose service → repo mapping | `compose.map.toml` + `workspace.toml` | Deterministic |
| Cross-repo dependency graph | Package manifests, import scanning | Deterministic |
| Historical co-change | Git log: which repos change together for comparable work | Deterministic |
| Ownership hints | `CODEOWNERS`, path conventions | Deterministic |
| Residual selection | LLM picks affected repos **from the registered repo list** (a closed enum) with per-repo justification | Classification only |

Output: a ranked per-repo impact set with confidence, written to `epic.toml` after human confirmation. It then drives PR creation — **one epic ⇒ one GitHub issue in the anchor repo ⇒ one PR per affected repo**, all cross-linked (D-2, confirmed by requester).

### The Task Contract

Requester's requirement: *"each task MUST have a clear goal and a definition how to test it (e2e or unit)."*

> **Drawn in [A-2](https://claude.ai/code/artifact/a269e827-a65f-4c3d-9434-6d9e58f0b1a4), screen S04** (the task graph and per-task detail) and specified in full in **[A-3](https://claude.ai/code/artifact/5dad1144-5059-4f1f-a0f7-b9c412af36f4)**.

**First, a naming fix.** Earlier drafts used three words for one thing — *item*, *task*, and *unit*. That is why tasks were invisible in the mockups. One word now:

| Term | Means | ID shape | Carries |
|---|---|---|---|
| **Item** | The container a session works on. Kind: `epic`, `task`, or `fix` | `epic-31` · `task-118` · `fix-204` | brief, timeline, addenda |
| **Task** | The unit of work. Standalone, or a child of an epic or a larger task | `task-118.1` · `epic-31.4` | **goal + verification**, dependencies, state |
| ~~Unit~~ | Removed — a third name for a task | | |

A standalone task is a task with no parent, so the fix, task, and epic flows share one code path. Hierarchical IDs follow the beads model already cited in the research summary.

**Every task carries a goal and a way to test it.** A task without a stated goal is a prompt; a task without a verification command closes on the agent's word. Both become structurally impossible:

```toml
# .circle/items/tasks/task-118/tasks/task-118.3.toml
id         = "task-118.3"
parent     = "task-118"
title      = "429 body + Retry-After header"
state      = "ready"                  # ready | claimed | closed | blocked
depends_on = ["task-118.2"]

[goal]
statement = "A rate-limited client receives a 429 carrying a Retry-After header."
# REQUIRED. One sentence. What "done" means for this task alone.

[verification]
kind    = "e2e"                       # unit | integration | e2e | manual
gate    = "test:e2e"                  # must name a gate declared in [quality]
command = "pnpm playwright test search/rate-limit"
expect  = "exit 0"
# REQUIRED. kind = "manual" additionally requires justification + reviewer,
# and counts against the machine-checkable ratio.
```

**Two hook-enforced rules make this real** (D-35), using the two Claude Code hooks that exist for exactly this:

- `TaskCreated` → exit 2 **prevents creation** of a task with no goal statement, no verification, or a gate not declared in the quality contract.
- `TaskCompleted` → exit 2 **prevents closure** until an event exists showing the verification command passed *after* the task's most recent commit.

**The consequence:** the *task closure* component of the Status score (20%) stops being an assertion and becomes a measurement. Before, closure meant the agent marked it done; now it means a named command exited zero after the last change. That is the same shift the change timeline made for commits — and it means all four Status components are now evidence-backed.

**The cost, stated plainly.** Some tasks genuinely resist automated verification — a copy change, a log message, a design tweak. `kind = "manual"` exists for those, but demands a written justification and a named reviewer, and lowers the ratio `circle goal validate` reports. The friction is deliberate: manual should be visible, not free. **If most tasks land on `manual` in real repos, the 70% machine-checkable target is wrong and should be lowered rather than gamed** — that is a Phase 0 finding, not a v1.0 discovery.

### Change Timeline

Requester's requirement: *"identify all the commits executed in the session and list all the changed files — not the exact changes. Commit number and file changes. Like a timeline."*

> **Drawn in [A-1](https://claude.ai/code/artifact/080c6119-47b4-4674-b95c-53ae14483feb), screen 05** (terminal) and **[A-2](https://claude.ai/code/artifact/a269e827-a65f-4c3d-9434-6d9e58f0b1a4), screen S05** (app), including the `rewritten`, `phantom`, and `uncommitted` states.

**Claude Code cannot answer this.** Checkpointing looks close but is a different thing: it captures only edits made by Claude's file-editing tools, is explicitly **not tracked for bash-command changes** (so `git commit` itself is invisible to it), **does not capture subagent edits** except foreground forked skills, does not see edits from other sessions or the human, is capped at 100 checkpoints, is deleted after 30 days, is local-only, and the docs state plainly that it is *"not a replacement for version control."* It answers "undo my last few prompts," not "what landed."

There is also a documented failure mode where Claude **reports commits that never happened** ([anthropics/claude-code#44035](https://github.com/anthropics/claude-code/issues/44035)). That is the strongest argument for this feature and it sets the design rule:

> **The timeline is git-truth, never agent narration.** The same principle as D-5 and D-17: deterministic signals compute, the model never asserts.

**Capture: boundary + attribution, not interception.**

The obvious implementation — a `PostToolUse` hook matching `Bash(git commit *)` — is wrong on its own. It misses commits the human makes in another terminal, misses `--amend`, rebase, and `gh pr merge`, and records *intent* rather than *outcome*, which is exactly the phantom-commit bug. So two layers:

| Layer | Mechanism | Role |
|---|---|---|
| **Truth set** | Baseline SHA recorded at `SessionStart` and at `WorktreeCreate`, then `git rev-list <baseline>..HEAD` per repo + branch | **Authoritative.** Every commit, regardless of who made it or how |
| **Attribution** | `PostToolUse` on Bash + the existing event store | Best-effort *only*: which task, which Claude session, which tool call sat next to each commit. Never the source of truth |
| **Reconciliation** | `circle timeline sync` | Commit in git but unattributed ⇒ listed as `unattributed`. Attributed but absent from git ⇒ flagged **`phantom`**, loudly |

Baselines are recorded per `(repo, worktree, branch)` — under the isolation model each work item is on its own branch in its own worktree, so `HEAD` in the main checkout never moves. The hook reads `cwd` from its stdin payload to identify which, per the trap noted above.

**What each entry holds** (no diffs — `git diff-tree --no-commit-id --name-status -r -M <sha>`):

```jsonl
{"seq":3,"sha":"a3f81c9","short":"a3f81c9","subject":"add rate limit middleware",
 "authored":"2026-08-29T14:22:03Z","author":"eng@example.com","branch":"worktree-task-118",
 "files":[{"status":"A","path":"src/mw/rate-limit.ts"},{"status":"M","path":"src/app.ts"},
          {"status":"R","path":"src/mw/index.ts","from":"src/middleware/index.ts"}],
 "attribution":{"item":"task-118","claude_session":"01MYnP…","confidence":"observed"},
 "state":"live"}
```

**History rewrites are the hard part, and the reason this isn't a one-day feature.** Agents amend constantly, and a squash-merge erases every session commit from the default branch. On each `sync`, any recorded SHA that fails `git cat-file -e <sha>^{commit}` is marked `rewritten` and its successor resolved by `git patch-id`, then reflog, then subject + author-date. The entry is superseded rather than deleted, so the timeline survives a real PR workflow instead of silently emptying after merge.

**The uncommitted tail matters as much as the commits.** `git status --porcelain` at handoff appends an `uncommitted` entry — most sessions end with work that isn't committed yet, and a timeline that omits it lies by omission at exactly the moment someone is resuming cold.

**Rendering.** Default is a markdown table, emitted into `brief.md` (so the **PR body carries its own change timeline**), `review.md`, `circle status`, and `--format json` for the web app. A mermaid `gitGraph` is available behind `--graph`, but the table is the default: `gitGraph` renders poorly for long linear histories, which is the common case.

**What this unlocks beyond the stated ask** — three things fall out for free, and one of them fixes an existing hole in this document:

1. **The diagram-accuracy metric finally has an instrument.** *"Module diagrams match the actual touched module set post-merge"* was specified with no way to measure it. `circle brief verify` now computes it: the brief's declared blast radius against the timeline's actual file set.
2. **Scope-drift detection.** Files touched that fall outside the approved brief's blast radius are a first-class finding — the empirical companion to brief invalidation (D-15). The gate says what was approved; the timeline says what happened.
3. **Honest handoff.** `circle:handoff` and cold resume stop depending on the agent's summary of its own work.

**Explicitly out of scope**: diff content. The requester ruled it out, the size cost is real, and `gh pr diff` already exists.

### File Structure

Lives in the **anchor repo** (D-2).

```
.circle/
  project.toml                   # THE replicable contract: knowledge + execution + quality
  settings.toml                  # trackers, scoring weights, model routing (env-specific)
  workspace.toml                 # multi-repo membership; anchor declaration
  artifacts/                     # implementation artifacts — consumed, not authored
    prd/  architecture/  ux/  plan/  addenda/
  assets/
    brief.assets.lock            # D-28: mermaid ships in the plugin (${CLAUDE_PLUGIN_ROOT}),
                                 # never committed here. This records the version used
    brief.css
  sessions/
    <app-session-id>/
      session.toml
      workspace.md               # session execution contract (inherits project.toml)
      goal.md                    # success definition
      compose.map.toml           # parsed services, app-container marks, live port map
      claude/                    # ← folder the web app watches
        <claude-session-id>/
          summary.md
          artifacts/
  items/
    epics/<id>/  { item.toml, plan.md, impact.toml, tasks.jsonl,
                   brief.md, brief.html, review.md, timeline.jsonl, addenda/ }
    tasks/<id>/  { item.toml, brief.md, brief.html, review.md, timeline.jsonl, addenda/ }
    fixes/<id>/  { item.toml, repro.md, brief.md, brief.html, timeline.jsonl, addenda/ }
                 # timeline.jsonl is DURABLE, not runtime: the commit set is
                 # regenerable from git, but the attribution is not (D-32)
  scores/<item-id>.json          # cached score + signal-bundle hash + explanation
  runtime/                       # GITIGNORED — volatile, regenerable
    events/<claude-session-id>.jsonl
    usage/<claude-session-id>.jsonl
    worktrees.lock               # port allocations, active worktrees
    cache/
```

`.beads/` stays at the anchor repo root, owned by `bd`, when present.

### Skill Set

Not one skill — a set, each owning one phase, each a thin client over `circle`.

> **Skills cannot block (D-22).** A skill is markdown injected into context; the model may ignore it, and auto-compaction re-attaches skills under a 5k-per-skill / 25k-total token budget, dropping older ones entirely. Every gate below is enforced by a **hook**, and the skill is the interface to it. See `circle-ai.review.md` §1.1.

| Skill | Responsibility | Enforced by | v0.1 |
|---|---|---|---|
| `circle:preflight` | Validates the three contracts + knowledge paths | `` !`circle gate check` `` in every skill body — a non-zero exit **aborts the skill invocation** before Claude sees it | ✅ |
| `circle:start` | Mandatory opener — epic / task / fix; binds the Claude session; **arms the write-block hook via skill frontmatter `hooks:`** | Skill-registered `PreToolUse` hook, persists for the session | ✅ |
| `circle:brief` | **Generates the human comprehension artifact and waits for approval** | `PreToolUse` on `Edit\|Write\|NotebookEdit` → `permissionDecision: "deny"` until an approval event exists | ✅ |
| `circle:run` | Brings up the worktree-isolated stack per the execution contract | — | ✅ |
| `circle:verify` | Runs `goal.md` criteria + quality gates; drives the status indicator | `Stop` hook exit 2 — the turn cannot end with red gates | ✅ |
| `circle:handoff` | Writes addendum + session summary for cold resume, **anchored on the change timeline rather than the agent's account of its own work** | — | ✅ |
| `circle:intake` | Ingests PRD / UX / architecture / plan into `.circle/artifacts/` | — | v0.2 |
| `circle:epic-plan` | Decomposes epic → task graph; repo impact detection; opens issue + PRs | `TaskCreated` / `TaskCompleted` hooks, exit 2 | v0.2 |
| `circle:task-run` | Executes one unit; runs quality gates as it goes | `TaskCompleted` hook — a task cannot close with a failing gate | v0.2 |
| `circle:fix` | Short-circuit path for bugs — no task graph | — | v0.2 |
| `circle:hunt` | Bug hunting grounded in the execution contract | — | v0.2 — overlaps bundled `/debug` |
| `circle:review` | PR review: transparency, architecture, clarity, simplicity, pipeline. Runs as `context: fork` + `agent:` | — | v0.2 — overlaps bundled `/code-review` |
| `circle:skill-forge` | Authors new framework-conformant skills — the extensibility mechanism | `circle skill validate` in CI | v0.2 |

### The Human Comprehension Gate (`circle:brief`)

Requester's constraint, stated directly: *"Humans cannot read hundreds of lines + multiple documents. Summaries and diagrams are the keys."*

> **Drawn in [A-1](https://claude.ai/code/artifact/080c6119-47b4-4674-b95c-53ae14483feb), screen 04** — the full rendered page, both diagrams, and the approval bar. Read it before implementing this section.

So implementation is **blocked** until a human has seen and approved a rendered brief. This is not a report produced after planning — it is a gate standing between planning and any code being written.

**What the brief contains** (generated from state already in `.circle/`, never hand-written):

| Section | Source | Why it's there |
|---|---|---|
| Goal | `goal.md` | What success looks like, in the human's own words |
| Plan summary | Task graph, ordered by readiness | The whole plan on one screen |
| Definitions consulted | Knowledge registry hits | Traceability — what the agent actually read |
| **Module diagram** | Repo impact + compose map + dependency graph | Which modules/services this touches |
| **Sequence diagram** | Planned runtime flow of the change | How it will actually behave |
| **Examples** | Before/after, sample request/response, sample invocation | Concrete beats abstract |
| Repo impact + PR plan | `impact.toml` | Which repos get PRs, and why |
| Risks & open unknowns | Unknown-score signals | What could still go wrong |

**Two rendered outputs, deliberately:**

- **`brief.md`** — mermaid in fenced blocks. GitHub renders mermaid natively in markdown, so this doubles as the **PR body**, giving free diagram rendering in review. Committed.
- **`brief.html`** — richer local view: inline CSS, mermaid source in `<pre class="mermaid">`, referencing a single committed `.circle/assets/mermaid.min.js`. No build step, no CDN, works offline. Committed and shareable via the repo, per the collaboration constraint. Also served by `circle serve`.

**Approval is a recorded event, not a vibe.** `circle brief approve <item-id>` writes an approval event; `circle:task-run` refuses to execute without one. `circle brief status` shows pending briefs. A brief is invalidated when the plan or impact set changes, forcing re-approval.

Diagram generation follows the same determinism rule as scoring: **Go assembles the graph from real signals** (compose map, import graph, impact set, task graph) and emits mermaid; the LLM only supplies labels, the sequence narrative, and examples. Module diagrams are therefore structurally accurate rather than imagined.

### Review, Hunt, and Forge

**`circle:review`** — PR review across the five dimensions the requester named. Each maps to a checkable source:

| Dimension | How it's assessed |
|---|---|
| Transparency | Does the PR link its brief, issue, and tasks? Are decisions recorded as addenda? **Is every commit traceable to a task, and every changed file inside the approved blast radius?** — computed from the change timeline (`circle timeline drift`), not judged |
| Architecture alignment | Diff checked against `.circle/artifacts/architecture/` and the knowledge registry — deviations must be justified or become addenda |
| Clarity | Naming, structure, comment density measured against the surrounding code rather than an absolute standard |
| Simplicity | Speculative abstraction, YAGNI violations, diff size vs. declared task scope |
| Pipeline | `circle quality run` locally + `gh pr checks` remotely |

Output is structured findings written to `.circle/items/<id>/review.md` and posted as PR comments via `gh`. Findings are severity-ranked; only CRITICAL/HIGH block.

**`circle:hunt`** — bug hunting that is only possible *because* the execution contract exists. Process:

1. Bring up the isolated stack (`circle worktree` + execution contract)
2. **Derive expected behavior** from `goal.md`, the `definitions` registry, and the `docs` registry — stated explicitly before any observation, so it can't be rationalized backwards
3. Exercise the app and record observed behavior as events
4. Diff expected vs. observed
5. Reduce to a **deterministic minimal reproduction**, recorded as a runnable command
6. Emit a `fix` work item with the repro attached, ready for `circle:fix`

The contract design pays off directly here: an agent that knows how to run and test the app can hunt bugs; one that doesn't, can only guess.

**`circle:skill-forge`** — the requester's *"key to work with the framework."* It authors new skills that conform to the Circle skill contract:

- Thin client over the `circle` CLI — no domain logic in skill prose
- Cannot bypass `circle:preflight`
- Records events via `circle event record`
- Ends by emitting copyable next actions
- Declares which contracts it reads and which it may write

Forged skills are scaffolded into `.claude/skills/circle-<name>/`, linted by `circle skill validate`, and **registered in `.circle/project.toml`** — so a team's custom skills are committed and replicate with the repo, exactly like the contracts. This is what makes Circle a framework rather than a fixed toolset.

**Bootstrapping note**: `circle:review` and `circle:hunt` should themselves be authored *using* `circle:skill-forge`. If forge can't produce them, it isn't ready — and that dogfooding is the cheapest possible validation of the extensibility claim.

### CLI Surface

Clean architecture — each command is one application use case; no domain logic in `cobra` handlers.

**Layering**

- **Domain** (pure, no I/O): `Project`, `KnowledgeBase`, `QualityContract`, `ExecutionContract`, `WorkItem`, `RepoImpact`, `AppSession`, `ClaudeSession`, `Worktree`, `ComposeMap`, `TaskGraph`, `Score`, `Event`
- **Ports** (interfaces): `TrackerPort`, `TaskGraphPort`, `VcsPort`, `WorktreePort`, `ComposePort`, `ProcessPort`, `LlmPort`, `EventStorePort`, `ArtifactRepoPort`, `ClockPort`
- **Adapters**: `gh` CLI, Jira REST, `bd`, local JSONL graph, the `git` CLI shelled out rather than `go-git` — the timeline needs `patch-id`, reflog, and worktree semantics identical to the user's own git — compose-file parser, Anthropic API, filesystem
- **Interfaces**: `cobra` CLI, `net/http` + SSE, hook receiver

**Commands**

| Group | Command | Purpose |
|---|---|---|
| Setup | `circle init` | Scaffold `.circle/`, detect compose + test commands, install skills + hooks |
| | `circle doctor` | Verify `git`, `docker`, `gh`, `bd`, model access |
| | `circle repo add\|list\|remove` | Multi-repo workspace membership |
| Contracts | `circle project validate` | Validate all three contracts at once |
| | `circle knowledge add\|list\|validate` | Knowledge-base registry (docs / definitions / validations) |
| | `circle quality run [gate]\|list` | Execute quality gates; record results as events |
| | `circle compose scan\|map` | Parse compose, classify + mark app containers |
| | `circle workspace edit\|validate` | Session execution contract |
| | `circle goal edit\|validate` | Success definition; reports checkable/total ratio |
| | `circle preflight` | **The gate.** Non-zero exit blocks the session |
| Execution | `circle worktree provision\|release` | **Hook targets, not user commands** (D-23). Invoked by `WorktreeCreate`/`WorktreeRemove` over native worktrees: Compose namespace + port allocation + `.env` override |
| | `circle run --print` | Emit the exact commands Claude should execute |
| Gate | `circle gate check [--skill <name>]` | Hook + skill-injection target. Non-zero exit denies the write or aborts the skill |
| Timeline | `circle timeline open\|close` | Hook target: record the baseline SHA per repo + worktree + branch |
| | `circle timeline sync` | Reconcile `git rev-list` against attribution; resolve rewrites; flag phantom commits |
| | `circle timeline show <item-id>` | The timeline: commit, subject, changed files. `--format md\|json`, `--graph`, `--since` |
| | `circle timeline drift <item-id>` | Files touched outside the approved brief's blast radius |
| Artifacts | `circle intake <path> --type prd\|architecture\|ux\|plan` | Ingest implementation artifacts |
| | `circle addendum add <item-id>` | Append a decision/finding record |
| Sessions | `circle session new\|list\|show\|attach\|close` | App sessions; many Claude sessions attach to one |
| Work | `circle item new --kind epic\|task\|fix` | The mandatory opening question, as a command |
| | `circle item show\|list\|close` | Work-item lifecycle |
| | `circle epic plan\|impact\|tasks\|sync` | Decomposition + repo impact detection |
| Brief | `circle brief generate <item-id>` | Render `brief.md` + `brief.html` from `.circle/` state |
| | `circle brief open\|status` | View a brief; list briefs pending approval |
| | `circle brief approve\|reject <item-id>` | **Records the gate event that unblocks implementation** |
| Review | `circle review run <item-id>` | Five-dimension PR review; writes `review.md` |
| | `circle review report --post` | Publish findings as PR comments via `gh` |
| Hunt | `circle hunt start <session-id>` | Stand up the stack and record expected behavior |
| | `circle hunt repro\|report` | Reduce to a minimal repro; emit a `fix` work item |
| Skills | `circle skill new <name>` | Scaffold a framework-conformant skill (`skill-forge` backend) |
| | `circle skill validate\|list\|sync` | Lint against the skill contract; register in `project.toml` |
| | `circle task ready\|claim\|close` | Mirrors `bd ready` / `bd update --claim` |
| Tracking | `circle track issue create\|sync` | GitHub issue in anchor repo (v1) |
| | `circle track pr open\|sync` | One PR per affected repo, cross-linked |
| Insight | `circle score compute\|show\|explain` | Three indicators + evidence |
| | `circle next` | Suggested actions as copyable Claude commands |
| | `circle status` | Terminal equivalent of the dashboard |
| | `circle usage` | Tokens in/out, approximate cost |
| Runtime | `circle event record` | Hook target — append to the event store |
| | `circle serve` | HTTP + SSE; serves the embedded JS app |
| | `circle export` | Portable snapshot for CI or sharing |

### Scoring Strategy — Unknown, Complexity, Status

Requester asked specifically for a *semi-deterministic* strategy. The governing principle:

> **Go computes the numbers. The LLM only classifies.** The model never emits a score — it selects from fixed enums and cites evidence. All arithmetic is pure Go. This is what buys reproducibility.

**Mechanics**

1. Signal collection is pure Go, versioned (`scoring_v1`), zero LLM involvement.
2. Signals are canonicalized into a **signal bundle** and hashed (SHA-256).
3. LLM calls run at temperature 0 against a strict JSON schema whose output is *classification only* — enum selections plus file/line evidence.
4. `score = f(signals, classifications)`, where `f` is deterministic Go arithmetic.
5. **Cache key = `sha256(scoring_version ‖ signal_bundle)`** ⇒ identical inputs produce byte-identical scores. No drift, no repeat cost.
6. Scores move only when a signal moves, so the app can always show *which* signal moved.
7. Each score carries **confidence** = fraction derived from deterministic signals. Low confidence drives suggested actions that increase determinism.
8. On low confidence, run classification 3× and majority-vote; escalate model tier only then.

**Unknown (0–100, lower is better)** — driven by the *knowledge* contract

| Signal | Source | Determinism |
|---|---|---|
| `definitions` coverage of the work item | Knowledge registry ∩ item scope | Deterministic |
| Unresolved open questions | Unchecked `- [ ]` in artifacts + addenda | Deterministic |
| Goal testability ratio | Parse `goal.md` for executable criteria | Deterministic |
| Contract completeness | Missing quality gates / unmarked app services / dead knowledge paths | Deterministic |
| Undocumented surface | Touched paths absent from `docs` registry | Deterministic |
| Impact confidence | Spread in repo-impact detection | Deterministic |
| Residual unknowns | LLM classifies into `spec` / `technical` / `environmental` + severity enum | Classification only |

**Complexity (0–100)** — driven by the *execution* contract

| Signal | Source | Determinism |
|---|---|---|
| Blast radius | Files/modules in plan + git history of comparable changes | Deterministic |
| Affected repo count | `impact.toml` | Deterministic |
| Cross-service count | `compose.map.toml` | Deterministic |
| Graph shape | Task count, dependency depth, fan-out, blocked edges | Deterministic |
| Coupling | Import-graph centrality of touched modules | Deterministic |
| Historical churn | Commit density + revert/hotfix rate on those paths | Deterministic |
| Test burden | Existing coverage of touched paths (inverse) | Deterministic |
| Risk factors | LLM selects from a **fixed enum** — schema migration, public API change, concurrency, auth/security, data backfill, cross-service contract — each with a fixed Go-side multiplier | Classification only |

The enum is the crux: the model does *recognition* (reliable); the weighting is code (reproducible).

**Status (0–100)** — driven by the *quality* contract, and the most deterministic of the three by design

| Component | Weight | Source |
|---|---|---|
| Quality gates passing | 40% | `circle quality run` exit codes |
| Goal criteria verified | 30% | Executed `goal.md` criteria — pass/total |
| Task closure | 20% | **Tasks whose `[verification]` gate passed after their last commit** (D-35) — not tasks the agent marked done |
| PR + CI health | 10% | `gh` — draft state, checks, reviews |

> With D-35 in place, **all four components are evidence-backed**. Task closure was the last one resting on an assertion.

The LLM contributes **only** where a goal criterion is not machine-checkable, and must cite session events. A project with a complete quality contract and fully checkable goals gets a **100% deterministic status score** — exactly the incentive the framework wants to create.

### User Flow (critical path)

```
/plugin install circle-ai   → skills + enforcement hooks arrive together, no trust dialog (D-21)
circle init                 → detect compose, test commands, and existing .claude/skills/run-*/
circle project validate     → fix the three contracts until they pass
claude --worktree <item-id> → WorktreeCreate hook: worktree + COMPOSE_PROJECT_NAME + ports (D-23)

/circle:start           → epic | task | fix ?
     ⛔ preflight runs as `!`circle gate check`` inside the skill body —
        a non-zero exit ABORTS the invocation; Claude never sees the content
     🔒 arms the PreToolUse write-block hook for the rest of the session
                    ↓
/circle:brief           → brief.md + brief.html: goal, plan, module + sequence
                          diagrams, examples, impact, risks
                    ↓
  ⛔ HUMAN GATE — circle brief approve <item-id> --approver <identity>
     PreToolUse DENIES every Edit/Write/NotebookEdit until this event exists.
     Not a convention. The tool call does not run.
                    ↓
/circle:run             → isolated compose stack, per the execution contract
                          (implement: quality gates run as you go)
/circle:verify          → goal criteria + quality gates → Status score
                          Stop hook exit 2 — the turn cannot end on red
/circle:handoff         → circle timeline sync → commit + file timeline, drift
                          against the approved blast radius, uncommitted tail;
                          addendum + summary committed; worktree released
```

v0.2 adds `/circle:epic-plan` (task graph + repo impact + PR fan-out), `/circle:fix`, `/circle:hunt`, `/circle:review`, and `/circle:intake` to this flow.

The gate sits after planning and before *any* code. `brief.md` becomes the PR body, so the same summary that got human approval is the one reviewers see.

---

## Design References

Rendered, shareable design artifacts. These are **normative for the surfaces they cover** — where a screen and this document disagree, the screen is the newer decision and this document should be corrected.

| # | Artifact | Covers | Status |
|---|---|---|---|
| **A-1** | [Screens](https://claude.ai/code/artifact/080c6119-47b4-4674-b95c-53ae14483feb) · `circle-ai.screens.html` | Seven full-size screens: `circle init`, `circle preflight`, the denied write in Claude Code, **`brief.html`**, `circle timeline show`, `circle status`, and the app | **Current** — the terminal and brief surfaces |
| **A-2** | [Observation app guidelines](https://claude.ai/code/artifact/a269e827-a65f-4c3d-9434-6d9e58f0b1a4) · `circle-ai.screens-app.html` | Navigation model, route table, component vocabulary, and eleven app screens sharing one identical shell — including **S04 · Item · Tasks** | **Current** — normative for the v0.2 app |
| **A-3** | [Flows & task contract](https://claude.ai/code/artifact/5dad1144-5059-4f1f-a0f7-b9c412af36f4) · `circle-ai.flows.html` | Surface authority (D-38), the task contract (D-35–D-37), and five cross-surface sequence flows: cold start, task loop, gate denial, drift, cold resume | **Current** — normative for interaction |
| A-0 | [Mockup set 01](https://claude.ai/code/artifact/6b54f017-6164-4561-a526-5fe6a62e305e) · `circle-ai.mockups.html` | First-iteration annotated requirement sheets | **Superseded by A-1.** Kept for the requirement-extraction record only |

**Three conventions the artifacts establish that this document must not contradict:**

- **Navigation** (A-2) — three levels only: an identical sidebar on every screen, a fixed tab set per object type, and master–detail within a screen. Contextual links never enter the sidebar.
- **Component vocabulary** (A-2) — a closed set of status chips with fixed meanings: green is confirmed by an executed command, amber needs a human, red is a claim that failed verification. Four provenance badges, three button kinds.
- **Surface authority** (A-3, D-38) — Claude Code may do everything except open a gate.

> Artifacts are private by default. Share them from each page's share menu before circulating this document.

---

## Technical Approach

**Feasibility**: **MEDIUM** for v0.1 in one month — high per-component, medium in aggregate because the month is tight.

**Architecture Notes**

- **Single backend.** `circle serve` runs `net/http` with SSE and serves the JS app from the stdlib `embed` package — one binary, no second runtime, nothing extra for teammates to install. Critical for OSS adoption.
- **`project.toml` is the product.** It is the artifact that makes a workflow replicable. `circle init` should *detect* as much of it as possible (compose file, test commands from `package.json`/`Makefile`/`pyproject.toml`) and ask about the rest. Time-to-first-preflight-pass is the adoption metric that matters.
- **Hooks as observation substrate.** `circle init` writes SessionStart / PostToolUse / Stop hooks into `.claude/settings.json`, each shelling to `circle event record`. Format-stable and real-time — chosen over transcript polling.
- **Ports before adapters.** `TrackerPort` and `TaskGraphPort` exist from day one though v0.1 ships only `gh` and `bd`+JSONL. Jira becomes additive.
- **Anchor repo.** `.circle/` lives in one designated repo — ideally whichever owns the compose file, since the execution contract is "how to run the whole system." Siblings declared in `workspace.toml` by remote URL + relative path.
- **Volatile/durable split.** `.circle/runtime/` is gitignored and fully regenerable. Everything else is committed. This is what makes "share via repo" honest.
- **OSS surface.** No absolute paths, no assumed org, no required cloud service. Degrades to local-only when `gh`/`bd`/network are absent.

**Technical Risks**

| Risk | Likelihood | Mitigation |
|---|---|---|
| **The gates are unenforceable as originally specified** | **H** | **D-22.** Skills are context, not control flow. Every gate moves to a hook: `PreToolUse` deny, injected-command abort, skill-frontmatter `hooks:`, `Stop` exit 2. Enforcement is now Phase 2, before the skills |
| **Claude Code's bundled `/run-skill-generator` already commits a launch recipe to the repo** — the core pitch, free | **H** | **D-25.** Interoperate: `circle init` reads it as a detection source. Differentiate on *executable + enforced + quality/knowledge + isolated*, and say so in the README |
| **One month is not enough for the full scope** | **H** | Revised phase plan: single-repo, five skills, Status only, worktrees via hook. Phase 0 validates on paper before any Go; Phase 7 is a hard stop-and-assess gate. **D-40 adds scope back** by pulling the minimal app into Phase 6 — it is the first thing to cut if the month is tight |
| Enterprise kill switches disable enforcement | **M** | `allowManagedHooksOnly` kills project hooks; `disableSkillShellExecution` kills `` !`cmd` `` injection. D-30 requires a specified degraded mode for each, not a best-effort fallback |
| ~~beads moved to Dolt~~ | ~~H~~ → **L** | **Resolved.** JSONL is the only v0.1 store; `bd` deferred to v0.2 (D-4 revised) |
| Worktree + Compose collisions (ports, volumes, networks) | **M** | `COMPOSE_PROJECT_NAME` namespacing + allocated port ranges in `runtime/worktrees.lock`, applied from a `WorktreeCreate` hook over native worktrees (D-23); concurrency test as the Phase 3 exit criterion |
| Hook scripts read the wrong repo inside a worktree | **M** | `${CLAUDE_PROJECT_DIR}` stays at the main checkout; only the payload's `cwd` follows. Every hook script reads `cwd` from stdin — asserted by a test. **Applies equally to timeline baselines**, which are recorded per repo + worktree + branch |
| **Timeline empties itself after a squash-merge** | **M** | D-33: entries are superseded, not deleted; successor resolved by patch-id → reflog → subject+date. Tested against amend, rebase, and squash-merge as a Phase 6 exit criterion |
| Agent claims commits it never made ([#44035](https://github.com/anthropics/claude-code/issues/44035)) | **M** | D-31: git is the truth set, attribution is advisory. A claimed-but-absent commit is surfaced as `phantom` rather than silently trusted — this is a feature, not a mitigation |
| Repo impact detection produces wrong repo sets | **M** | Closed-enum classification + deterministic co-change signals + mandatory human confirmation before PR creation |
| `circle init` auto-detection fails on unfamiliar stacks | **M** | Detection is best-effort and always editable; preflight reports specific gaps rather than failing opaquely |
| Scores feel arbitrary → ignored | **M** | Confidence values + `circle score explain`; deterministic-first arithmetic; property test asserting reproducibility. Deferred to v0.2 anyway |
| Claude Code hook API changes | **M** | Hooks are thin shims; the event schema is ours and versioned |
| Preflight too strict → developers bypass | **M** | Report ratios with actionable fixes rather than binary rejection; `--force` is allowed but recorded as an event |
| **Generated diagrams are wrong** — a plausible-but-false module diagram manufactures confidence at the exact moment a human is trusting the summary | **H** | D-17: structure assembled deterministically from compose map, import graph, and impact set; LLM only labels. Phase 6 exit criterion compares the diagram against the merged diff |
| Brief gate becomes a rubber stamp — approved without reading | **M** | Keep briefs short by construction (one screen per section); measure comprehension, not approval rate; invalidate on plan change so stale approvals can't carry. **The deeper failure is self-approval by the person who prompted the agent** — D-29 records approver identity and measures comprehension on non-authors |
| `skill-forge` produces skills that drift from the contract | **M** | `circle skill validate` runs in CI; the contract is machine-checkable, not prose guidance |
| `circle:hunt` rationalizes expected behavior backwards from what it observed | **M** | Expected behavior must be written and recorded as an event *before* the stack is exercised; the ordering is enforced by the CLI, not by prompt discipline |
| Committed `mermaid.min.js` (~1MB) bloats repos | **L** | One copy per repo, not per brief. Open question flags the pre-rendered-SVG alternative if it proves objectionable |

---

## Implementation Phases

> **Scope warning.** The original 11-phase v0.1 — a Go CLI with 12 domain entities, 10 ports, ~45 commands, 13 skills, deterministic diagram generation, multi-repo impact detection, PR fan-out, and a self-hosting skill forge — is a quarter of work, not a month. Five things were cut to make the month plausible: multi-repo (~2 wks), the `circle worktree` command tree (~1 wk), `skill-forge`/`review`/`hunt` (~1.5 wks), the Unknown and Complexity indicators, and the two-level session model. Rationale in `circle-ai.review.md` §3.2.

**v0.1 — Phases 0–7.** Target one month; expect 5–6 weeks. Single-repo, five skills, Status only.
**v0.2+ — Phases 8–15.** Deferred without loss, since all interaction happens in Claude Code and `circle status` covers the terminal case.

<!--
  STATUS: pending | in-progress | complete
  PARALLEL: phases that can run concurrently
  DEPENDS: phases that must complete first
  PRP: link to generated plan file once created
-->

> **Revised.** Single-repo. Status score only. Five skills. **Enforcement before features**, and **validation before code**. Full rationale and cut analysis in `circle-ai.review.md` §3–4.

| # | Phase | Description | Status | Parallel | Depends | PRP Plan |
|---|---|---|---|---|---|---|
| **0** | **Paper validation** *(no code)* | Hand-written `project.toml` + 3 markdown skills + 1 hook script on two real repos; README problem framing. **Targets: `bb-control` (2 services, clean) and `surebets` (18 services, 494 lines of existing CLAUDE.md)** | **build done · trial pending** | - | - | [`phase-0-paper-validation.plan.md`](./.claude/PRPs/plans/phase-0-paper-validation.plan.md) |
| 1 | Plugin + contracts | `circle-ai` plugin scaffold; `project.toml` schema, knowledge registry, quality gates, compose parse + marking; `circle init\|doctor\|project validate\|quality run\|preflight`. `init` detects Compose, test commands, **and existing `.claude/skills/run-*/`** | **done** | - | 0 | **A-1** |
| 2 | **Enforcement** | `hooks/hooks.json`: `PreToolUse(Edit\|Write)` gate, `Stop` verify, `FileChanged` revalidate; skill-body `` !`circle gate check` ``; `--force` recorded **and consumed** | **done** | with 3 | 1 | - |
| 3 | Compose isolation | `WorktreeCreate`/`WorktreeRemove` hooks: `COMPOSE_PROJECT_NAME`, port allocation, `.env` override, `.worktreeinclude`, `circle run --print` | **done** | with 2 | 1 | - |
| 4 | Core skills (5) + **task contract** | `start`, `brief`, `run`, `verify`, `handoff`. Small bodies (5k-token compaction budget). `context: fork` where expensive. **`circle task create\|validate\|ready\|claim\|close\|show\|list` with required goal + verification; `TaskCreated`/`TaskCompleted` hooks** | **done** | - | 2, 3 | **A-3** |
| 5 | **Brief + human gate** | Deterministic diagram assembly, `brief.md` (PR body) + `brief.html`, mermaid from `${CLAUDE_PLUGIN_ROOT}`, approval event with approver identity, invalidation on plan change | pending | with 6 | 4 | **A-1** |
| 6 | Events + status + **timeline** | Hook-fed event store, `circle event record`, `circle status`, **Status score only**, mirror into the native task list, **`circle timeline *` — baselines, reconciliation, rewrite resolution, drift**. **Plus `circle serve`: one read-only page over SSE, built to A-2's shell (D-40)** | **done** | with 5 | 4 | **A-1** · **A-2** |
| 7 | **v0.1 gate** | Scripted cold-start ×3, brief comprehension ×5 (non-authors), **correction-rate paired trial ×5**, README, license, marketplace submission | pending | - | 5, 6 | - |
| 8 | Task graph *(v0.2)* | `TaskGraphPort`, JSONL store, `bd` optional adapter, `circle epic plan\|tasks` | pending | with 9 | 7 | - |
| 9 | GitHub + multi-repo *(v0.2)* | `TrackerPort`, `gh` adapter, `workspace.toml`, repo impact detection, issue + per-repo PRs | pending | with 8 | 7 | - |
| 10 | `skill-forge` *(v0.2)* | Skill contract (real frontmatter fields), scaffolder, `circle skill validate\|sync` | pending | - | 7 | - |
| 11 | Review + hunt *(v0.2)* | `circle:review` and `circle:hunt` — **both authored via `skill-forge`**; justify against bundled `/code-review` and `/debug` first | pending | - | 9, 10 | - |
| 12 | Scoring engine *(v0.2)* | Unknown + Complexity signals, classification schema, cache, `circle score *` | pending | - | 7 | - |
| 13 | Web app *(v0.2)* | **The full eleven-screen app** — the minimal read-only page already shipped in Phase 6 (D-40). Remaining panels, routes, and object tabs. **Build to A-2: the navigation model and component vocabulary are settled, not open** | pending | - | 12 | **A-2** |
| 14 | Suggestions + config *(v0.2)* | `circle next`, copyable commands, in-app contract + settings editing | pending | - | 13 | - |
| 15 | Jira adapter *(v0.2)* | Second `TrackerPort` implementation | pending | - | 9 | - |

**Phase 0 is the hard gate.** Hand-write the contract and the skills for two real repos and run the cold-start trial with no Go at all. If a hand-written `project.toml` plus three markdown files lets a stranger clone, run, test, and land a correct PR, the thesis holds and the CLI is an optimization worth building. If it doesn't, no amount of Go fixes it — and the `≥70% of goal criteria machine-checkable` assumption, which currently has no evidence behind it, gets tested for the price of a week.

### Phase Details

**Phase 0: Paper validation** — *no code* · [plan](./.claude/PRPs/plans/phase-0-paper-validation.plan.md)
- **Goal**: Test the hypothesis before spending a month on infrastructure that assumes it
- **Scope**: Hand-write `project.toml` for two real repos (no parser, no CLI). Hand-write three skills — `circle-start`, `circle-run`, `circle-verify` — as plain `.claude/skills/` markdown. One `PreToolUse` hook script gating on preflight. Draft the README problem statement
- **Targets, chosen for contrast**: `bb-control` — 2 services both `build:`, a real pytest suite, **no linter, formatter, or coverage tool**, no existing `.claude/` skills. `surebets` — 18 services with 1 infrastructure and 17 app candidates, and **494 lines of existing `CLAUDE.md` + `AGENTS.md`**, which is the competing hypothesis stated in code
- **Success signal**: **2 of 3 developers new to the repo clone, run, test, and land a correct PR** using only the hand-written contract
- **Four measurements this phase exists to produce**: the real machine-checkable ratio (vs. the assumed ≥70%); wall-clock time to hand-author each contract (the case for `circle init`); how many participants the workspace-trust dialog silently disarms (the case for D-21's plugin); and **whether surebets' participants used `CLAUDE.md` or the contract** (the case for the whole knowledge-capture claim)
- **If this fails, stop.** The CLI cannot rescue a thesis that a hand-written contract disproves. **A NO-GO that prevents a month of Go is this phase's highest-value outcome**

**Phase 1: Plugin + contracts**
- **Goal**: Make the workflow replicable — the heart of the product
- **Scope**: `circle-ai` plugin scaffold (`.claude-plugin/plugin.json`, `skills/`, `hooks/`, assets). Entities + ports (traits only), `.circle/` scaffolding. `project.toml` schema; knowledge registry with path validation; quality gate execution + result events; compose parser with app-container heuristic and confirmation. `circle init|doctor|project validate|knowledge validate|quality run|preflight`. **`init` detects Compose, test commands from `package.json`/`Makefile`/`pyproject.toml`, and existing `.claude/skills/run-*/SKILL.md` (D-25)**
- **Success signal**: A repo with a valid `project.toml` lets a stranger run and test the app from the contract alone, with no verbal instruction; `circle doctor` correctly reports missing `gh`/`docker`

**Phase 2: Enforcement**
- **Goal**: Make the gates real rather than advisory — the correction this revision exists for
- **Scope**: `hooks/hooks.json` shipped in the plugin: `PreToolUse` on `Edit|Write|NotebookEdit` → `circle gate check` → `permissionDecision: "deny"`; `Stop` → `circle verify`, exit 2 on red; `FileChanged` on `package.json|Makefile|pyproject.toml|docker-compose.yml` → `circle project validate`. `` !`circle gate check` `` injection in every skill body. `--force` recorded **and consumed** (degrades Status, blocks PR creation). Versioned event schema, append-only store, `circle event record`. Degraded modes for `allowManagedHooksOnly` and `disableSkillShellExecution` (D-30)
- **Success signal**: In a **fresh clone with no `settings.json` edit and no trust dialog**, a `Write` is denied when no approval event exists, and a skill invocation aborts when preflight fails

**Phase 3: Compose isolation**
- **Goal**: Safe, parallel local execution — layered on native worktrees, not rebuilt
- **Scope**: `WorktreeCreate` hook (`COMPOSE_PROJECT_NAME` namespacing, port allocation into `runtime/worktrees.lock`, `.env` override), `WorktreeRemove` hook (`compose down -v`), `.worktreeinclude` guidance, `circle run --print`. **All hook scripts read `cwd` from stdin, never `${CLAUDE_PROJECT_DIR}`**
- **Success signal**: Three `claude --worktree` sessions run isolated Compose stacks simultaneously with zero collisions; exit leaves no orphan volumes or networks; a test asserts correct repo resolution from inside a worktree

**Phase 4: Core skills (5)**
- **Goal**: Circle becomes usable
- **Scope**: `start`, `brief`, `run`, `verify`, `handoff`. `circle:start` arms the write-block hook via skill frontmatter `hooks:`. Bodies kept under the 5k-token compaction budget, with reference material in sibling files. `context: fork` where the work is expensive. Establishes the **skill contract** that v0.2's `skill-forge` will codify — expressed in real frontmatter fields (`disable-model-invocation`, `user-invocable`, `allowed-tools`, `paths`, `model`, `effort`, `context`, `agent`, `hooks`, `metadata`), not a parallel schema
- **Success signal**: A full fix-flow completes end-to-end through skills only

**Phase 5: Brief + human gate**
- **Goal**: No code is written that a human hasn't understood first
- **Scope**: Deterministic diagram assembly (module diagram from compose map + import graph, optionally via the plugin's LSP config; sequence diagram from planned flow); LLM supplies labels, narrative, and examples only. `brief.md` (GitHub-renderable, doubles as PR body) + `brief.html` (mermaid resolved from `${CLAUDE_PLUGIN_ROOT}`, never committed). `circle brief generate|open|status|approve|reject`. **Approval records the approver's git identity (D-29)**; the Phase 2 `PreToolUse` hook consumes it; brief invalidated on plan change
- **Success signal**: 5 reviewers **who did not author the plan** answer scope/blast-radius/risk questions correctly from the brief alone in under 5 minutes, without opening a source artifact. Module diagram matches the eventually-merged diff

**Phase 6: Events + status + change timeline**
- **Goal**: One honest number instead of three soft ones — and an honest record of what actually landed
- **Scope**: Hook-fed event store; `circle status` as the terminal dashboard; **Status score only** — gates 40%, goal criteria 30%, task closure 20%, PR/CI 10%; mirror work items into Claude Code's native task list so the terminal task panel and `TaskCompleted` hooks work. **Change timeline**: `SessionStart` / `WorktreeCreate` baselines, `git rev-list` truth set, `PostToolUse` attribution, `circle timeline sync|show|drift`, rewrite resolution via patch-id → reflog → subject+date, uncommitted tail, markdown table into `brief.md` and `review.md`. **Minimal app**: `circle serve` on `net/http` + SSE, serving a single read-only page from the stdlib `embed` package — A-2's shell, sidebar, and status chips over the event store, and nothing else (D-40)
- **Success signal**: Every number in `circle status` traces to a recorded gate execution. **The timeline reports the same commit set as `git log` after an amend, a rebase, and a squash-merge**, and flags a commit the agent claimed but never made
- **Sizing**: the capture is a day; the **rewrite resolution is where the time goes** (~2–3 days). Do not let it grow — resolve or mark `rewritten`, never guess

**Phase 7: v0.1 gate**
- **Goal**: Prove replication and be installable by strangers
- **Scope**: Scripted cold-start ×3 (fresh container, `claude -p`, assert exit codes); brief comprehension ×5 on non-authors; **correction-rate paired trial ×5**; README with the problem framing; license; install path; `.gitignore` guidance; `claude plugin validate` and community marketplace submission
- **Success signal**: 3/3 cold starts pass unattended, and correction rate is measurably lower with Circle than without. **If this fails, stop — no dashboard fixes it**

**Phase 8: Task graph** *(v0.2)*
- **Goal**: Epic decomposition that survives concurrent work
- **Scope**: `TaskGraphPort`; JSONL store with ready/claim semantics; `bd` as an **optional** adapter; `circle epic plan|tasks|sync`
- **Success signal**: Two developers claim different ready tasks on one epic without collision, with no `bd` installed

**Phase 9: GitHub + multi-repo** *(v0.2)*
- **Goal**: External visibility with correct multi-repo fan-out
- **Scope**: `TrackerPort`; `gh` adapter; `workspace.toml` and the anchor model (D-2); impact detection (deterministic signals + closed-enum classification + human confirmation); one issue in anchor repo, one PR per affected repo, cross-linked; `brief.md` as the PR body
- **Success signal**: An epic touching three repos opens one issue and exactly three linked PRs, each carrying the approved brief, with the repo set confirmed by a human before creation

**Phase 10: `skill-forge`** *(v0.2)*
- **Goal**: Make Circle a framework rather than a fixed toolset
- **Scope**: Formalize the skill contract discovered in Phase 4 (thin CLI client, opens with the gate injection, records events, ends with copyable actions, body under the compaction budget, declares contract reads/writes in frontmatter `metadata`). `circle skill new|validate|list|sync`; scaffolding into the plugin's `skills/`; registration in `project.toml`
- **Success signal**: A developer forges a working project-specific skill in under 15 minutes, and it passes both `circle skill validate` and `claude plugin validate` on first run

**Phase 11: Review + hunt** *(v0.2)*
- **Goal**: Close the quality loop — and dogfood `skill-forge`
- **Scope**: **First, justify each against the bundled `/code-review` and `/debug`** — build only the delta. `circle:review` across transparency / architecture alignment / clarity / simplicity / pipeline, writing `review.md` and posting via `gh`. `circle:hunt` — stand up the isolated stack, state expected behavior from the knowledge registry *before* observing, diff against observed, reduce to a deterministic minimal repro, emit a `fix` item. **Both authored using `skill-forge`**
- **Success signal**: Both skills are produced through forge with no hand-editing of the scaffold. `circle:hunt` finds and reproduces a seeded bug end-to-end

**Phase 12: Scoring engine** *(v0.2)*
- **Goal**: Trustworthy indicators
- **Scope**: Signal collectors, canonical bundle + hashing, classification schema, cache, `circle score compute|show|explain`, confidence values
- **Success signal**: Property test proves identical inputs ⇒ identical scores; `explain` shows every contributing signal

**Phase 13: Web app** *(v0.2)*
- **Goal**: The full observation surface
- **Scope**: The remaining ten of A-2's eleven screens on top of the Phase 6 page (D-40); panels for artifacts, tasks, suggested actions, executed commands, token usage, approximate cost, three indicators
- **Success signal**: App reflects live Claude activity within 2s, with no build step in the repo

**Phase 14: Suggestions + config** *(v0.2)*
- **Goal**: Close the observation→action loop
- **Scope**: `circle next` emitting copyable slash commands; unknown-reduction suggestions from low-confidence scores; in-app editing of `workspace.md` / `goal.md` / `settings.toml`
- **Success signal**: Every suggestion runs unmodified when pasted into Claude Code

**Phase 15: Jira adapter** *(v0.2)*
- **Goal**: Second tracker
- **Scope**: Jira REST adapter behind the existing port; project/issue-type mapping in settings
- **Success signal**: One epic produces both a GitHub issue and a Jira ticket when both are configured

### Parallelism Notes

Phases **2, 3, and 4** are the big win — contracts (pure validation), worktrees (git + process I/O), and session plumbing (hooks + event store) touch disjoint ports and share only the Phase 1 domain model. Running all three concurrently is what makes the timeline plausible at all.

Phases **5 and 6** both depend on Phase 4 but touch disjoint concerns — brief rendering and the event/status store — so they form the second parallel band. One caveat: Phase 5's module diagram wants v0.2's impact detection to be accurate, so **5 should consume impact data behind a trait** and degrade to a single-repo diagram, which is also the v0.1 default under D-26.

Phase **7 is a hard barrier by intent** — no v0.2 work begins until cold-start replication, brief comprehension, and the correction-rate trial are all proven. In v0.2, Phase **10 (`skill-forge`) depends on 4, not on 8 or 9** — the skill contract is discovered by writing skills. Phase **11 is deliberately sequenced after 10** so that review and hunt are forge output rather than hand-written, which is the only real test of the extensibility claim. Phase **15** depends only on Phase 9.

### If the Month Is Fixed

Cut in this order, last-first. Each line is a coherent stopping point, not a partial phase.

| Cut | What you lose | What still works |
|---|---|---|
| 1. Phases 8–15 *(already deferred)* | Epics, multi-repo, forge, review/hunt, scores, dashboard, Jira | The full single-repo implementation loop, via `circle status` in the terminal |
| 2. Phase 6 (events + status) | The Status indicator and the terminal dashboard | Contracts, enforcement, isolation, skills, and the brief gate — `circle quality run` still reports pass/fail directly |
| 3. Phase 3 (Compose isolation) | Parallel work items sharing a machine | Native `claude --worktree` still isolates *files*; only the Compose namespacing is lost |

**Do not cut Phase 2 or Phase 5.** Phase 2 is what makes every other claim in this document true rather than aspirational — without hook enforcement, the contracts are documentation. Phase 5's brief gate is the requester's hardest stated constraint. Cutting either removes the reason a team would adopt this over ad-hoc prompting, or over plan mode.

---

## Decisions Log

| ID | Decision | Choice | Alternatives | Rationale |
|---|---|---|---|---|
| D-1 | Web app runtime | Go CLI serves embedded static JS over `net/http` + SSE | Static-only w/ JSON snapshot; separate Node server | One binary, one dependency, live updates; critical for OSS install simplicity |
| D-2 | Multi-repo model | **Anchor repo holds `.circle/`; one epic ⇒ one issue + one PR per affected repo** — *design stands, but deferred to v0.2 (D-26)* | Per-repo `.circle/` stitched by untracked parent manifest | **Confirmed by requester.** An untracked parent manifest would break git-based sharing |
| D-3 | Session observation | Claude Code hooks → `circle event record` | Transcript polling; skill-only reporting | Real-time, multi-session-safe, and the event schema is ours rather than an internal format we don't control |
| D-4 | Task graph dependency | ~~`bd` when present, native JSONL fallback~~ → **JSONL is the only v0.1 store; `bd` optional in v0.2** | Hard beads requirement; adapter-only | **Revised.** beads went entirely to Dolt in early 2026, dropping SQLite and the git-committed JSONL. A hard dependency now ships an embedded SQL database to every OSS user |
| D-5 | Scoring determinism | Go computes; LLM classifies from fixed enums only | Free-form LLM scoring; pure heuristics | Free-form scoring drifts and can't be cached; pure heuristics can't read intent |
| D-6 | Suggestions output | Copyable Claude slash commands | CLI invocations; app-triggered dispatch | Preserves "all interaction happens in Claude Code" |
| D-7 | Web app write surface | `workspace.md`, `goal.md`, `settings.toml` only | Full artifact editing; read-only | The app authors the two session docs, but artifact editing would pull definition work into an implementation-only tool |
| D-8 | v1 tracker | GitHub via `gh` CLI | Jira first; both; local-only | No token handling, reuses existing auth, fastest path to visibility |
| D-9 | **Contract split** | **Repo-level `project.toml` (knowledge + quality + execution) inherited by session-level `workspace.md` + `goal.md`** | Everything per-session; everything repo-level | Replicability requires stable repo-level contracts; the requirement for two session docs is preserved on top |
| D-10 | App-container marking | `build:` present ⇒ app candidate; heuristic proposes, human confirms | Fully automatic; fully manual | Never silent — a wrong mark corrupts the execution contract |
| D-11 | Compose execution | CLI generates and records config; **Claude executes** | CLI orchestrates compose | Single execution model, consistent with "all interaction in Claude Code" |
| D-12 | Worktree isolation | `COMPOSE_PROJECT_NAME` + allocated port range per work item — **delivered as a `WorktreeCreate` hook, see D-23** | Shared stack; container-per-branch | Only mechanism that makes parallel work items safe without manual port juggling |
| D-13 | Repo impact detection | Deterministic signals + closed-enum LLM classification + human confirmation | Manual selection; free-form LLM | Consistent with D-5; a wrong repo set means wrong PRs, so confirmation is mandatory |
| D-14 | **v0.1 scope** | ~~CLI + contracts + worktrees + skills + brief gate + tracker~~ → **Phases 0–7: paper validation, plugin + contracts, enforcement, Compose isolation, 5 skills, brief gate, status. Single-repo. Tracker, task graph, forge, review/hunt, web app and Unknown/Complexity deferred** | Everything in one month | **Revised.** 11 phases do not fit a month either. See D-26, D-27, and `circle-ai.review.md` §3.2 |
| D-15 | **Brief is a blocking gate** | **Approval recorded as an event; a `PreToolUse` hook denies file writes without it (D-22); invalidated when plan or impact changes** | Advisory summary; post-hoc report | "Humans cannot read hundreds of lines" is a hard constraint. An advisory summary gets skipped; a gate cannot be — but only if a hook, not a skill, enforces it |
| D-16 | Brief rendering | `brief.md` (mermaid fences, GitHub-native, doubles as PR body) **plus** `brief.html` (mermaid resolved from `${CLAUDE_PLUGIN_ROOT}`, see D-28) | Single HTML only; pre-rendered SVG; PDF | GitHub renders mermaid in markdown for free, so the approved summary becomes the reviewed summary at zero cost. HTML covers the rich local read |
| D-17 | Diagram generation | Go assembles graph structure from real signals; LLM supplies labels, narrative, examples | LLM generates whole diagrams | Same rule as D-5. A hallucinated module diagram is worse than none — it manufactures false confidence at the exact moment a human is trusting the summary |
| D-18 | `skill-forge` sequencing | Built in v0.2 (Phase 10), **after** core skills exist; review + hunt are then forged with it | Build forge first; hand-write all skills | The skill contract can only be codified after enough skills exist to reveal it. Forging review + hunt is the cheapest possible proof that forge works |
| D-19 | Forged skill scope | Repo-local, registered in `project.toml`, committed and replicated with the repo | Global user-level skills; a shared registry | Consistent with the replication thesis — a team's custom skills should travel with the codebase, not the developer |
| D-20 | Review findings destination | `review.md` in the item dir + PR comments via `gh`; severity-ranked, only CRITICAL/HIGH block | Blocking on all findings; chat-only output | Keeps review auditable in-repo and non-blocking for cosmetics |
| D-21 | **Distribution** | **Ship as a Claude Code plugin**, not `.claude/` scaffolding | `circle init` writes into `.claude/`; marketplace-only | Only path to `circle:*` namespacing. Critically: **plugin hooks always run when the plugin is enabled, while project `settings.json` hooks require a workspace-trust dialog first** — which would break enforcement at exactly the moment a teammate clones the repo. Also brings `bin/`, `agents/`, `monitors/`, `${CLAUDE_PLUGIN_ROOT}`, and a marketplace review pipeline |
| D-22 | **Gates are hooks, not prose** | `PreToolUse` → `permissionDecision: "deny"`; injected-command abort (`` !`circle gate check` ``); skill-frontmatter `hooks:`; `Stop` exit 2 | Skill instructions that say "refuse" | A skill is context, not control flow — the model may ignore it, and compaction can drop it entirely. These three are the only deterministic mechanisms Claude Code offers |
| D-23 | **Worktree isolation** | **`WorktreeCreate`/`WorktreeRemove` hooks over native worktrees**; no `circle worktree` command tree | Standalone worktree management | Native worktrees (v2.1.49+) already do creation, cleanup, locking, `.worktreeinclude`, and isolation enforcement. Circle adds only Compose namespacing and port allocation. Saves ~1 week and gives `claude --worktree x` as the UX |
| D-24 | Task store | JSONL only for v0.1; **mirror into Claude Code's native task list** for the terminal panel | beads; native task list as the store | The native list has dependencies and file-locked claiming, but lives in `~/.claude/tasks/`, is never uploaded, and is swept by `cleanupPeriodDays` — it cannot satisfy the share-via-git constraint. Projection, not backend |
| D-25 | Bundled-skill interop | `circle init` **reads existing `.claude/skills/run-*/` and `verify/SKILL.md`** as a contract detection source | Ignore them; compete with them | `/run-skill-generator` is the closest competitor and ships with the platform. Reading its recipe converts overlap into the fastest path to a passing preflight |
| D-26 | v0.1 repo scope | **Single-repo.** `workspace.toml` shape reserved, unimplemented | Multi-repo from day one (original D-2) | Multi-repo taxes four phases for the 10% case. D-2's anchor model stands as the v0.2 design; it just isn't v0.1 |
| D-27 | v0.1 indicators | **Status only** | All three indicators | Status is ~100% deterministic and carries the entire quality argument. Unknown and Complexity are the two the risk table itself predicts will be ignored |
| D-28 | Brief assets | **Mermaid ships in the plugin** (`${CLAUDE_PLUGIN_ROOT}`), never committed to the user's repo | Commit `mermaid.min.js` per repo; pre-render SVG | Removes ~1 MB per repo at zero cost. `brief.md` still covers repo-native sharing through GitHub's built-in mermaid rendering |
| D-29 | Approval integrity | Approval records the **approver's git identity**; comprehension measured on **non-authors** | Any approval counts | Self-approval by the person who prompted the agent is a formality, not comprehension. This is what makes the gate's central claim testable |
| D-30 | Degraded modes | **Specified, not just listed**: no network, no `gh`, `allowManagedHooksOnly`, `disableSkillShellExecution` | Best-effort fallbacks | The OSS constraint makes these common, not edge cases. Two of them are enterprise kill switches that disable Circle's *enforcement* mechanisms specifically |
| D-31 | **Timeline capture** | **Baseline SHA + `git rev-list` is the truth set; `PostToolUse` supplies attribution only** | Intercept `Bash(git commit *)`; poll transcripts; ask the agent | Interception misses the human's commits, `--amend`, rebase, and `gh pr merge`, and records intent rather than outcome — which is precisely the documented phantom-commit bug. Reconciliation against git catches what interception invents |
| D-32 | Timeline durability | **`timeline.jsonl` is committed, not `runtime/`** | Regenerate on demand from git | The *commit set* is regenerable from git; the *attribution* — which task, which session, which decision — is not. Losing it would make cold resume depend on narration again |
| D-33 | History rewrites | Recorded entries are **superseded, never deleted**; successor resolved by `git patch-id` → reflog → subject + author-date, else marked `rewritten` | Drop unreachable SHAs; block rewriting | Agents amend constantly and squash-merge erases every session commit from the default branch. A timeline that empties itself on merge is worse than none |
| D-34 | Timeline rendering | Markdown table by default, into `brief.md` (the PR body) and `review.md`; mermaid `gitGraph` behind `--graph` | `gitGraph` by default; HTML only | `gitGraph` renders poorly for long linear histories, which is the common case. The table puts the change record in the PR where reviewers already are |
| D-35 | **Task contract** | **Every task requires a one-sentence `[goal]` and a `[verification]` naming a declared quality gate.** `TaskCreated` exit 2 blocks creation without them; `TaskCompleted` exit 2 blocks closure until that gate passed *after* the task's last commit | Goal at item level only; closure on the agent's word | A task without a goal is a prompt; a task without a verification closes on an assertion. This moves the 20% task-closure component of Status from claimed to proven |
| D-36 | Task vocabulary | **One word: *task*.** Items contain tasks; a standalone task is a task with no parent; hierarchical IDs (`task-118.3`) | *item* / *task* / *unit* used interchangeably | Three names for one concept is why tasks were invisible in the first mockups. The requester found the ambiguity before a user did |
| D-37 | Manual verification | Permitted, but requires `justification` + `reviewer` and lowers the machine-checkable ratio | Forbid it; allow it silently | Forbidding makes Circle unusable on repos with real copy and design work. Allowing it silently would let the ratio be gamed. **If most real tasks land on manual, the 70% target is wrong** — a Phase 0 finding |
| D-38 | Surface authority | **Claude Code may do everything except open a gate.** It generates a brief but cannot approve it; it runs a gate but cannot declare it passed; it closes a task only when a command it ran actually passed | Agent self-approval with an audit trail | The agent never grants itself permission. This single rule resolves most "which surface does X" questions — see `circle-ai.flows.html` |
| D-39 | **Implementation language** | **Go** | Rust (the original stated constraint); Python | A single static binary and trivial cross-compilation — one machine builds every platform, which is what makes shipping the binary inside a plugin's `bin/` directory viable. Per-platform Rust builds were the distribution problem, not the language. beads is Go for the same reasons. **Reverses the original Rust constraint at the owner's explicit direction** |
| D-40 | **Minimal observation app in v0.1** | **The full event + timeline layer plus one read-only page over SSE, in Phase 6** | Full app in v0.2 as originally planned; no app at all | Validates A-2's navigation model and component vocabulary early, at a fraction of the cost of eleven screens. The event layer the page reads from (Phase 6) was already in v0.1, so the marginal cost is the page itself. The full app stays in Phase 13 |

---

## Research Summary

**Market Context**

> Revised after landscape review. beads is *not* the nearest neighbor — it solves task memory and is an optional dependency. The real neighbors are the spec-driven-development (SDD) frameworks that went mainstream in 2026.

| Framework | Model | Overlaps Circle at | Where it leaves an opening |
|---|---|---|---|
| **GitHub Spec Kit** (~111k ★, agent-agnostic) | `constitution → specify → plan → tasks → implement` | The constitution is the quality contract, in prose | Most-cited criticism: **waterfall, weak on brownfield, no story for mid-implementation spec change** — precisely Circle's stated scope |
| **OpenSpec** (~52k ★, most actively maintained OSS SDD) | Strict state machine `propose → apply → archive`, `/opsx:*` plugin namespace | The gated state machine | Definition-side only. And it wins on **three commands** — surface area is a competitive liability |
| **AWS Kiro** | specs + **steering files** + agent hooks | **Steering files are the knowledge contract**; Kiro hooks are the quality contract | IDE-locked, AWS-flavored, prose not executable |
| **Anthropic `spec-driven-development`** (community marketplace) | 5-phase spec-first skill | Prior art inside Circle's own distribution channel | Definition-side, no enforcement |
| **beads** (`bd`) | Dolt-backed graph issue tracker | Task memory | Not a competitor — an optional dependency |

**The positioning finding:** every one of these is *definition-side* — spec → code. Not one encodes **how to run the system, how to prove it works, and where the truth lives**, and none makes those contracts *executable with exit codes*. Circle is the only entrant on the execution side. That, not task tracking and not planning, is the wedge.

**The nearest real competitor is Claude Code itself.** Bundled `/run`, `/verify`, and `/run-skill-generator` already record a project's launch recipe and **commit it to the repo** at `.claude/skills/run-<name>/`, at zero install cost. Circle's remaining differentiation is that its contract is *executable* (exit codes, not prose), *enforced* (writes blocked on failure), *broader* (quality + knowledge, not just execution), and *isolated* (Compose per work item). **Interoperate, don't compete: `circle init` reads existing `run-*` skills as a detection source** (D-25) — a repo that already ran `/run-skill-generator` should reach a passing preflight in under a minute.

**Resolved**: beads migrated **entirely to Dolt in early 2026**, eliminating both SQLite and the git-committed JSONL. A hard `bd` dependency now means shipping an embedded version-controlled SQL database — see D-4 (revised).

Sources: [Spec Kit](https://github.com/github/spec-kit) · [Spec Kit review (Scott Logic)](https://blog.scottlogic.com/2025/11/26/putting-spec-kit-through-its-paces-radical-idea-or-reinvented-waterfall.html) · [OpenSpec](https://github.com/Fission-AI/OpenSpec/blob/main/docs/concepts.md) · [Kiro steering](https://kiro.dev/docs/steering/) · [gastownhall/beads](https://github.com/gastownhall/beads) · [Better Stack: Beads](https://betterstack.com/community/guides/ai/beads-issue-tracker-ai-agents/) · [beads_rust](https://github.com/Dicklesworthstone/beads_rust) · [Spec Growth Engine (arXiv 2606.27045)](https://arxiv.org/pdf/2606.27045)

**Positioning for OSS**

Circle's differentiator is not task tracking and not planning — it is the **executable, enforced project contract**. The one-line pitch: *"Teach Claude how to run, test, and validate your project once. Commit it. The repo enforces it."* The word *enforces* is what separates this from every framework above and from the bundled skills. Validate this framing on the README in **Phase 0**, before any Go is written.

**Technical Context**

Greenfield build; no existing Circle codebase. The requester's working directory contains an unrelated multi-project layout (`docs/`, `plans/`, `db/`, `migrations/`, and two application directories) that is a plausible first real test case for the anchor-repo and repo-impact models.

---

*Generated: 2026-08-29 · Revised: 2026-08-29 after platform validation, landscape review, and three design iterations*
*Status: DRAFT v3 — v0.1 rescoped to 8 phases, single-repo, hook-enforced, tasks contract-bound*

**Document set**

| Document | Holds |
|---|---|
| `circle-ai.prd.md` | This file — the plan of record |
| [`circle-ai.review.md`](./circle-ai.review.md) | Evidence, competitive analysis, and the rationale for every cut |
| [`circle-ai.cli.md`](./circle-ai.cli.md) | CLI surface: global options, exit codes, hook wiring |
| **A-1** [Screens](https://claude.ai/code/artifact/080c6119-47b4-4674-b95c-53ae14483feb) | Terminal and `brief.html` surfaces · `circle-ai.screens.html` |
| **A-2** [App guidelines](https://claude.ai/code/artifact/a269e827-a65f-4c3d-9434-6d9e58f0b1a4) | Navigation model, vocabulary, 11 app screens · `circle-ai.screens-app.html` |
| **A-3** [Flows & task contract](https://claude.ai/code/artifact/5dad1144-5059-4f1f-a0f7-b9c412af36f4) | Surface authority and cross-surface flows · `circle-ai.flows.html` |
