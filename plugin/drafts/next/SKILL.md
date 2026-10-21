---
description: Says where the bound item stands, which Circle skill comes next, and how to cross the phase boundary — continue, clear, hand off, subagent or compact. Use whenever unsure what to do next.
disable-model-invocation: true
allowed-tools: Read Bash(circle *)
metadata:
  circle:
    kind: orchestrator
    reads: [items, initiatives, decisions, events]
    writes: []
---

# Next

```!
circle next
```

The block above is computed from recorded state — the bound item's lifecycle
state, open tasks, open decisions, pending approvals, red gates. **Present it as
it stands.** Before describing what a skill does, read its `SKILL.md`; the
summaries below are for orientation.

## The routes

| Situation | Route |
|---|---|
| New work that fits one session of thinking | `/circle:shape` → `/circle:spec` → `/circle:tasks` → `/circle:brief` → `/circle:implement` → `/circle:review` → `/circle:retro` |
| Too big or too foggy for one session | `/circle:chart`, then each resulting item through the route above |
| Something is broken | `/circle:hunt` → `/circle:tasks` → ... → `/circle:retro` |
| Need the app running | `/circle:run` |
| Need to know what is proven | `/circle:verify` |
| Work is leaving this session | `/circle:handoff` |

## The phase boundary

A **phase** ends when you think "ok, that part is done". Decide how to cross it
**at** the boundary, never mid-phase. Take the first **yes**:

1. **Continue** — does the next phase need this one verbatim, and is there room
   left? Shape → spec → tasks is the standard yes: keep them in one context.
2. **`/clear`** — is this context disposable? Between two `/circle:implement`
   tasks it is: the task, spec and code carry everything.
3. **`/circle:handoff`** — is the work travelling: new harness, new repo, a
   colleague, a side task forked mid-phase?
4. **Subagent** — is the next step tightly scoped enough to run with nobody
   steering? Review, research, and every task in `/circle:implement --all`.
5. **`/compact`** with an instruction — otherwise.

Every move except **Continue** turns the session into a summary of itself. In
Circle that loss is smaller than usual — the spec, brief, tasks, decisions and
timeline are on disk — which is why **Clear** is right more often here than in an
unstructured session.
