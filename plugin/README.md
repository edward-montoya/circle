# circle — Claude Code plugin

Skills and enforcement hooks for [Circle AI](../README.md).

## Why a plugin and not `.claude/`

Two reasons, both discovered by testing rather than argued from documentation:

1. **`.claude/` is commonly gitignored.** One of the two Phase 0 target repos
   ignores it at `.gitignore:190`, so skills and hooks committed there simply do
   not travel with the repo — which breaks the entire "share the workflow via
   the repository" premise. A plugin's skills and hooks do not live in the
   repo's `.claude/` at all.

2. **Plugin hooks run the moment the plugin is enabled.** Hooks in a project's
   `settings.json` do not run until the workspace-trust dialog is accepted — so
   a teammate cloning the repo gets no enforcement at exactly the moment it
   matters most.

The contract stays in the repo, where it belongs. The machinery ships here.

## Install

```
/plugin install circle
```

The `circle` binary must be on PATH. Build it with `go build -o bin/circle ./cmd/circle`.

## What it registers

| Skill | Does |
|---|---|
| `/circle:start` | Loads the contracts, binds the session to one item or decision. Aborts if the contracts are invalid |
| `/circle:next` | Where this session stands, the next skill, and whether to continue, clear, hand off or compact |
| `/circle:shape` | Interviews you into a settled item, writing `DOMAIN.md` terms and ADRs as it goes |
| `/circle:spec` | Turns the shaped item into a spec whose every test seam names a declared gate |
| `/circle:tasks` | Breaks it into tracer-bullet tasks, each with a goal, a gate and a blast radius |
| `/circle:brief` | Renders the gate a human must approve |
| `/circle:implement` | Builds one task test-first — or every ready task in parallel worktrees |
| `/circle:review` | Three axes: computed trace, spec, standards. Closes the item only when nothing blocks |
| `/circle:handoff` | Reports what landed, from git rather than from memory |
| `/circle:retro` | Turns the session's mistakes into new gates and knowledge |
| `/circle:chart` | Maps an effort too big for one session as decisions, one per session |
| `/circle:hunt` | Builds a command that goes red on a bug, then files the fix as an item |
| `/circle:run` | Brings the stack up from the execution contract |
| `/circle:verify` | Runs the gates and reports what is actually proven |

Three practices run underneath and are swappable per repository under
`[lifecycle]`: `circle:grill` (the interview), `circle:domain` (the domain model),
`circle:tdd` (test-first against declared gates). Write your own with
`circle skill new`; `circle skill validate` holds it to the same contract.
The full flow is in [`docs/LIFECYCLE.md`](../docs/LIFECYCLE.md).

| Hook | Enforces |
|---|---|
| `PreToolUse` | Denies file writes while preflight fails, or outside the session's item's approved plan |
| `TaskCompleted` | Refuses to close a task with no passing gate behind it |
| `SessionStart` | Exports the session id, opens the item's timeline once, refreshes the domain index |
| `PostToolUse` (writes) | Regenerates `.circle/DOMAIN-INDEX.md`; feeds a new, undocumented concept straight back to the agent |
| `PostToolUse` (Bash) | Records tool use as events |
| `PreToolUse` (Skill), `UserPromptSubmit` | Records which skills ran, for `retro` |
| `FileChanged` | Revalidates when a manifest changes |
