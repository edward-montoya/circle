---
description: Shapes an idea into a settled item by interview, writing the domain model — DOMAIN.md terms and ADRs — as it goes. Use when starting any new feature, fix or refactor that fits in one session's thinking.
disable-model-invocation: true
allowed-tools: Read Grep Glob Edit Write Bash(circle *) Agent Skill
argument-hint: "<the idea>"
metadata:
  circle:
    kind: orchestrator
    requires: none | shaping
    produces: shaped
    reads: [contracts, knowledge, domain]
    writes: [item, domain, adr]
    calls: [lifecycle.grill, lifecycle.domain]
---

# Shape

```!
circle preflight --context
```

## 1. Open the item

If the session is not bound to an item in state `shaping`, create one:

```
circle item new <id> --title "<title>" --kind feature|fix|refactor [--initiative <i>] --bind
```

Until the item has an approved plan, the write hook lets this session write only
`DOMAIN.md` files, ADRs and the item's own files under `.circle/items/<id>/` —
exactly what shaping produces.

Pick a short kebab-case id from the idea. If the idea already belongs to an
initiative, name it.

**Too big?** If, after the first round, the idea plainly needs more than one
session of *deciding* — not building — stop and suggest `/circle:chart`
instead. A well-scoped feature is never a chart.

## 2. Grill with the domain open

Call the Skill tool for the `grill` and `domain` slots named under
`[lifecycle]` in the block above (default `circle:grill`, `circle:domain`).
Run them together: the interview settles decisions, the domain practice writes
terms and ADRs the moment they crystallise.

Ground every round in the contracts:

- **Execution** — can the change run in an isolated stack? Does it touch a
  service `[execution]` declares?
- **Quality** — which declared gate could prove each behaviour? A behaviour no
  gate can prove is a decision: add a gate, accept `manual`, or cut it.
- **Knowledge** — which registered definition does this extend or contradict?

If a question needs a runnable answer (a state model you must feel, a UI you must
see), it is a prototype. Record it as an open question in `shape.md` and say so;
do not build the prototype in this session.

## 3. Write `shape.md`

`.circle/items/<id>/shape.md`, in the human's own words:

```markdown
## Goal
<one sentence: what is true when this item is done>

## Decisions
- <settled decision> — <why, in one line>

## Behaviours
- <behaviour> — proved by `<gate>` | manual (<reason>)

## Out of scope
- <ruled out>

## Open
<!-- must be empty to advance -->
```

Write the file only once **Open** is empty. The item's state is derived from
what is on disk, so `shape.md` existing *is* the item being shaped — writing it
with open questions would advance an item that is not settled.

## Done

`circle item show` reports `shaped`. **Do not clear or compact.** `/circle:spec`
wants this conversation as its primary source; carry straight on.
