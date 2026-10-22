---
description: Opens a Circle session. Loads the contracts, binds the session to one item or decision, and refuses to proceed if the contracts are invalid. Use at the start of any work.
disable-model-invocation: true
allowed-tools: Read Grep Glob Bash(circle *)
argument-hint: "[item-id | decision-id | what we are building]"
metadata:
  circle:
    kind: orchestrator
    reads: [contracts, items, initiatives]
    writes: [session]
---

# Circle session

**If the contracts are invalid, the command below exits 2 and you never see this
file.** That is the gate, not a convention.

```!
circle preflight --context
```

```!
circle item list --open
```

```!
circle decision frontier
```

## Bind the session

A session works on **exactly one** item or decision. The write hook enforces
against the bound item's approval and the claimed task's paths, so an unbound
session can write nothing outside the `default` item.

- `$ARGUMENTS` names an open item or decision → `circle session bind --item <id>`
  (or `--decision <id>`).
- `$ARGUMENTS` describes new work → it is a new item. Do not create it here;
  that is `/circle:shape`'s first step.
- No arguments → show the open items and the decision frontier above, and ask
  which.

A worktree session (`claude --worktree <id>`) is already bound to `<id>`. Confirm
it rather than rebinding.

## Standing instructions

1. **Do not re-derive how to run or test this project.** It is in the block
   above. If something is missing, propose the edit to `.circle/project.toml`.
2. **Speak the domain.** Use the terms in `.circle/DOMAIN-INDEX.md` exactly. A
   concept you need that is not there is a `circle:domain` moment — and Circle
   will flag it anyway the moment you write a type for it.
3. **You may not open a gate.** You can generate a brief, not approve it; run a
   gate, not declare it passed. Run gates with `circle quality run`, never by
   pasting their commands: only a recorded run counts.

## Next

Run `circle next` and present its suggestion. It reads the bound item's state;
you do not guess it.

$ARGUMENTS
