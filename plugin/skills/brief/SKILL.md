---
description: Renders the comprehension gate a human must approve before any code is written. Use after planning tasks and before implementing them.
disable-model-invocation: true
allowed-tools: Read Bash(circle *)
---

# Brief

```!
circle brief generate
```

## Your task

Show the human the brief above and **stop**. Do not begin implementing.

You cannot approve it. That is not a rule you are being asked to respect — the
`PreToolUse` hook denies every file write until an approval event exists, and it
checks on every write rather than once at session start.

While they read, you may answer questions about the approach. You may not touch
the working tree.

## What invalidates an approval

The approval is bound to a plan hash covering every task's goal, verification,
paths and dependencies. **Changing any of those silently voids it** and writes
are blocked again — which is the intended behaviour, not a bug to work around.

If the plan needs to change, say so, change it, regenerate, and ask for a fresh
approval. Do not look for a way to keep writing.

## If a write is denied

Read the reason. There are exactly three:

- **No approved brief** — generate one and wait.
- **The approval is stale** — the plan moved. Regenerate and re-approve.
- **Outside the blast radius** — the path is not one any task declared. Either
  the task's `--paths` were too narrow, or the edit does not belong to this
  plan. Say which; do not widen the radius silently to get unblocked.
