---
description: Brings up this project's stack from the execution contract. Use when you need the app running locally.
disable-model-invocation: true
allowed-tools: Read Bash(circle *) Bash(docker compose *) Bash(make *) Bash(curl *)
---

# Run the stack

```!
circle preflight --context
```

## Your task

Read `[execution]` from `.circle/project.toml`, then run `bootstrap`, then `up`.
Wait for the declared URL to answer and report the mapped ports.

**You execute; the contract only records.** Do not wrap these commands in a
helper script or a Makefile target — a second way to start the app is a second
thing that goes stale. If a command is wrong, fix the contract, not the call.

If the contract sets `ports_fixed`, two checkouts cannot run at once. Say so
plainly rather than picking different ports; the contract would then be lying.
