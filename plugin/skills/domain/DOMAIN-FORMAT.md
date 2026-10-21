# DOMAIN.md format

One `DOMAIN.md` per **bounded context**, in the directory it covers: `src/orders/DOMAIN.md`
covers `src/orders/`. A small repository may have one at the root. Circle finds every one
and indexes them into `.circle/DOMAIN-INDEX.md` — never edit the index, edit these.

Circle parses three things, so keep their shape exact:

- `**Term**:` at the start of a line — a term. The definition follows on the next lines.
- `_Avoid_:` directly under a term — banned synonyms. Code that names a type with one is
  flagged and told which term to use.
- Backticked paths under `## Modules` — the modules this domain owns.

```markdown
# <Context name>

<One paragraph: what this part of the system is for, in the domain's words.>

## Language

**<Term>**:
<One or two sentences. What it is in the domain, never how it is built.>
_Avoid_: <synonym>, <synonym>

## Relationships

- An **<Term>** has many **<Term>**
- A **<Term>** is a **<Term>**

## Modules

- `src/orders` — <what this module holds, in the terms above>

## Flagged ambiguities

- "<word>" was used for both X and Y. Resolved: X is **<Term>**; Y is **<Term>**.
```

Rules:

- **Every new concept the code introduces gets a term.** A new type whose name is not in
  any `DOMAIN.md` is fed back to you the moment it is written. Technical words —
  Service, Repository, Handler, Config, Props and the like — are stripped before matching,
  so `OrderRepository` is covered by **Order**. Plumbing that is not a concept goes under
  `[domain] ignore` in `.circle/project.toml`, not into the language.
- **Every new module gets an owner.** List it under `## Modules`, put a `DOMAIN.md` inside
  it, or name its path in an ADR or architecture document.
- One entry per concept. A second name for it goes under `_Avoid_`, not a second entry.
- No file paths, types or endpoints in a definition. If a definition names a module, it is
  a spec.
- A resolved ambiguity stays in **Flagged ambiguities** — it is the record of why the word
  is banned.
- When a term belongs to two contexts with different meanings, define it in both and say
  so in each. That is the bounded context doing its job, not a contradiction.
