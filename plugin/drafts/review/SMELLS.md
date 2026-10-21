# Smell baseline

Applies even when the repo registers no standards. Every smell is a labelled
judgement call ("possible Feature Envy"), never a hard violation. A registered
standard always overrides it.

Each reads *what it is* → *the fix*.

- **Mysterious Name** — a name that does not say what it does or holds, or
  disagrees with the glossary. → Rename it to the glossary term; if no honest
  name comes, the design is murky.
- **Duplicated Code** — the same logic shape in more than one hunk. → Extract it.
- **Feature Envy** — a function that reaches into another module's data more
  than its own. → Move it to the data.
- **Data Clumps** — the same few fields travelling together. → Give them a type.
- **Primitive Obsession** — a string or number standing in for a domain
  concept the glossary names. → Give the concept its own type.
- **Repeated Switches** — the same branch on the same type in several places.
  → One map or polymorphism.
- **Shotgun Surgery** — one logical change scattered across many files. → Gather
  what changes together. In Circle this usually also shows as a wide blast
  radius in the brief.
- **Divergent Change** — one module edited for unrelated reasons. → Split it.
- **Speculative Generality** — abstraction the spec does not need. → Inline it.
- **Message Chains** — `a.b().c().d()` the caller should not depend on. → Hide
  the walk behind one method.
- **Middle Man** — a module that only delegates. → Call the target directly.
- **Refused Bequest** — an implementer that ignores most of what it inherits.
  → Composition.
