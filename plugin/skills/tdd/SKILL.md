---
description: Test-first development against the project's declared quality gates. Use when implementing a Circle task, fixing a bug test-first, or when the human says red-green or TDD.
allowed-tools: Read Grep Glob Edit Write Bash(circle *)
metadata:
  circle:
    kind: practice
    reads: [quality, spec, task]
    writes: [code, tests]
---

# TDD

```!
circle task show --current
```

The block above is the task you hold: its goal, its gate, its paths, and the
seams its spec agreed. You test **only at those seams**. A seam the spec did not
agree is a question for the human, not a place to put a test.

## Red and green are recorded, not claimed

```
circle quality run <gate> --task <id> --expect red     # passes only if the gate FAILS
circle quality run <gate> --task <id> --expect green   # passes only if the gate passes
```

Each run records an event. The task's history then shows red → green per slice,
and `circle task close` reads it. A slice with no recorded red was not written
test-first, whatever the transcript says.

Run a single test file while iterating if the runner allows it; record against
the task's gate at each red and each green.

## The loop

One **tracer bullet** at a time:

1. Write one test at an agreed seam that states one behaviour the goal needs.
2. `--expect red`. If it passes already, the test is not testing the new
   behaviour — fix the test, not the code.
3. Write only the code that turns it green. Nothing the next test will want.
4. `--expect green`.
5. Next behaviour.

Refactoring is not part of the loop. It belongs to `/circle:review`.

## What a test worth keeping is

It verifies behaviour **through the public interface at the seam** and survives a
rewrite of everything behind it. It reads like a line of the spec. Its expected
value comes from an independent source — a known-good literal, a worked
example, the spec — never recomputed the way the code computes it.

See [ANTI-PATTERNS.md](${CLAUDE_SKILL_DIR}/ANTI-PATTERNS.md) when a test feels wrong and you cannot
say why.

## Done

Every behaviour in the task's goal has a recorded red → green at an agreed seam,
and the full gate is green after the last commit. Then the orchestrator closes
the task — this skill does not.
