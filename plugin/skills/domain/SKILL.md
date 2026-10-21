---
description: Sharpen and record the domain model in DOMAIN.md files and ADRs. Use when a term is vague or overloaded, when the code introduces a new concept or module, when circle flags an undocumented concept, or when a decision may deserve an ADR.
allowed-tools: Read Grep Glob Edit Write Bash(circle *)
metadata:
  circle:
    kind: practice
    reads: [domain]
    writes: [domain, adr]
---

# Domain

```!
circle domain index --print
```

The block above is the domain model as it stands: every bounded context and its
terms, every decision, every module and what documents it. Circle generates it
from the `DOMAIN.md` files, ADRs and architecture documents, and regenerates it
on every write. **Edit those, never the index.**

## When this runs

- **Shaping.** Called alongside `grill` to settle the language before planning.
- **A write was flagged.** Circle told you a new type or module is missing from
  the model. Fix it now, before the next write: the flag repeats until you do.
- **Review.** `circle domain check --since <base>` lists everything a branch added
  that the model does not cover.

## While the conversation runs

- **Challenge against the model.** A term used against its defined meaning gets
  called out at once: "Orders defines *cancellation* as X; you seem to mean Y.
  Which?"
- **Sharpen.** A vague or overloaded word gets a proposed canonical term:
  "*account* — the Customer or the User?"
- **Find the context.** A word meaning different things in two places is two
  terms in two `DOMAIN.md` files, each saying so. That is a boundary, not a
  conflict.
- **Stress-test with scenarios.** Invent the edge case that forces a boundary
  between two concepts to be stated.
- **Cross-reference the code.** When the human states how something works, check
  whether the code agrees. A contradiction is a finding, not a footnote.

## Write as you go

- **Terms.** When a term resolves, write it into the `DOMAIN.md` of the context
  it belongs to, in [DOMAIN-FORMAT.md](DOMAIN-FORMAT.md). No such file yet? Create
  it in the directory the context covers — Circle finds it on the next write.
  Language only: what a thing *is*, never how it is built.
- **Modules.** A new module goes under `## Modules` in its context's `DOMAIN.md`,
  with one line on what it holds.
- **ADR.** Offer one only when the decision is **hard to reverse**, **surprising
  without context**, and **the result of a real trade-off**. All three, or no ADR.
  Write it in [ADR-FORMAT.md](ADR-FORMAT.md) under `docs/adr/`, or under the
  context's own `docs/adr/` when only that context is affected.

There is nothing to register. Circle indexes every `DOMAIN.md`, ADR and
architecture document it finds, and `circle knowledge verify` checks each one
against the code — a definition that no longer matches the code is reported,
not trusted.

## Done

Every concept and module that came up is in a `DOMAIN.md`, deliberately listed
under `[domain] ignore` as plumbing, or named as a contradiction with the code.
`circle domain check` is clean for this change.
