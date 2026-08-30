---
description: Breaks work into tasks that each carry a goal and a verification command. Use when planning an item before implementing it.
disable-model-invocation: true
allowed-tools: Read Grep Bash(circle *)
---

# Decompose into tasks

```!
circle quality list
```

## The contract every task must satisfy

Each task needs, and will be rejected without:

- **`--goal`** — one sentence. What does done mean for this task *alone*?
- **`--verify-kind`** — `unit`, `integration`, `e2e`, or `manual`.
- **`--verify-gate`** — a gate from the list above. A gate that is not declared
  can never run, so a task pointing at one could never close honestly.

`manual` is permitted but never free: it additionally requires
`--justification` and `--reviewer`, and it lowers the machine-checkable ratio.
Use it for things a command genuinely cannot judge — wording, visual design —
and not as an escape from writing a test.

## Your task

Propose the breakdown, then create each task with `circle task create`. Order
them with `--depends-on` so `circle task ready` returns only what is claimable.

If you cannot state a task's goal in one sentence, the task is too big. If you
cannot name a gate that would prove it, either the quality contract is missing
something or the task is not really testable — say which.

$ARGUMENTS
