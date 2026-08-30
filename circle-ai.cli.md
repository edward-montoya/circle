# Circle AI — CLI Surface (v0.1)

*Companion to [`circle-ai.prd.md`](./circle-ai.prd.md) · everything here is a proposal*

*Rendered surfaces: **[A-1 Screens](https://claude.ai/code/artifact/080c6119-47b4-4674-b95c-53ae14483feb)** shows `init`, `preflight`, `timeline show`, and `status` as they actually print. **[A-3 Flows](https://claude.ai/code/artifact/5dad1144-5059-4f1f-a0f7-b9c412af36f4)** shows which of these commands runs on which surface.*

The CLI is the whole product. Skills are thin clients over it, hooks are thin clients over it, and the web app (v0.2) is a thin client over it. Two consequences shape every decision below:

1. **Every command is machine-callable first.** Each one is invoked by a hook or injected into a skill body before a human ever types it. `--format json` is a stable contract, not a convenience.
2. **Exit codes are the API.** Claude Code's `PreToolUse` treats exit 2 as *deny*, and a non-zero exit from an injected `` !`command` `` **aborts the whole skill invocation**. The exit code table below is what makes the gates real rather than advisory.

---

## Global options

Available on every command.

| Option | Default | Notes |
|---|---|---|
| `--repo <path>` | git toplevel of `cwd` | **Resolved from `cwd`, never `CLAUDE_PROJECT_DIR`** — inside a worktree those differ |
| `--circle-dir <path>` | `<repo>/.circle` | Escape hatch for tests and CI |
| `--format <human\|json\|md>` | `human` (`json` when not a TTY) | `json` is versioned and stable; `md` is what gets embedded in `brief.md` and PR bodies |
| `--quiet` / `-q` | off | Exit code only. What hooks use |
| `--verbose` / `-v` | off | `-vv` for trace |
| `--no-color` | auto | Also honors `NO_COLOR` and non-TTY |
| `--yes` / `-y` | off | Assume yes for prompts. **Never bypasses a gate** — only `--force` does, and it is recorded |
| `--dry-run` | off | Print the plan, write nothing. Every mutating command supports it |
| `--no-llm` | off | Refuse classification calls; deterministic signals only. Also `CIRCLE_NO_LLM=1` |
| `--config <path>` | `.circle/settings.toml` | |
| `--version` / `-V`, `--help` / `-h` | | |

**Environment**: `CIRCLE_DIR`, `CIRCLE_FORMAT`, `CIRCLE_NO_LLM`, `CIRCLE_LOG`, `NO_COLOR`.

## Exit codes

Richer than a hook needs, so one binary serves both humans and hooks. The hook wrapper collapses them.

| Code | Meaning | Hook mapping |
|---|---|---|
| `0` | Success / gate passed | allow |
| `1` | Generic error | non-blocking error |
| `2` | **Gate failed** — contract invalid, approval missing, preflight not passed | **deny / abort skill** |
| `3` | Precondition missing — no `.circle/`, not a git repo, no item bound | deny, with a fix hint |
| `4` | External tool missing or failed — `git`, `docker`, `gh` | degrade per D-30 |
| `5` | Quality gate failed — a command in `[quality]` exited non-zero | `Stop` hook → 2 |
| `6` | Declined by the user | allow, no-op |
| `7` | Drift detected — files touched outside the approved blast radius | warn, or deny under `[gate] strict_drift` |

> Only `2`, `3`, and `5` block. `4` and `7` are policy-configurable, because an OSS tool that hard-fails on a missing `gh` is unusable on half the repos it lands in.

---

## Setup

```
circle init [--force] [--compose <file>] [--from-run-skill <path>]
            [--no-hooks] [--detect-only] [--non-interactive]
```
Scaffolds `.circle/`, detects the execution and quality contracts, registers knowledge paths.

| Option | Effect |
|---|---|
| `--from-run-skill <path>` | Seed the execution contract from an existing `.claude/skills/run-*/SKILL.md` (D-25). **Auto-detected when present** — this is the fastest path to a passing preflight |
| `--detect-only` | Print what it would write. Use before committing to the contract |
| `--no-hooks` | Skip hook wiring — for repos where enforcement comes from the plugin instead |
| `--non-interactive` | Fail rather than prompt. CI and `Setup`-hook mode |

```
circle doctor [--fix]
```
Verifies `git`, `docker`, `gh`, plugin installation, hook registration, and model access. `--fix` repairs hook wiring only; it never touches contracts.

---

## Contracts

```
circle project validate [--strict] [--explain]
circle knowledge add <path> --as <docs|definitions|validations>
circle knowledge list [--unresolved]
circle knowledge validate
circle compose scan [--file <f>]
circle compose map [--mark <service>=<app|infra>] [--confirm]
circle goal validate [--item <id>] [--ratio]
circle quality list
circle quality run [<gate>...] [--all] [--changed] [--fail-fast] [--timeout <s>] [--no-record]
circle preflight [--explain] [--force --reason <text>]
```

| Option | Effect |
|---|---|
| `quality run --changed` | Only gates whose declared paths intersect the working tree. The difference between a 4-second and a 4-minute inner loop |
| `quality run --no-record` | Don't emit result events. For local scratch runs that shouldn't move the Status score |
| `goal validate --ratio` | Prints `machine-checkable / total`. The number behind the "≥70%" metric |
| `preflight --explain` | Every check, its source, and the exact command that fixes it. **Actionable ratios, not binary rejection** |
| `preflight --force --reason <text>` | Bypass. `--reason` is **mandatory**, the bypass is recorded as an event, and it degrades Status and blocks PR creation. A bypass with consequences, not a free pass |

---

## Gate — the hook target

```
circle gate check [--skill <name>] [--tool <name>] [--path <p>] [--item <id>]
circle gate status [--item <id>]
```

The single most-called command in the system. Invoked two ways:

- **`PreToolUse` hook** on `Edit|Write|NotebookEdit` → exit 2 emits `permissionDecision: "deny"` with the reason on stderr. The tool call does not run.
- **Injected into every skill body** as `` !`circle gate check --skill circle:brief` `` → a non-zero exit aborts the invocation and Claude never sees the skill content.

`--path` lets the write-block scope denials to the approved blast radius rather than the whole tree, so a brief approved for `src/api/**` doesn't silently authorize edits to `infra/`.

---

## Execution

```
circle worktree provision --item <id> [--ports <lo-hi>] [--project-name <n>]
circle worktree release --item <id> [--keep-volumes]
circle run --print [--item <id>] [--service <s>] [--profile <p>]
```

`provision` and `release` are **hook targets, not user commands** (D-23) — wired to `WorktreeCreate` / `WorktreeRemove`. The user types `claude --worktree task-118` and the Compose namespace, port allocation, and `.env` override happen underneath.

`run --print` emits the exact commands Claude should execute (D-11). Circle never runs the app itself.

---

## Work items

```
circle item new --kind <epic|task|fix> [--title <t>] [--from-artifact <path>]
circle item list [--kind <k>] [--state <open|blocked|closed>]
circle item show <id>
circle item current
circle item close <id> [--reason <text>]
circle addendum add <item-id> [--decision|--finding] [--text <t>|--file <p>]
```

`item current` resolves the item bound to this session and worktree — used by nearly every hook, so it must be fast and never prompt.

---

## Tasks — every one carries a goal and a test

```
circle task create <parent> --title <t> --goal <statement>
                            --verify-kind <unit|integration|e2e|manual>
                            --verify-gate <gate> [--verify-command <cmd>]
                            [--justification <t> --reviewer <id>]   # manual only
                            [--depends-on <id>...]
circle task validate [<id>]
circle task ready
circle task claim <id>
circle task close <id>
circle task show <id>
circle task list [--state <s>] [--unverified]
```

| Command | Effect |
|---|---|
| `create` | **Rejects** a missing goal or verification, or a `--verify-gate` not declared in `[quality]`. `TaskCreated` hook target — exit 2 prevents creation |
| `close` | **Exit 5** unless an event shows the verification gate passed *after* the task's last commit. `TaskCompleted` hook target — exit 2 prevents closure |
| `ready` | Claimable tasks — dependencies met. Prints the goal and verification alongside the title, so a cold-resuming agent knows what "done" means without reading the brief |
| `claim` | File-locked; safe across concurrent sessions and worktrees |
| `list --unverified` | Tasks closed without a passing gate. **Should always be empty** — a non-empty result means the hook was bypassed |

`--verify-kind manual` additionally demands `--justification` and `--reviewer`, and lowers the ratio reported by `circle goal validate`. Manual is permitted but never free (D-37).

## Brief — the human gate

```
circle brief generate <item-id> [--refresh] [--no-llm] [--sections <list>]
circle brief open <item-id> [--format <md|html>]
circle brief status [--pending]
circle brief approve <item-id> [--approver <identity>] [--note <t>] [--allow-self]
circle brief reject <item-id> --reason <text>
circle brief verify <item-id>
```

| Option | Effect |
|---|---|
| `generate --no-llm` | Diagrams and structure only, no labels or narrative. Proves the structure is deterministic (D-17) |
| `approve --approver` | Defaults to `git config user.email`. Recorded on the event (D-29) |
| `approve --allow-self` | Required when the approver authored the plan. **Explicit, and recorded** — self-approval is visible rather than silent |
| `verify` | Declared blast radius vs. the change timeline's actual file set. The instrument for the diagram-accuracy metric |

---

## Change timeline

```
circle timeline open --item <id>
circle timeline close --item <id>
circle timeline sync [--item <id>] [--no-resolve-rewrites]
circle timeline show [<item-id>] [--format <human|md|json>] [--graph]
                     [--since <ref|duration>] [--no-files] [--include-uncommitted]
                     [--author <a>] [--state <live|rewritten|phantom|all>]
circle timeline drift <item-id> [--fail-on-drift]
```

`open` / `close` are hook targets (`SessionStart`, `WorktreeCreate`) that record the baseline SHA per repo + worktree + branch.

| Option | Effect |
|---|---|
| `sync --no-resolve-rewrites` | Skip patch-id/reflog resolution. Fast path for the inner loop; the full resolve runs at handoff |
| `show --no-files` | Commits only — the one-screen view |
| `show --graph` | Mermaid `gitGraph` instead of the table. Not the default: `gitGraph` renders poorly for long linear histories |
| `show --state phantom` | Only commits the agent claimed that git doesn't have |
| `drift --fail-on-drift` | Exit 7. For CI and the `Stop` hook |

---

## Insight

```
circle status [--item <id>] [--watch]
circle score explain [--item <id>]
circle usage [--since <duration>]
circle event record [--stdin]
```

`status` is the terminal dashboard and the reason the web app is deferrable. `score explain` shows every contributing signal — in v0.1 that is Status only (D-27).

`event record --stdin` reads a hook payload from stdin. It is the only command that should ever be hot-path fast.

---

## Hook wiring

What `circle init` writes, or what the plugin ships in `hooks/hooks.json`. This table *is* the enforcement model.

| Event | Matcher | Command | Blocking |
|---|---|---|---|
| `SessionStart` | — | `circle timeline open --item "$(circle item current)"` | no |
| `PreToolUse` | `Edit\|Write\|NotebookEdit` | `circle gate check --tool "$TOOL" --path "$PATH"` | **exit 2 → deny** |
| `PostToolUse` | `Bash` | `circle event record --stdin` | no |
| `Stop` | — | `circle verify --quiet` | **exit 2 on red** |
| `FileChanged` | `package.json\|Makefile\|pyproject.toml\|docker-compose.yml` | `circle project validate --quiet` | no |
| `WorktreeCreate` | — | `circle worktree provision` | **non-zero aborts creation** |
| `WorktreeRemove` | — | `circle worktree release` | no |
| `TaskCreated` | — | `circle task validate --stdin` | **exit 2 → prevent creation** — no goal, no verification, or an undeclared gate |
| `TaskCompleted` | — | `circle task close --verify-only --stdin` | **exit 2 → prevent closure** — the verification gate has not passed since the task's last commit |

Every script reads `cwd` from the hook's stdin payload. None reads `${CLAUDE_PROJECT_DIR}`.

---

## Deferred to v0.2

`circle epic plan|impact|sync` · `circle track issue|pr` · `circle review run|report` · `circle hunt start|repro|report` · `circle skill new|validate|sync` · `circle repo add|list|remove` · `circle intake` · `circle next` · `circle serve` · `circle export`

---

## Open for your call

1. **`circle` vs. `crc` as the typed binary.** Every hook invocation and every skill injection types it. Also see the name-collision question in the PRD.
2. **Is `--force` the right escape hatch at all?** It currently requires a reason, degrades Status, and blocks PRs. The stricter alternative is no bypass — preflight failures are always fixable, and a recorded bypass may just be a socially acceptable way to skip the product.
3. **Should `quality run` default to `--changed`?** Faster inner loop, but a Status score computed from a partial run needs to say so.
4. **Does `timeline show` default to including the uncommitted tail?** It's the most useful default and the least honest one — uncommitted work isn't a change yet.
