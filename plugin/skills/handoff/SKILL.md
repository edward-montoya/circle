---
description: Records what actually landed this session, from git rather than from memory. Use before ending a session.
disable-model-invocation: true
allowed-tools: Read Bash(circle *)
---

# Handoff

```!
circle timeline sync
```

```!
circle timeline show
```

```!
circle status
```

## Your task

Summarise the session **from the timeline above, not from your recollection of
it**. Git is the truth set; your account is not.

Call out specifically:

- Any commit marked **phantom** — you reported it and git does not have it.
- Anything **uncommitted** — most sessions end here, and omitting it lies by
  omission at exactly the moment someone resumes cold.
- Any task still open, and the gate blocking it.

Then say what the next session should pick up first. Be concrete: the task id
and its goal, not a paragraph of context.
