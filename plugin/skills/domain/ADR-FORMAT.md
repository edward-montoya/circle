# ADR format

`docs/adr/NNNN-<slug>.md`, numbered from `0001`, never renumbered.

```markdown
# NNNN · <decision, as a statement>

**Status:** accepted | superseded by NNNN
**Date:** YYYY-MM-DD
**Item:** <circle item id, if one prompted it>

## Context

<The forces at play. What made this a decision at all.>

## Decision

<What we chose, in one paragraph.>

## Alternatives

| Option | Why not |
|---|---|
| <option> | <the specific reason> |

## Consequences

<What becomes easier, what becomes harder, what is now hard to reverse.>
```

An ADR is registered as `definitions`, so `circle knowledge verify` checks it
against the repository. Name technologies and paths precisely — a vague ADR
cannot be verified, and an unverifiable one rots silently.
