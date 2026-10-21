# Slicing

## Tracer bullets

- A slice cuts a narrow but **complete** path through every layer the
  behaviour needs — schema, logic, interface, test. Vertical, never one layer.
- A finished slice is demoable or verifiable alone, through its gate.
- A slice fits one fresh session: the implementer reads the task, the spec and
  the code, and nothing from the session that planned it.
- Prefactoring comes first. *Make the change easy, then make the easy change.*

## Wide refactors: expand → migrate → contract

A **wide refactor** is one mechanical change — rename a column, retype a shared
symbol — whose blast radius fans across the codebase, so no vertical slice can
land green. Sequence it instead:

1. **Expand.** Add the new form beside the old. Nothing breaks.
2. **Migrate**, in batches sized by blast radius (per package, per directory).
   Each batch is a task blocked by the expand, with `--paths` set to that batch.
   The gate stays green because the old form still exists.
3. **Contract.** Delete the old form, in a task blocked by every migrate batch.

When even batches cannot stay green alone, keep the sequence on one integration
branch and add a final **integrate-and-verify** task blocked by every batch.
Green is promised only there — say so in that task's goal.

## Paths

Paths are what the write hook enforces, so they are part of the contract the
human approves. Too narrow and the implementer is blocked mid-task; too wide and
the approval means nothing.

- Directory globs over file names.
- Tests go in the same task as the code they prove; include the test directory.
- A shared file two tasks must both touch is a blocking edge between them, or a
  prefactor task that owns it.
