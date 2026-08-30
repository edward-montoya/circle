---
description: Runs every quality gate and reports what is actually proven. Use before claiming any work is done.
disable-model-invocation: true
allowed-tools: Read Bash(circle *)
---

# Verify

```!
circle quality run
```

```!
circle status
```

## Your task

Report the result above exactly as it stands. Then:

- For any gate that failed, show the failure and propose the fix. Do not rerun
  selectively to get a greener number.
- For any task still open, name the gate blocking it.
- **Do not imply completeness the contract does not have.** If there is no
  linter and no coverage floor, the honest answer is that the declared gates
  passed and nothing else was measured.

A task closes because a command passed after the work, not because you judged it
done. If `circle task close` refuses, that is the system working.
