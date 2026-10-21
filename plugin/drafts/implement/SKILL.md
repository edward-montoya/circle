---
description: Builds an approved item task by task, test-first. One task in this session, or every ready task in parallel worktrees. Use after the brief is approved.
disable-model-invocation: true
allowed-tools: Read Grep Glob Edit Write Bash(circle *) Bash(git *) Agent Skill
argument-hint: "[task-id | --all]"
metadata:
  circle:
    kind: orchestrator
    requires: approved | building
    produces: reviewing
    reads: [item, spec, tasks, quality]
    writes: [code, tests, tasks]
    calls: [lifecycle.tdd]
---

# Implement

```!
circle item require --state approved --state building
```

```!
circle task ready --item "$(circle session current)"
```

The block above is the **ready set**: open tasks whose blockers are all closed.
The tasks are not a list of steps; they are a graph, and the ready set moves
as tasks close.

## One task — `/circle:implement <task-id>`, or no argument

The default. Take the named task, or the first one in the ready set.

1. `circle task claim <id>`. The write hook now allows that task's paths and no
   others.
2. State the task's title and goal back in one line.
3. Call the Skill tool for the `tdd` slot (default `circle:tdd`) and build it,
   one recorded red → green at a time.
4. Run the task's full gate, commit, then `circle task close <id>`. It refuses
   unless the gate passed **after** your last commit.
5. **Phase boundary.** The next task is self-contained, so this context is
   disposable: suggest `/clear`, then `/circle:implement`.

A write denied as *outside the blast radius* means the task's paths were wrong
or the edit belongs to another task. Say which. Do not widen paths to get
unblocked — that changes the plan and voids the approval, by design.

## The whole item — `/circle:implement --all`

For when you would rather orchestrate than drive. Every task lands on one
**integration branch**.

1. Create `circle/<item-id>` from the current branch.
2. Optionally, send one **explorer subagent** to read the area once and save
   notes to `.circle/runtime/explore/<item-id>.md`. Implementers read the notes
   instead of re-exploring.
3. For every task in the ready set, start an **implementer subagent** in its own
   worktree named after the task (`isolation: worktree`), in the background.
   The `WorktreeCreate` hook binds it to the item and isolates its Compose stack.
   Its prompt is **pointers, not content**: the task id, the spec path, the
   explore notes. Each implementer claims its task, rebases onto the integration
   branch, runs the `tdd` practice, closes its task through `circle task close`,
   and merges the integration branch tip into its own branch before reporting.
4. As each implementer finishes, merge its branch into the integration branch.
   Re-run `circle task ready`; start implementers for anything newly unblocked.
5. When the ready set is empty and every task is closed, the item moves to
   `reviewing`. Clean up the worktrees.

The enforcement is what makes this safe to run unattended. Each implementer is
confined to its task's paths by the write hook and cannot close its task without
a passing gate — the orchestrator never has to trust a subagent's report.

## Done

`circle item show` reports `reviewing`. Next: `/circle:review`, in a fresh
context — the reviewer should read the diff, not inherit the builder's
reasoning.
