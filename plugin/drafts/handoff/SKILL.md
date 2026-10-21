---
description: Records what landed, from git rather than memory, and writes a handoff the next session — or another harness or a colleague — can start from. Use before ending a session or moving the work elsewhere.
disable-model-invocation: true
allowed-tools: Read Write Bash(circle *)
argument-hint: "[what the next session is for]"
metadata:
  circle:
    kind: orchestrator
    reads: [timeline, item, tasks, decisions]
    writes: [handoff]
---

# Handoff

```!
circle timeline sync
```

```!
circle handoff write
```

The block above is the item's handoff skeleton, generated from the timeline,
the task graph and the decision log. **Git is the truth set; your account is
not.** You fill in only what git cannot know.

## Fill in

In `.circle/items/<id>/handoff.md` (or the decision's file, for a decision
session):

- **Phantoms.** Any commit you reported and git does not have. Say so plainly.
- **Uncommitted.** Anything in the working tree. Most sessions end here, and
  leaving it out lies by omission at exactly the moment someone resumes cold.
- **Why.** Decisions made this session that no artifact holds yet — and if one
  should be an ADR, say so rather than burying it here.
- **Next.** The task or decision id and its goal. One line, not a paragraph.
- **Suggested skills.** Which Circle skills the next session should run, in
  order.

**Point, do not restate.** The spec, the brief, ADRs, tasks and commits already
exist; link them by path or id. Redact every secret.

`$ARGUMENTS`, if given, is what the next session is for: tailor **Next** and
**Suggested skills** to it.

## When this is the right move

A handoff buys **portability**. Reach for it when the work travels: a new
harness, a new directory or repo, a colleague, or a side task forked off
mid-phase. Staying in the same harness and repo, `/clear` or `/compact` is
usually better — `circle next` will say which.
