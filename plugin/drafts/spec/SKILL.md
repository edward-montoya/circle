---
description: Turns a shaped item into a spec whose test seams map to declared quality gates. No new interview. Use right after /circle:shape, in the same session.
disable-model-invocation: true
allowed-tools: Read Grep Glob Write Bash(circle *) Agent
metadata:
  circle:
    kind: orchestrator
    requires: shaped
    produces: specified
    reads: [item, glossary, knowledge, quality]
    writes: [spec]
---

# Spec

```!
circle item require --state shaped
```

```!
circle item show --with shape
```

```!
circle quality list
```

**Synthesise; do not interview.** Everything the spec needs is in `shape.md`,
the conversation, and the repository. If a gap appears that only the human can
fill, the item is not shaped — say so and return to `/circle:shape`.

## 1. Explore

Read the code the item touches, using glossary terms to search. Respect every
registered ADR in the area. Use a subagent if the area is wide; keep only its
conclusions.

Look for a **prefactor**: a change that makes the real change easy. If one
exists, it becomes the first task.

## 2. Agree the seams

A **seam** is a public boundary you test behaviour through. Prefer existing
seams to new ones, and the highest seam that can see the behaviour. The ideal
number is one.

Propose them as a table and **confirm with the human before writing**:

| Seam | Behaviours it proves | Gate | Misses |
|---|---|---|---|
| `POST /orders` | create, reject invalid | `test:integration` | UI rendering |

**Every seam names a declared gate.** This is the Circle rule Matt's `to-spec`
does not have, and it is checked: `circle spec check` refuses a seam whose gate
`circle quality list` does not contain. A seam no gate can run produces a task
that could never close honestly.

## 3. Write the spec

`.circle/items/<id>/spec.md`, using [SPEC-TEMPLATE.md](SPEC-TEMPLATE.md). Glossary
terms throughout; no file paths or code, except a prototype snippet that encodes
a decision more exactly than prose can.

```
circle spec check
```

## Done

`circle spec check` exits 0 and `circle item show` reports `specified`.
Carry on to `/circle:tasks` in this session.
