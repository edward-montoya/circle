---
description: Reviews an item's diff on three separate axes — standards, spec, and traceability — and holds the item open on blocking findings. Use when an item reaches reviewing, or to review any branch against its brief.
disable-model-invocation: true
allowed-tools: Read Grep Glob Bash(circle *) Bash(git *) Agent
metadata:
  circle:
    kind: orchestrator
    requires: reviewing
    produces: done
    reads: [item, spec, brief, knowledge, timeline]
    writes: [review]
---

# Review

```!
circle item require --state reviewing
```

```!
circle review drift
```

```!
circle knowledge list --as standards --content
```

## Three axes, never merged

| Axis | Question | Decided by |
|---|---|---|
| **Trace** | Is every commit tied to a task, and every changed file inside the approved paths? | **Computed** — the `circle review drift` block above. Not judged |
| **Spec** | Does the diff do what `spec.md` asked — no less, no more? | A subagent |
| **Standards** | Does the diff follow the registered standards and the smell baseline? | A subagent |

A change can pass one and fail another. Reporting them apart stops one axis from
hiding the other, so **never rerank across axes.**

## Steps

1. **Pin the diff.** The fixed point is the commit the item's approval was
   recorded against (`circle item show` names it). Confirm the diff is
   non-empty before spending subagents on it.
2. **Trace** is already above. Any untraced commit or out-of-radius file is a
   blocking finding as it stands.
3. **Spawn Spec and Standards together**, as parallel subagents, each given
   pointers rather than content:
   - **Spec:** the diff command, `spec.md`, the brief. Report (a) behaviours
     missing or partial, (b) behaviour nobody asked for, (c) behaviours present
     but wrong. Quote the spec line for each. Under 400 words.
   - **Standards:** the diff command, the standards files above, and
     [SMELLS.md](SMELLS.md) pasted in full. Report documented-standard breaches
     as hard, smells as judgement calls; a documented standard overrides a
     smell; skip anything a gate already enforces. Under 400 words.
4. **Record each axis:**

   ```
   circle review record --axis trace     --file <f>
   circle review record --axis spec      --file <f>
   circle review record --axis standards --file <f>
   ```

   Findings carry a severity. Only `blocking` holds the item.
5. **Report** the three axes under their own headings, then one line per axis:
   count and the worst finding within it.

## Done

- No blocking findings and `circle quality run` green → `circle review close`,
  item `done`. Suggest `/circle:handoff`, then `/circle:retro` **in this session,
  before clearing** — retro wants the primary source.
- Blocking findings → the item returns to `building`. Fix them as new tasks
  through `/circle:tasks`, which re-plans and needs a fresh approval. Fixing
  outside the plan is what the trace axis exists to catch.
