# spec.md template

`circle spec check` requires every `##` section below to be non-empty, and
parses the **Seams** table.

```markdown
# <item title>

## Problem
<From the user's perspective. Glossary terms.>

## Solution
<From the user's perspective.>

## Behaviours
1. As a <actor>, I want <capability>, so that <benefit>.
   <!-- exhaustive: every behaviour from shape.md, and the edge cases grilling found -->

## Seams
| Seam | Behaviours | Gate | Misses |
|---|---|---|---|
| <public boundary> | <numbers from Behaviours> | <declared gate> | <what it cannot see> |

## Decisions
- <module built or changed, interface, schema, contract> — <why>
  <!-- link ADRs by number; restate nothing they hold -->

## Out of scope
- <from shape.md, plus anything ruled out while exploring>

## Risks
- <what could make this wrong; which definition it leans on>
```

Every behaviour must appear in at least one seam row. A behaviour with no seam
is either `manual` — say so in the row, with the reason — or not part of this
item.
