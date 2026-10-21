# Draft skills — not loaded

The plugin loads `plugin/skills/` only. These drafts are for review, and they
call CLI commands that do not exist yet. Design and CLI surface:
[`docs/LIFECYCLE.md`](../../docs/LIFECYCLE.md).

| Draft | Kind | Requires → produces | Replaces |
|---|---|---|---|
| [`start`](start/SKILL.md) | orchestrator | — (binds session) | `skills/start` |
| [`next`](next/SKILL.md) | orchestrator | — | new |
| [`chart`](chart/SKILL.md) | orchestrator | — → initiative | new |
| [`shape`](shape/SKILL.md) | orchestrator | none · shaping → shaped | new |
| [`spec`](spec/SKILL.md) | orchestrator | shaped → specified | new |
| [`tasks`](tasks/SKILL.md) | orchestrator | specified → planned | `skills/tasks` |
| [`implement`](implement/SKILL.md) | orchestrator | approved · building → reviewing | new |
| [`review`](review/SKILL.md) | orchestrator | reviewing → done | new |
| [`hunt`](hunt/SKILL.md) | orchestrator | — → specified (fix item) | new |
| [`handoff`](handoff/SKILL.md) | orchestrator | — | `skills/handoff` |
| [`retro`](retro/SKILL.md) | orchestrator | — (edits contracts) | new |
| [`grill`](grill/SKILL.md) | practice | — | new |
| [`domain`](domain/SKILL.md) | practice | — | new |
| [`tdd`](tdd/SKILL.md) | practice | — | new |

`brief`, `run` and `verify` are unchanged.

To promote a draft: build the commands it calls, move it to `plugin/skills/`,
and run `claude plugin validate`.
