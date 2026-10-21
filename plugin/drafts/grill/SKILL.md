---
description: Interview the human about a plan, decision or idea until every branch is settled. Use when shaping work, resolving a grill decision, or when the human says grill.
allowed-tools: Read Grep Glob Bash(circle *) Agent
metadata:
  circle:
    kind: practice
    reads: [knowledge, contracts]
    writes: []
---

# Grill

Interview the human **relentlessly** until you share one understanding. Hold it
as a **design tree**: every decision branches into the decisions that hang off it.

## The split

**Facts are yours. Decisions are theirs.**

- A fact is anything the repository, the contracts or the running system can
  answer: what the stack is, which gates exist, what the glossary says, whether a
  module already does this. Find it yourself. `circle preflight --context`
  already holds how to run and test; the knowledge registry holds where the
  truth lives. When a fact needs digging, send a subagent and keep asking the
  questions that do not wait on it.
- A decision is a choice between options a human must own. Put every one to
  them, and wait.

Asking the human a fact the repo could answer is the failure this skill exists
to prevent. Answering a decision on their behalf is the other one.

## Rounds

The **frontier** is every decision whose prerequisites are settled — the
questions you can ask now without guessing at an answer you have not heard.
Ask the whole frontier in one round. A question that depends on another one
still open this round belongs to the next round.

```
❓ **Q1 · <title>** — <question; options if there are options>

➡️ <your recommended answer, and the one-line reason>

---

❓ **Q2 · <title>** — ...
```

Word every question so **yes** accepts your recommendation.

After each round, recompute the frontier: settled decisions unblock the ones
that hung on them. Keep a running list of what is settled, in the human's
words, so the orchestrator that called you can write it down.

## Done

The frontier is empty: every branch visited, nothing silently assumed. Say so,
list the settled decisions, and **wait for the human to confirm** before the
calling skill acts on them.
