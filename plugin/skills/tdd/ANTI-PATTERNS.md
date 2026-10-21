# Tests that are not worth keeping

| Pattern | The tell | Instead |
|---|---|---|
| **Implementation-coupled** | Breaks on a refactor when behaviour did not change. Mocks an internal collaborator, tests a private function, or checks the database instead of the interface | Assert through the seam's public interface only |
| **Tautological** | The expected value is computed the way the code computes it — `expect(add(a, b)).toBe(a + b)` — so it can never disagree | Use a literal, a worked example, or a line from the spec |
| **Horizontal** | All tests written first, then all code. Tests describe the imagined shape, not the behaviour | One test, one implementation, repeat |
| **Unseamed** | A test at a boundary the spec never agreed | Ask the human whether the seam belongs in the spec |
| **Gate-less** | Passes locally through a command no declared gate runs | Run it through the task's gate; if the gate cannot run it, the quality contract is missing something — say so |

## Mocking

Mock at the **edge of the system** — the network, the clock, a third-party API —
never between two of your own modules. If two of your own modules are hard to
test together, that is a design finding for `/circle:review`, not a reason to
mock.

Prefer what the execution contract already gives you: if `[execution]` brings
up a real database in an isolated Compose project, an integration gate against
it beats a mock of it.
