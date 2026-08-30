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
| `/circle:start` | Opens a session; aborts if the contracts are invalid |
| `/circle:tasks` | Breaks work into tasks that each carry a goal and a gate |
| `/circle:run` | Brings the stack up from the execution contract |
| `/circle:verify` | Runs the gates and reports what is actually proven |
| `/circle:handoff` | Reports what landed, from git rather than from memory |

| Hook | Enforces |
|---|---|
| `PreToolUse` | Denies file writes while preflight fails |
| `TaskCompleted` | Refuses to close a task with no passing gate behind it |
| `SessionStart` | Records the timeline baseline |
| `PostToolUse` | Records tool use as events |
| `FileChanged` | Revalidates when a manifest changes |
