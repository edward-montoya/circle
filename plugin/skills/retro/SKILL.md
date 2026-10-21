---
description: Looks back over a session and proposes changes to the contract, not the code — new gates, new knowledge, sharper standards — so the next session starts from a better repository. Use after review, or after any session that went sideways.
disable-model-invocation: true
allowed-tools: Read Grep Glob Bash(circle *) Bash(git *)
argument-hint: "[session or item id]"
metadata:
  circle:
    kind: orchestrator
    reads: [events, timeline, contracts, review]
    writes: [contracts]
---

# Retro

```!
circle event list --item "$(circle session current)" --types gate.deny,gate.run,preflight.force,skill.run
```

```!
circle preflight --context
```

You are improving the **environment** the next agent works in. The primary
source is this session, if it is still in context, plus the event log above —
denied writes, red gates, forced preflights and which skills actually ran are
facts, not recollection.

## Where each finding goes

This is Circle's thesis applied to itself: **a mistake that can be caught by a
command must become a command.**

| Finding | Destination | How |
|---|---|---|
| **Mechanical** mistake — a pattern a tool could catch | A quality gate | `circle quality add <name> "<cmd>"` — a lint rule, a new test command, a check script |
| A gate exists but is unwired or broken | Fix the contract | Edit `[quality]`; `circle preflight` must still pass |
| **Judgement** call the reviewer missed | A standard | Add the rule to the registered standards file (`circle knowledge add <f> --as standards`) |
| The agent could not find something | Knowledge | `circle knowledge add <path> --as docs` — a pointer, not a copy |
| A term caused confusion | Domain model | Run the `domain` practice; the right `DOMAIN.md` gets the term or an `_Avoid_` |
| A registered doc misled the agent | Drift | `circle knowledge verify`; fix or unregister the doc |
| An expensive or repeated tool call | Tooling | A `circle` subcommand or a skill change — propose it as an issue |
| A plan kept breaking mid-build | Slicing | A note for `/circle:tasks`, or paths that were too narrow |

Default to the gate over the standard. A standard is read by a reviewer that
can miss it; a gate cannot be missed.

## Present

Candidates in order of severity: what happened (with the event or commit), the
destination, and the exact command or edit. **Apply none until the human picks.**
Then apply the chosen ones and run `circle preflight` — a retro that breaks the
contract has made the environment worse.
