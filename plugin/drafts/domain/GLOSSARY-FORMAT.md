# GLOSSARY.md format

```markdown
# <Project> glossary

## Language

**<Term>**:
<One or two sentences. What it is in the domain, never how it is built.>
_Avoid_: <synonyms that must not be used for it>

## Relationships

- A **<Term>** has many **<Term>**
- A **<Term>** is a **<Term>**

## Flagged ambiguities

- "<word>" was used for both X and Y. Resolved: X is **<Term>**; Y is **<Term>**.
```

Rules:

- One entry per concept. A second name for it goes under `_Avoid_`, not a second entry.
- No file paths, no types, no endpoints. If a definition names a module, it is a spec.
- Bold every glossary term where another entry uses it, so the terms link by eye.
- A resolved ambiguity stays in **Flagged ambiguities** — it is the record of why
  the word is banned.
