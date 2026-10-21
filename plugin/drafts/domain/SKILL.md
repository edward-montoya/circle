---
description: Sharpen the project's domain language. Use when a term is vague or overloaded, when writing GLOSSARY.md, or when a decision may deserve an ADR.
allowed-tools: Read Grep Glob Edit Write Bash(circle *)
metadata:
  circle:
    kind: practice
    reads: [knowledge]
    writes: [glossary, adr]
---

# Domain

```!
circle knowledge list --as glossary --as definitions --content
```

The block above is the registered glossary and ADRs. This skill **changes** the
domain model. Reading the glossary for vocabulary is a habit every skill has;
this one is for when the language itself is the problem.

## While the conversation runs

- **Challenge against the glossary.** A term used against its registered meaning
  gets called out at once: "the glossary defines *cancellation* as X; you seem to
  mean Y. Which?"
- **Sharpen.** A vague or overloaded word gets a proposed canonical term:
  "*account* — the Customer or the User?"
- **Stress-test with scenarios.** Invent the edge case that forces a boundary
  between two concepts to be stated.
- **Cross-reference the code.** When the human states how something works, check
  whether the code agrees. A contradiction is a finding, not a footnote.

## Write as you go

- **Glossary.** When a term resolves, write it to `GLOSSARY.md` immediately, in
  [GLOSSARY-FORMAT.md](GLOSSARY-FORMAT.md). Language only — no implementation,
  no plan, no scratch.
- **ADR.** Offer one only when the decision is **hard to reverse**, **surprising
  without context**, and **the result of a real trade-off**. All three, or no
  ADR. Write it in [ADR-FORMAT.md](ADR-FORMAT.md) under `docs/adr/`.

Create either file lazily, on the first thing worth writing. The first time
you create one, register it so drift detection covers it:

```
circle knowledge add GLOSSARY.md --as glossary
circle knowledge add docs/adr/ --as definitions
```

That registration is what makes the glossary different from a markdown file:
`circle knowledge verify` will flag it when the code stops agreeing with it.

## Done

Every term that came up is either in the glossary, explicitly left alone, or
named as a contradiction with the code. Report which.
