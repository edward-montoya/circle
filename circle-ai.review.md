# Circle AI PRD — Review & Iteration

*Reviewed: 2026-08-29 · Against: Claude Code docs (code.claude.com, current) and the 2026 SDD framework landscape*

## Verdict

The thesis is right and the wedge is real, but the document is **wrong about the platform in four load-bearing places** and **wrong about who the competition is**. Both are fixable without changing the bet. The schedule is not fixable — 11 phases is a quarter, not a month.

Three sentences:

1. **The gates don't work as specified.** Skills are prompt text; they cannot block. Every "Blocking" row in the skill table is currently a suggestion the model may ignore. Claude Code gives you three real enforcement mechanisms and the PRD uses none of them.
2. **Roughly a third of v0.1 is already shipped by Claude Code** — worktree isolation, a shared task list with dependency-aware claiming, and bundled `/run` + `/verify` + `/run-skill-generator` that record a project's launch recipe into the repo. That last one is Circle's elevator pitch, shipped by Anthropic, at zero install cost.
3. **The competitive framing is wrong.** beads is not the nearest neighbor. Kiro's steering files and Spec Kit's constitution are — and the single most valuable thing in this document (execution-side contracts with exit codes) is a genuine gap that nobody in the landscape occupies.

---

## Part 1 — Platform validation

### 1.1 The gates are not gates (CRITICAL)

The PRD marks `circle:preflight`, `circle:start`, and `circle:brief` as **Blocking**, and says `circle:task-run` "refuses to execute without" an approval event. A skill is markdown injected into context. Claude can decline to follow it, and after auto-compaction it may not even be in context — skills are re-attached with a 5,000-token-per-skill cap under a 25,000-token combined budget, and older skills are dropped entirely when many have been invoked.

Claude Code provides three deterministic mechanisms. Use all three:

| Mechanism | How it enforces | Use for |
|---|---|---|
| **`PreToolUse` hook** on `Edit\|Write\|NotebookEdit` returning `permissionDecision: "deny"` (or exit 2) | Claude Code refuses the tool call outright | **The brief gate.** No approval event ⇒ no file writes. This is the only true implementation block |
| **Injected-command abort** — `` !`circle gate check` `` in a `SKILL.md` body | A non-zero exit **aborts the whole skill invocation**; Claude never sees the content | **Preflight.** Every Circle skill opens with the check. Fail the contract, the skill won't even load |
| **Skill frontmatter `hooks:`** | A skill registers hooks that persist for the rest of the session | `/circle:start` arms the write-block on session bind, so the gate exists without touching `settings.json` |

Two more worth having:

- `Stop` hook, exit 2 → the turn cannot end until `circle verify` passes (prevents "I've finished" with red gates).
- `TaskCreated` / `TaskCompleted` hooks, exit 2 → a task cannot be marked complete while its quality gate fails.

**Consequence for the plan:** the enforcement layer is its own phase and it comes *before* the skills, not after. Rewrite the "Gate?" column as "Enforced by" with the hook named.

### 1.2 `circle:*` requires a plugin — which resolves four open questions at once

Standalone `.claude/skills/foo/` gives `/foo`. The `plugin:skill` namespace exists only for plugins (`my-plugin/skills/review/SKILL.md` → `/my-plugin:review`). The PRD's own naming presupposes a plugin, but the open questions still treat this as undecided. Ship a plugin. What that buys:

| Plugin capability | Replaces / resolves |
|---|---|
| `hooks/hooks.json` — **"Plugin hooks always run when the plugin is enabled"** | Project `.claude/settings.json` hooks need **workspace trust** first. A teammate cloning the repo gets no enforcement until they accept a dialog — which breaks the replication thesis at exactly the moment it matters |
| `${CLAUDE_PLUGIN_ROOT}` / `${CLAUDE_PLUGIN_DATA}` (survives updates) | **Kills the 1 MB committed `mermaid.min.js`.** Ship it in the plugin, resolve at render time. Repo stays clean |
| `bin/` — executables added to Bash's PATH while enabled | Distribution channel for the `circle` binary. *Caveat: not permitted for plugins distributed through claude.ai org settings, and a Rust binary still needs per-platform builds* |
| `agents/`, `.mcp.json`, `.lsp.json`, `settings.json` | `circle:review` as a real subagent; LSP gives a genuine import graph for the module diagram instead of hand-rolled parsing |
| `monitors/monitors.json` — background processes streaming lines to Claude as notifications | A cheaper v0.1 observation surface than the deferred web app |
| Marketplace + `claude plugin validate` + community submission | The OSS distribution answer, with a review pipeline already built |

**Two enterprise kill switches to design around:** `allowManagedHooksOnly` disables project/user/non-force-enabled-plugin hooks, and `disableSkillShellExecution: true` disables `` !`cmd` `` injection. Circle needs a documented degraded mode where the skills tell Claude to run `circle …` via Bash instead.

### 1.3 Worktree isolation is 80% native — cut Phase 3 to a hook

Claude Code shipped native worktrees in v2.1.49. What already exists:

- `claude --worktree <name>` / `-w`, worktrees under `.claude/worktrees/<name>/`, branch `worktree-<name>`
- `EnterWorktree` / `ExitWorktree` tools; `isolation: worktree` on subagent frontmatter
- `.worktreeinclude` — copies gitignored files (`.env`, `.env.local`) into every new worktree automatically
- `worktree.baseRef` (`fresh` | `head`), branch-from-PR via `--worktree "#1234"`
- Automatic cleanup, `git worktree lock` while an agent runs, a stale-lock sweep, and hard isolation enforcement (edits into the main checkout are blocked at the tool layer)
- **`WorktreeCreate` / `WorktreeRemove` hooks that replace the default creation logic entirely**

Circle should not build a parallel `circle worktree create|list|destroy`. It should implement a `WorktreeCreate` hook that adds the one thing Claude Code genuinely does not do: **`COMPOSE_PROJECT_NAME` namespacing, host-port allocation from `port_range`, and the `.env` override**. The UX becomes `claude --worktree feat-x` and the isolated stack is simply there.

Two concrete corrections this surfaces:

- **Worktree base location** (an open question) is answered: `.claude/worktrees/`, gitignored. Not a sibling directory, not `.circle/worktrees/`.
- **A latent bug**: after Claude enters a worktree, `${CLAUDE_PROJECT_DIR}` *stays at the main checkout* and only the hook payload's `cwd` field follows. Every Circle hook script that assumes `CLAUDE_PROJECT_DIR` is the working tree will read and write the wrong repo during exactly the parallel work Circle is built for. Read `cwd` from stdin JSON.

### 1.4 A native task list exists — but it does not satisfy the replication thesis

Claude Code has `TaskCreate` / `TaskGet` / `TaskList` / `TaskUpdate`, three states, **task dependencies with automatic unblocking**, and **file-locked claiming to prevent races** — the exact primitives the PRD says it needs beads for.

It is stored at `~/.claude/tasks/{session-derived-name}/`, is never uploaded, and is swept by `cleanupPeriodDays`. So it is **local, ephemeral, and not shareable via git** — Circle's core constraint rules it out as the store. Correct move: keep the JSONL graph as the durable store and **mirror into the native task list** so the terminal task panel and `TaskCompleted` hooks work. `TaskGraphPort` gains a projection adapter, not a third backend.

### 1.5 beads: the open question is closed, and the answer changes D-4

beads migrated **entirely to Dolt in early 2026, eliminating both SQLite and the git-committed JSONL**, tied to the Gas Town multi-agent system. A hard `bd` dependency now means shipping an embedded version-controlled SQL database to every user of an OSS tool.

**Resolution:** invert D-4. Native JSONL is the default and the only v0.1 backend; `bd` is an optional adapter deferred to v0.2. (Note `Dicklesworthstone/beads_rust` — a Rust port that kept SQLite + JSONL export — as a possible library rather than a subprocess.) This removes the "Resolve before Phase 6" blocker and drops a High risk to Low.

### 1.6 `/run`, `/verify`, and `/run-skill-generator` are the competition (CRITICAL)

Claude Code bundles three skills that work together to launch and verify an app. `/run-skill-generator` "gets your app running from a clean environment, captures what worked (the install commands, the env vars, the launch script), and **commits it as a per-project skill at `.claude/skills/run-<name>/`**. After that, `/run`, `/verify`, and any other agent in the repo follow the recorded recipe instead of rediscovering it." `/verify` records its own recipe to `.claude/skills/verify/SKILL.md`.

That is Circle's one-line pitch — *teach Claude how to run your project once, commit it, everyone gets the same workflow* — already shipped, at zero install cost, by the platform vendor.

The PRD must answer this head-on. Circle's honest remaining differentiation:

| Circle has | `/run-skill-generator` has |
|---|---|
| Contract is **executable with exit codes**, so status is measured, not narrated | Prose recipe a model interprets |
| **Enforced** — writes are blocked when the contract fails | Advisory |
| **Quality + knowledge** contracts, not just execution | Execution only |
| **Compose/worktree isolation** for parallel work items | Single stack |
| Multi-repo | Single repo |

Recommendation: **interoperate, don't compete.** `circle init` should detect and parse existing `.claude/skills/run-*/SKILL.md` and `verify/SKILL.md` as a detection source for `project.toml`, and emit them as the human-readable projection of the contract. A repo that already ran `/run-skill-generator` should reach a passing preflight in under a minute — which is also the fastest path to the "< 15 min time-to-first-preflight-pass" metric.

### 1.7 Plan mode is already a pre-code human gate

Plan mode + `ExitPlanMode` already stops Claude from writing code until a human approves a plan. The brief gate must differentiate explicitly on three axes, or the first reviewer says "that's plan mode with extra steps":

1. **Durable** — approval is a recorded event in the repo, surviving the session; plan approval is ephemeral.
2. **Structured** — deterministically assembled module and sequence diagrams; a plan is free prose.
3. **Reused** — `brief.md` becomes the PR body, so the approved summary is the reviewed summary.

Worth adding as an explicit row in *What We're NOT Building* and a sentence in the brief section.

### 1.8 Change timeline — checked against the platform, and it *is* a gap

Added after the initial review. Given how much of v0.1 turned out to be already shipped (§1.3, §1.6), the first question for "track the session's commits and changed files" was whether Claude Code does it. **It doesn't**, and the near-miss is instructive.

**Checkpointing is not this.** It captures only edits made by Claude's file-editing tools and explicitly does **not** track files modified by bash commands — so `git commit` itself is invisible to it. It also doesn't capture subagent edits (except foreground forked skills, which matters because Circle plans `context: fork`), doesn't see edits from other sessions or the human, caps at 100 checkpoints, deletes after 30 days, is local-only, and the docs state directly that it is *"not a replacement for version control."* It answers "undo my last few prompts," not "what landed."

**And there is a documented reason to want git-truth specifically:** [anthropics/claude-code#44035](https://github.com/anthropics/claude-code/issues/44035) — Claude reporting successful writes and commits that never persisted. A timeline built by asking the agent what it did would inherit that bug. One built from `git rev-list` catches it.

Three design points worth defending (D-31 … D-34 in the PRD):

1. **Boundary, not interception.** A `PostToolUse` hook on `Bash(git commit *)` is the obvious build and it's wrong alone — it misses the human's commits in another terminal, `--amend`, rebase, and `gh pr merge`, and it records *intent* rather than *outcome*. Baseline SHA + `git rev-list` is the truth set; the hook supplies attribution only.
2. **Rewrites are the whole difficulty.** Agents amend constantly, and a squash-merge erases every session commit from the default branch. Without supersede-and-resolve logic, the timeline empties itself at merge — the exact moment it's most wanted. This is ~2–3 days of the estimate; the capture itself is a day.
3. **Baselines are per worktree and branch.** Under the isolation model each work item is on its own branch in its own worktree, so `HEAD` in the main checkout never moves. Same `cwd`-vs-`${CLAUDE_PROJECT_DIR}` trap as §1.3.

**It also closes a hole this review didn't catch the first time.** The PRD's *"Diagram accuracy — module diagrams match the actual touched module set post-merge"* metric had **no instrument**; it was specified as a manual N=5 comparison. The timeline is the instrument, and `circle brief verify` makes it automatic. The same data yields scope-drift detection — files touched outside the approved brief's blast radius — which is the empirical companion to brief invalidation (D-15): the gate records what was approved, the timeline records what happened.

### 1.9 Smaller corrections

- **`circle:review` as a subagent** (an open question) is answered: `context: fork` plus `agent:` in the skill frontmatter, with `background: false` when the findings must land in the same turn.
- **Skill contract must match real frontmatter.** The levers are `disable-model-invocation`, `user-invocable`, `allowed-tools`, `disallowed-tools`, `paths`, `model`, `effort`, `context`, `agent`, `hooks`, `argument-hint`, `arguments`, `metadata`. `circle skill validate` should check these rather than invent a parallel schema. Note `metadata` is a free-form map explicitly intended for third-party tooling — that is where Circle's contract-read/write declarations belong.
- **Keep SKILL.md bodies small** — the 5k/25k compaction budget means a verbose skill is silently truncated. This belongs in the skill contract as a hard limit, not a style note.
- **Overlap with bundled skills**: `/code-review`, `/debug`, `/verify`, `/loop` overlap `circle:review`, `circle:hunt`, `circle:verify`. Rebuilding them is the least defensible use of a fixed month.
- **`Setup` hook** (fires on `--init-only` / `-p --init`) is the right home for CI-mode preflight, and makes `circle export` mostly unnecessary in v0.1.
- **Contract drift is unhandled.** `project.toml` says `pnpm vitest run`; the script gets renamed; nothing notices until someone runs preflight. Add a `FileChanged` hook on `package.json` / `Makefile` / `pyproject.toml` / `docker-compose.yml` → `circle project validate`. Cheap, native, and it is the "drift enforcement" idea from the current literature.

---

## Part 2 — Competitive landscape

The PRD says "the nearest neighbor is beads." That is not right, and it makes the positioning weaker than it should be.

| Framework | Model | Overlaps Circle at | Where it fails (Circle's opening) |
|---|---|---|---|
| **GitHub Spec Kit** (~111k ★, v0.10, agent-agnostic) | `constitution → specify → plan → tasks → implement` | The constitution is Circle's quality contract in prose | Most-cited criticism is that it is **waterfall, weak on brownfield/legacy, and has no story for mid-implementation spec change** — precisely Circle's stated scope |
| **OpenSpec** (~52k ★, most actively maintained OSS SDD) | Strict 3-phase state machine `propose → apply → archive`, `/opsx:*` plugin namespace | The gated state machine | Definition-side. No execution or quality contract. **Three commands vs. Circle's thirteen** |
| **AWS Kiro** | specs + **steering files** + **agent hooks** | **Steering files are the knowledge contract.** Kiro's hooks are the quality contract | IDE-locked, AWS-flavored, prose-not-executable |
| **Anthropic `spec-driven-development`** (community marketplace) | 5-phase spec-first skill | Prior art inside Circle's own distribution channel | Definition-side, no enforcement |
| **BMAD-METHOD** | Agent personas (PM/architect/dev/QA) | Role decomposition | No repo-durable contract |
| **beads** | Dolt-backed graph issue tracker | Task memory only | Not a competitor — an optional dependency |

**The finding that should lead the PRD:** every one of these is *definition-side* — spec → code. Not one of them encodes **how to run the system, how to prove it works, and where the truth lives**, and none makes those contracts *executable*. Circle is the only entrant on the execution side. Lead with it.

Two corollaries:

- **Surface area is a competitive liability.** OpenSpec wins on 3 commands. Circle proposes 13 skills and ~45 CLI commands for v0.1. Adoption cost is the metric that decides OSS outcomes here.
- **Spec Kit's biggest criticism is Circle's stated scope.** "No good guidance for mid-implementation spec changes" is answered exactly by *"a brief is invalidated when the plan or impact set changes, forcing re-approval."* Say so in the README.

Also worth citing: arXiv 2606.04967 (*From Prompt to Process*, a process taxonomy of agent frameworks) and arXiv 2606.27045 (*The Spec Growth Engine: Spec-Anchored, Code-Coupled, Drift-Enforced Architecture*). The second gives Circle's invalidation mechanism a literature anchor.

---

## Part 3 — Plan critique

### 3.1 The validation is scheduled after the build

The Evidence section concedes: *"no evidence yet that non-requester developers experience this as their top pain… Validation method: publish the README's problem statement and measure whether the framing resonates before building the dashboard."* The plan then puts the README and all three cold-start trials in **Phase 11**, after eleven phases of work.

**Fix: a Phase 0 that ships no Rust.** Hand-write `project.toml` for two real repos. Hand-write three skills as plain `.claude/skills/` markdown. Add one `PreToolUse` hook script. Run the cold-start trial. One week.

If a hand-written contract plus three markdown files lets a stranger clone, run, test, and land a PR — the thesis holds and the CLI is an optimization worth building. If it doesn't, no amount of Rust fixes it. This is also the cheapest possible test of the "≥70% of goal criteria machine-checkable" assumption, which currently has no evidence behind it at all.

### 3.2 One month buys about a third of v0.1

A Rust CLI with 12 domain entities, 10 ports, ~45 commands, 13 skills, deterministic diagram generation, multi-repo impact detection, PR fan-out, and a self-hosting skill forge is a quarter of work for one person, not a month. The PRD half-admits this ("expect 5–6 weeks") and then keeps all 11 phases in v0.1.

Cuts, in confidence order:

| Cut | Rationale | Saves |
|---|---|---|
| **Multi-repo (D-2)** — anchor model, `workspace.toml`, impact detection, PR fan-out | Single-repo is the 90% OSS case. This tax is paid across Phases 1, 2, 6, and 8 | ~2 weeks |
| **`circle worktree *` command tree** → a `WorktreeCreate` hook | §1.3. Native worktrees do the rest | ~1 week |
| **Unknown + Complexity scores** | Already deferred, but they are 2 of the 3 named indicators. **Status alone is ~100% deterministic and carries the entire quality argument.** Ship one honest number instead of three soft ones | (already deferred) |
| **`skill-forge`, `review`, `hunt`** | `/code-review` and `/debug` ship with the platform. Forge is a v0.2 extensibility story | ~1.5 weeks |
| **The two-level session model** (`sessions/<app-id>/claude/<claude-id>/`) | Duplicates Claude Code's own session system. Make session a *field on an event*, not a directory tree | ~3 days |

### 3.3 The metrics measure the artifact, not the outcome

"Contract completeness ≥70% machine-checkable" and "skill conformance 100%" are process metrics — they can all pass while the product is useless. The missing metric is the one the hypothesis actually claims:

> **Correction rate**: same task, same repo, with and without Circle. Count human interventions (corrections, re-prompts, manual fixes) to a correct PR. Target: a measurable reduction. N=5 paired trials.

Two more fixes:

- **Cold-start is unfalsifiable as written.** "Zero out-of-band setup questions," N=3, judged by the author, is not a test. Script it: fresh container, `claude -p`, assert `circle preflight` exit 0 and every quality gate green with no human turn.
- **`--force` has no consequence.** Preflight bypass is recorded as an event and then nothing consumes it. It should degrade the Status score and block PR creation, or it is simply a free bypass with paperwork.

### 3.4 The brief gate's real failure mode

"Rubber stamp" is rated Medium, mitigated by short briefs and comprehension measurement. The deeper problem is not brief length — it is that **the approver is the person who prompted the agent**. Self-approval is a formality, not comprehension.

Fix: record the approver's git identity on the approval event, and make the N=5 comprehension trial use people who did not author the plan. Optionally, allow `[gate] require_second_approver = true` in `project.toml` for teams that want it real. This is a small change that makes the gate's central claim testable.

### 3.5 Smaller items

- **The name.** `circle` collides with Circle (USDC/Circle Internet Financial) and reads adjacent to CircleCI. For an OSS project this is a discoverability and trademark problem worth an hour now rather than a rename later.
- **"One month" is still unconfirmed** as constraint-vs-metric. Everything downstream depends on it. Ask.
- **Rust is a stated constraint, so this is a note not a recommendation:** for v0.1 the CLI does schema validation, command execution, and file assembly. None of it needs Rust, and Rust makes plugin distribution harder (per-platform binaries, `bin/` restrictions on org-distributed plugins). If the month is genuinely fixed, the language is the cheapest constraint to relax.
- **Degraded modes need specifying**, not just listing: no network, no `gh`, `allowManagedHooksOnly`, `disableSkillShellExecution`. Each disables a different Circle mechanism. The OSS constraint ("must work on an arbitrary third-party repo") makes these the common case, not the edge case.

---

## Part 4 — Revised plan

**v0.1 = Phases 0–7.** Single-repo. Status score only. Five skills. Enforcement before features.

| # | Phase | Scope | Exit criterion |
|---|---|---|---|
| **0** | **Paper validation** *(no code)* | Hand-written `project.toml` + 3 markdown skills + 1 hook script on two real repos | **2 of 3 strangers clone, run, test, and land a correct PR.** If this fails, stop — the CLI cannot save the thesis |
| 1 | Plugin + contracts | `circle-ai` plugin scaffold; binary: `init`, `doctor`, `project validate`, `knowledge validate`, `quality run`, `preflight`. `init` detects Compose, test commands, **and existing `.claude/skills/run-*/`** | A stranger runs and tests the app from `project.toml` alone |
| 2 | **Enforcement** | `hooks/hooks.json`: `PreToolUse(Edit\|Write)` → gate check; `Stop` → verify; `FileChanged(package.json\|docker-compose.yml\|Makefile)` → revalidate. Skill-body `` !`circle gate check` ``. `--force` recorded *and consumed* | Writes are denied with no approval event, in a fresh clone, with no `settings.json` edit and no trust dialog |
| 3 | Compose isolation | `WorktreeCreate`/`WorktreeRemove` hooks: `COMPOSE_PROJECT_NAME`, port allocation, `.env` override, `.worktreeinclude` | Three `claude --worktree` sessions run isolated stacks, zero collisions, no orphan volumes |
| 4 | Core skills (5) | `start`, `brief`, `run`, `verify`, `handoff`. Small bodies. `context: fork` where expensive | A full fix-flow completes through skills only |
| 5 | Brief + human gate | Deterministic diagrams (LSP/compose/task-graph derived, LLM labels only); `brief.md` as PR body; mermaid from `${CLAUDE_PLUGIN_ROOT}`; approval carries approver identity; invalidation on plan change | 5 reviewers *who did not author the plan* answer scope/blast-radius/risk correctly from the brief in <5 min |
| 6 | Events + status + timeline | Hook-fed event store; `circle status`; **Status score only**; mirror into the native task list; **change timeline** (§1.8) | Every status number traces to an executed gate; timeline matches `git log` across amend, rebase, and squash-merge |
| 7 | v0.1 gate | Scripted cold-start ×3; comprehension ×5; **correction-rate paired trial ×5**; README, license, marketplace submission | 3/3 cold starts pass. Correction rate measurably lower with Circle |

**v0.2:** task graph (JSONL, `bd` optional) · GitHub + multi-repo impact + PR fan-out · `skill-forge` · `review` + `hunt` · Unknown/Complexity · web app · Jira.

### New decisions

| ID | Decision | Rationale |
|---|---|---|
| D-21 | **Ship as a Claude Code plugin**, not `.claude/` scaffolding | Only path to `circle:*` namespacing; plugin hooks run without a workspace-trust dialog; brings `bin/`, `agents/`, `monitors/`, `${CLAUDE_PLUGIN_ROOT}`, marketplace distribution |
| D-22 | **Gates are hooks, not prose.** `PreToolUse` deny + injected-command abort + skill-frontmatter hooks | Skills cannot block, and compaction can drop them entirely |
| D-23 | **Compose isolation via `WorktreeCreate` hook**; no `circle worktree` command tree | Native worktrees do creation, cleanup, locking, `.worktreeinclude`, and isolation enforcement. Circle adds only Compose namespacing and ports |
| D-24 | **JSONL is the only v0.1 task store**; `bd` optional in v0.2 | beads is Dolt-only since early 2026 — too heavy a dependency for OSS. Mirror into the native task list for the terminal panel |
| D-25 | **`circle init` consumes `/run-skill-generator` output** as a detection source | The bundled skill is the closest competitor; reading its recipe converts overlap into the fastest path to a passing preflight |
| D-26 | **Single-repo for v0.1.** `workspace.toml` shape reserved, unimplemented | Multi-repo taxes four phases for the 10% case |
| D-27 | **Status is the only v0.1 indicator** | It is ~100% deterministic and carries the whole quality argument. Unknown and Complexity are the two most likely to be ignored |
| D-28 | **Mermaid ships in the plugin**, never committed to the user's repo | `${CLAUDE_PLUGIN_ROOT}` exists; `brief.md` covers repo-native sharing via GitHub's mermaid rendering |
| D-29 | **Approval records approver identity**; comprehension is measured on non-authors | Self-approval is not a comprehension gate |
| D-30 | **Degraded modes are specified, not just listed** — no network, no `gh`, `allowManagedHooksOnly`, `disableSkillShellExecution` | The OSS constraint makes these common, not edge cases |
| D-31 | **Timeline capture is boundary-based**: baseline SHA + `git rev-list` is the truth set; `PostToolUse` supplies attribution only | Interception misses the human's commits, `--amend`, rebase, and `gh pr merge`, and records intent rather than outcome — the documented phantom-commit bug |
| D-32 | `timeline.jsonl` is **committed, not `runtime/`** | The commit set is regenerable from git; the attribution is not |
| D-33 | Rewritten commits are **superseded, never deleted**; successor resolved by patch-id → reflog → subject+date | A timeline that empties itself on squash-merge is worse than none |
| D-34 | Timeline renders as a **markdown table into `brief.md`** (the PR body); mermaid `gitGraph` behind `--graph` | `gitGraph` renders poorly for long linear histories, which is the common case |

### Open questions — status

**Resolved by this review:** skills distribution (→ plugin, D-21) · beads backend (→ Dolt, avoid, D-24) · worktree base location (→ `.claude/worktrees/`, gitignored) · mermaid committing (→ plugin-shipped, D-28) · `circle:review` as subagent (→ `context: fork` + `agent:`).

**Still open, now sharper:**
- Is "one month" a constraint or the metric? *(everything downstream depends on it)*
- Name collision with Circle / CircleCI — rename now or accept it?
- What consumes a `--force` preflight bypass? *(currently nothing)*
- Is brief approval revocable mid-implementation, and what happens to in-flight work?
- Token/cost source: hook-captured vs. transcript reconciliation
- OSS license; binary distribution (`cargo install` / releases / plugin `bin/`)

---

## Design artifacts produced after this review

| # | Artifact | Covers |
|---|---|---|
| **A-1** | [Screens](https://claude.ai/code/artifact/080c6119-47b4-4674-b95c-53ae14483feb) | Terminal surfaces and `brief.html`, at full size |
| **A-2** | [App guidelines](https://claude.ai/code/artifact/a269e827-a65f-4c3d-9434-6d9e58f0b1a4) | Navigation model, component vocabulary, 11 app screens |
| **A-3** | [Flows & task contract](https://claude.ai/code/artifact/5dad1144-5059-4f1f-a0f7-b9c412af36f4) | Surface authority, the per-task goal + verification contract, five cross-surface flows |

Two findings emerged from drawing them that this review had missed: the *diagram accuracy* metric had no instrument (fixed by the change timeline), and **task closure was the last Status component resting on an assertion** rather than on an executed command (fixed by D-35).

## Sources

Claude Code documentation: [Hooks](https://code.claude.com/docs/en/hooks) · [Skills](https://code.claude.com/docs/en/skills) · [Plugins](https://code.claude.com/docs/en/plugins) · [Worktrees](https://code.claude.com/docs/en/worktrees) · [Agent teams](https://code.claude.com/docs/en/agent-teams)

Landscape: [GitHub Spec Kit](https://github.com/github/spec-kit) · [Spec Kit review, Scott Logic](https://blog.scottlogic.com/2025/11/26/putting-spec-kit-through-its-paces-radical-idea-or-reinvented-waterfall.html) · [OpenSpec concepts](https://github.com/Fission-AI/OpenSpec/blob/main/docs/concepts.md) · [Kiro steering](https://kiro.dev/docs/steering/) · [Anthropic community SDD plugin](https://claudeskills.info/plugins/anthropics/claude-plugins-community/spec-driven-development/) · [beads](https://github.com/gastownhall/beads) · [Beads guide, Better Stack](https://betterstack.com/community/guides/ai/beads-issue-tracker-ai-agents/) · [beads_rust](https://github.com/Dicklesworthstone/beads_rust) · [Best SDD tools 2026](https://www.augmentcode.com/tools/best-spec-driven-development-tools) · [From Prompt to Process (arXiv 2606.04967)](https://arxiv.org/pdf/2606.04967) · [The Spec Growth Engine (arXiv 2606.27045)](https://arxiv.org/pdf/2606.27045)
