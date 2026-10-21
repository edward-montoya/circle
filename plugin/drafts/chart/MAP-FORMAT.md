# map.md format

`.circle/initiatives/<id>/map.md`. **An index, not a store**: a decision's answer
lives in exactly one place — its own file — and the map gists it and links.

`circle initiative show` renders **Decisions so far** from the resolved
decisions and lists the frontier by query. Open decisions are never listed by
hand.

```markdown
# <initiative title>

## Destination
<What reaching the end of this map looks like. One or two lines. Every session
reads it before choosing a decision.>

## Notes
<The domain. Which skills or slots every session should use. Standing
preferences for this effort.>

## Decisions so far
<!-- generated: one line per resolved decision -->
- [<decision title>](decisions/03-auth-provider.md): <one-line gist of the answer>

## Not yet specified
<!-- in scope, but too dim to state as a question yet; graduates as the frontier advances -->
- <the suspected question, the area to revisit>

## Out of scope
<!-- ruled beyond the destination; never graduates -->
- [<decision title>](decisions/07-sso.md): <gist> — <why it is out>

## Items
<!-- generated: the items this map has produced, with their state -->
```

Always refer to a decision by its **title**, never a bare id. A wall of
`d-03, d-04, d-07` is illegible; the id rides inside the link.
