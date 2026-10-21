---
description: Charts an effort too big for one session as an initiative — a map of decisions resolved one per session until the way is clear, then hands off as items. Use for greenfield projects or large, foggy features; never for a well-scoped one.
disable-model-invocation: true
allowed-tools: Read Grep Glob Write Bash(circle *) Agent Skill
argument-hint: "<the loose idea> | <initiative-id> [decision-id]"
metadata:
  circle:
    kind: orchestrator
    reads: [contracts, knowledge, initiatives, decisions]
    writes: [initiative, decisions, items]
    calls: [lifecycle.grill, lifecycle.domain]
---

# Chart

```!
circle preflight --context
```

An idea has arrived that is too big for one session and wrapped in fog: the way
from here to the **destination** is not visible yet. Charting finds the way; it
does not charge at the destination.

**Plan, do not build.** Each decision resolves a question. The map is done when
nothing is left to decide — and then it produces **items**, which go through the
ordinary flow and the ordinary gate. The pull to just start building is the
signal you have reached the edge of the map.

Two modes. **Either way, resolve at most one decision per session**, except
`research` decisions, which run in parallel.

## Mode 1 — chart a new map (`$ARGUMENTS` is an idea)

1. **Name the destination.** Grill (with the `domain` practice open) until the
   destination is one or two lines: the change, spec set or decision this effort
   ends in. It fixes the scope, so it comes first.
2. **Map the frontier, breadth-first.** Grill again across the whole space, not
   deep on one thread. If **no fog** appears — the way is already clear and
   small — you do not need a map: stop and suggest `/circle:shape`.
3. **Create the initiative:**

   ```
   circle initiative new <id> --destination "<one or two lines>"
   ```

   Then write **Notes**, **Not yet specified** and **Out of scope** into its
   `map.md` ([MAP-FORMAT.md](MAP-FORMAT.md)).
4. **Create the decisions you can state sharply now** — first all of them, then
   wire blockers in a second pass so every reference is a real id:

   ```
   circle decision new --initiative <id> --type research|prototype|grill|task \
     --question "<the question>" [--blocked-by <ids>]
   ```

   **Ticket or fog?** If you can state the question precisely now — even if you
   cannot answer it yet — it is a decision. If you cannot phrase it sharply, it
   stays in **Not yet specified**.
5. **Fire research.** For each `research` decision, a background subagent
   investigates primary sources and writes a cited file it links from the
   decision.
6. **Stop.** Charting is one session's work. Run `/circle:handoff`.

## Mode 2 — work the map (`$ARGUMENTS` is an initiative id)

1. `circle initiative show <id>` — the map at low resolution. Do not load every
   decision body.
2. Take the named decision, or the first in `circle decision frontier`, and
   **claim it before any work**: `circle decision claim <did>`. Other sessions
   are working the same map; the claim is what keeps them off it.
3. Resolve it by its type:

   | Type | Who | How |
   |---|---|---|
   | `research` | AFK | A subagent reads primary sources; cited findings, linked |
   | `prototype` | HITL | A throwaway on a `prototype/<name>` branch that makes the question concrete; the human reacts to it |
   | `grill` | HITL | The `grill` and `domain` practices. The default |
   | `task` | either | Work that must happen before a decision can be made — provisioning, signing up, moving data. Records the resulting facts |

   A HITL decision resolves only through the live exchange. Answering the human's
   side yourself breaks it.
4. **Record:** `circle decision resolve <did> --answer <file>`. The answer lives
   in the decision; the map gets one line pointing at it.
5. **Push back the fog.** Graduate anything now specifiable into new decisions.
   A decision that turns out to lie past the destination:
   `circle decision scope-out <did> --why "..."`. A decision this answer
   invalidates gets updated or removed.

## When the map clears

The frontier is empty and **Not yet specified** is empty. The way is clear. Each
buildable piece becomes an item, linked to the initiative and seeded from the
decisions it rests on:

```
circle item new <id> --initiative <initiative-id> --title "..."
```

Each item then runs `/circle:shape` → `/circle:spec` → ... with the decisions
as its primary source. Going straight from map to code skips the spec and throws
away the linked detail.
