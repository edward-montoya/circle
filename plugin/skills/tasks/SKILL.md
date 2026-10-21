---
description: Breaks a specified item into tracer-bullet tasks, each with a goal, a gate, a blast radius and its blockers. Use right after /circle:spec, or on any plan that needs splitting.
disable-model-invocation: true
allowed-tools: Read Grep Glob Bash(circle *)
metadata:
  circle:
    kind: orchestrator
    requires: specified | planned | approved | building | reviewing
    produces: planned
    reads: [spec, quality, domain]
    writes: [tasks]
---

# Tasks

Runs on a freshly specified item, and again whenever the plan must change: a
task that turned out too narrow, or blocking review findings. Adding or
changing a task changes the plan, so the approval goes stale and the human
approves again — that is the point, not a cost.

```!
circle item require --state specified --state planned --state approved --state building --state reviewing
```

```!
circle item show --with spec
```

```!
circle quality list
```

## 1. Draft tracer bullets

Each task is a **tracer bullet**: a narrow slice through every layer it needs,
demoable or verifiable on its own, small enough for one fresh session. Not a
layer. See [SLICING.md](SLICING.md) for the rules and the one exception — a
**wide refactor**, which goes expand → migrate → contract.

A prefactor the spec named goes first.

## 2. Give each task its contract

| Field | Rule |
|---|---|
| **goal** | One sentence. What done means for this task *alone*. Domain terms |
| **gate** | A gate from the list above, and the seam row from the spec it proves |
| **kind** | `unit` · `integration` · `e2e` · `manual` (needs `--justification` and `--reviewer`) |
| **paths** | The blast radius, as **directory globs** (`src/orders/**`), not files. Directories survive the implementation; file names do not |
| **blocked by** | Only the tasks that genuinely gate this one |

If you cannot state the goal in one sentence, the task is too big. If no
declared gate can prove it, the spec's seam table is wrong — go back.

## 3. Quiz the human

Present a numbered list — title, blocked by, what it delivers, gate, paths —
and ask:

- Is the granularity right — too coarse, too fine?
- Is every blocking edge real?
- Should any tasks merge or split?
- Are the paths wide enough to do the work, and narrow enough to mean something?

Iterate until they approve.

## 4. Create

Blockers first, so each task can reference real ids:

```
circle task create --item <id> --title "..." --goal "..." \
  --verify-kind <kind> --verify-gate <gate> --paths "<globs>" \
  [--depends-on <ids>]
```

Tasks land in `.circle/items/<id>/tasks.jsonl`. Write them only through
`circle task`, never by editing the file: the CLI validates the contract and
records the event. Then:

```
circle task validate
```

It exits 2 on any task missing its contract. The item becomes `planned` the
moment it has tasks — the state is derived, not set.

## Done

`circle item show` reports `planned`. Next is `/circle:brief` — the human gate.
This is the phase boundary where the shaping conversation stops being needed:
the brief, the spec and the tasks now carry it.
