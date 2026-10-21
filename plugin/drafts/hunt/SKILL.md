---
description: Diagnoses a bug by first building one command that goes red on it, from the execution contract. Ends with a fix item carrying the reproduction. Use when something is broken, flaky, or slow.
disable-model-invocation: true
allowed-tools: Read Grep Glob Edit Write Bash(circle *) Bash(docker compose *) Bash(curl *) Bash(git *) Agent
argument-hint: "<what is broken>"
metadata:
  circle:
    kind: orchestrator
    produces: specified
    reads: [contracts, knowledge, quality]
    writes: [item, repro]
---

# Hunt

```!
circle preflight --context
```

An agent that knows how to run and test the system can hunt. One that does not
can only guess. The block above is how this one runs and tests.

**Redact every secret** in every command and output you show: `<REDACTED>`.

## 1. State the expected behaviour first

Before observing anything, write what *should* happen — from the registered
`definitions`, the `DOMAIN.md` files and the docs — into
`.circle/runtime/hunt/expected.md`. Written first, it cannot be rationalised
backwards from what you see.

## 2. Build a loop that goes red

**This is the skill.** Everything after it is mechanical.

Bring up an isolated stack (`/circle:run`, or a worktree), then find **one
command** that fails on *this* bug, in roughly this order: a failing test at the
highest seam that reaches it · a `curl` against the stack · a CLI call against a
fixture · a browser script · a replayed captured request · a minimal harness · a
fuzz loop · a `git bisect run` harness · a differential run of two versions.

Then make it **tight**: faster, a sharper assertion, deterministic (pin time,
seed randomness, isolate the filesystem). A flaky bug needs a higher reproduction
rate, not a clean one — loop it, parallelise it, narrow the window.

**Done when** you can show the command, redacted, that you have already run and
seen go red. If you cannot build one, stop: say what you tried, and ask for an
environment that reproduces it, a captured artifact, or permission to
instrument. Do not hypothesise without a loop.

## 3. Minimise, hypothesise, instrument

Shrink the input until every remaining part matters. Write hypotheses down and
rank them; test each against the loop, one variable at a time. Instrument only
to separate two hypotheses.

## 4. Hand it to the flow

The fix is an item, so it goes through the same gate as everything else:

```
circle item new <id> --kind fix --title "<symptom>"
```

Write `.circle/items/<id>/repro.md` (expected, observed, the red command) and a
`spec.md` whose only seam is that command, turned into a regression test at the
right gate. `circle spec check`, and the item is `specified`.

## Done

A `fix` item in `specified`, carrying a command that is red today and will be
green when the fix is right. Next: `/circle:tasks` — usually one task. After the
fix, `/circle:retro` asks what would have caught it.
