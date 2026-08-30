# Circle AI

Teach Claude how to run, test and validate your project once. Commit it.
**The repo enforces it.**

Every spec-driven framework in 2026 is definition-side: spec → code. None of them
encodes how to *run* the system, how to *prove* it works, or where the truth
lives — and none makes those contracts executable. That is the gap this fills.

## Status

Pre-v0.1. The core is built and verified against two real repositories; the
cold-start trial that would confirm the central hypothesis has not been run.
See [`trials/FINDINGS.md`](trials/FINDINGS.md).

## The three contracts

One file, `.circle/project.toml`, committed with the repo:

- **knowledge** — where the truth about this project lives
- **execution** — how to run it, in isolation
- **quality** — how to prove it is correct

`circle preflight` validates all three and exits 2 when they are not honest.

## What makes it more than documentation

Exit codes are the API. `circle gate check` exiting 2 is simultaneously what
makes a `PreToolUse` hook deny a write and what aborts an injected `` !`command` ``
inside a skill. A skill can be ignored; a hook cannot.

Two rules follow from that:

- **No file is written while the contracts are invalid.**
- **No task closes without a passing gate dated after its last commit.**

The second is why the status score is a measurement. Every one of its four
components traces to a recorded event, the task graph, or git — nothing is
inferred, and no model contributes to the number.

## Quick start

```
go build -o bin/circle ./cmd/circle
circle init          # detects compose services, gates, knowledge paths
circle preflight     # exit 2 until the contracts are honest
circle serve         # read-only observation app on :7777
```

Install the [plugin](plugin/README.md) to get the skills and enforcement hooks.

## Documents

| | |
|---|---|
| [`circle-ai.prd.md`](circle-ai.prd.md) | The plan of record |
| [`circle-ai.review.md`](circle-ai.review.md) | Platform and landscape validation |
| [`circle-ai.cli.md`](circle-ai.cli.md) | CLI surface, exit codes, hook wiring |
| [`trials/FINDINGS.md`](trials/FINDINGS.md) | What building it actually surfaced |
