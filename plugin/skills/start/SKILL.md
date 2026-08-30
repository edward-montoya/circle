---
description: Opens a Circle session. Loads the project contracts and refuses to proceed if they are invalid. Use at the start of any implementation work.
disable-model-invocation: true
allowed-tools: Read Grep Glob Bash(circle *)
---

# Circle session

The block below is produced at invocation time. **If the contracts are invalid
the command exits 2 and you never see this file at all** — that is the gate, not
a convention you are asked to respect.

```!
circle preflight --context
```

```!
circle task ready
```

## Standing instructions for this session

1. **Do not re-derive how to run or test this project.** Both are in the block
   above. If something is missing there, say so and propose the edit to
   `.circle/project.toml` — knowledge that stays in the transcript dies with the
   session.

2. **Every task needs a goal and a verification.** `circle task create` rejects
   a task without both, and `circle task close` refuses until the named gate has
   passed *since the task's last commit*. Do not try to work around either; they
   are what makes the status score a measurement instead of a claim.

3. **A PreToolUse hook denies file writes while preflight fails.** If a write is
   refused, run `circle preflight --explain` and fix the contract.

4. **You may not open a gate.** You can generate a brief but not approve it, run
   a quality gate but not declare it passed. Approval is the human's.

## Your task

Confirm the contracts, then ask what we are building — or if the user already
said, restate it and propose the task breakdown, each with its goal and the gate
that will prove it.

$ARGUMENTS
