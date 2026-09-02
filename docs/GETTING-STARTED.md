# Getting started

A complete walkthrough on your own repository, from nothing to a task closed with
evidence. Roughly 15 minutes.

Each step says **what it is for**, not just what to type — because the point of
Circle is the contract, and typing commands without understanding it produces a
contract nobody trusts.

---

## Before you start

You need `git` and Go 1.22+. Docker only if your project uses Compose.

```bash
git clone https://github.com/edward-montoya/circle.git
cd circle && go build -o bin/circle ./cmd/circle
export PATH="$PATH:$(pwd)/bin"
circle doctor
```

`circle doctor` tells you what is missing. Only `git` is required; anything else
absent means Circle degrades rather than fails.

Work in a repository you know well. The whole exercise is about writing down what
you already know, and you cannot check the contract is honest in a codebase you
have never seen.

---

## Step 1 — Scaffold the contract

```bash
cd ~/your-project
circle init
```

**What this is for.** `init` reads your repository and writes down what it finds:
which compose services hold your code, which commands test it, which paths hold
documentation. The output is `.circle/project.toml` — one committed file that is
the entire point of the framework.

**What it will get right.** Compose services split into application containers
(`build:` present) and infrastructure (`image:` only). Test and lint commands
from `pyproject.toml`, `package.json` or `go.mod`.

**What it will get wrong.** Anything that is not stated in a file. A service that
looks like infrastructure but holds code. A test command that needs an env var.
Fix those by hand — the file is designed to be edited.

**What it will never do.** Invent tooling. If your repo has no linter, the
contract has no lint gate. A contract that claims a command nobody can run is
worse than one that admits the gap.

> Open `.circle/project.toml` now and read it. If a line is wrong, this is the
> cheapest moment to fix it.

---

## Step 2 — Make preflight pass

```bash
circle preflight
```

**What this is for.** Preflight asks one question: *could a stranger who just
cloned this repo run and test it using only this file?* Anything that would make
the answer "no" is reported with the command that fixes it.

Two severities:

- `✗` **blocks.** A registered path that matches nothing; a service that is not
  in the compose file; a gate that cannot run and no bootstrap to make it
  runnable.
- `⚠` **advises.** No coverage floor. No definitions registered. Real gaps, but
  not ones that stop you working.

Typical fixes:

```bash
circle knowledge add docs/ --as definitions   # register where specs live
circle knowledge list --unresolved            # what still points at nothing
```

Preflight exits `0` when it passes and `2` when it does not. That exit code is
load-bearing: it is what a hook reads to deny a write, and what aborts a skill
before Claude sees it.

**Commit `.circle/project.toml` now**, before doing anything else. That is the
moment the workflow becomes replicable.

---

## Step 3 — Understand the three contracts

Read your generated file against this. Five minutes here saves an hour later.

### knowledge — *where is the truth about this project?*

```toml
[knowledge]
docs        = ["README.md", "docs/"]     # how the system works today
definitions = ["docs/adr/"]              # what SHOULD be built
validations = ["tests/", "e2e/"]         # how correctness is proven
```

Three classes, deliberately separate. `definitions` is the one most repos leave
empty, and Circle will tell you so — a plan built with nothing authoritative to
read rests on inference, and the brief says as much.

### execution — *how do I run this, in isolation?*

```toml
[execution]
compose      = "docker-compose.yml"
app_services = ["api", "web"]
bootstrap    = "mkdir -p data"           # what a fresh clone needs first
up           = "docker compose up --build -d"
down         = "docker compose down"
url          = "http://localhost:8080"
ports_fixed  = true                      # host ports hard-coded in compose
```

`app_services` is the list of containers that hold *your* code. Getting it wrong
corrupts everything downstream, which is why `init` proposes and you confirm.

### quality — *how do I prove it is correct?*

```toml
[quality]
bootstrap = "pip install -e \".[dev]\""  # makes the gates runnable
lint      = "ruff check ."

[quality.test]
unit = "pytest"
e2e  = "playwright test"
```

`bootstrap` is not a gate. It is how the gates become runnable on a clean clone.
Without it, preflight would report a green contract that a stranger cannot
actually run — which is a lie with a checkmark next to it.

---

## Step 4 — Break work into tasks

```bash
circle task create --id feat-1 \
  --title "add a health endpoint" \
  --goal "GET /health returns 200 with the build SHA." \
  --verify-kind unit --verify-gate test:unit \
  --paths "src/**,tests/**"
```

**What this is for.** Two fields make a task more than a prompt:

- `--goal` — one sentence. What does done mean for *this task alone*?
- `--verify-gate` — which declared gate proves it?

Both are required. A task missing either is **not created** — the command exits 2
and the error names the field. That is not strictness for its own sake: a task
without a verification closes on the agent's word, and the status score would be
reporting an opinion.

`--verify-kind` is one of `unit`, `integration`, `e2e`, `manual`.

**`manual` is permitted but never free.** It additionally requires
`--justification` and `--reviewer`, and it lowers the machine-checkable ratio.
Use it for things a command genuinely cannot judge — wording, visual design — not
as an escape from writing a test.

`--paths` is the task's blast radius: the globs it may write to. Declare it per
task, not per plan, so a task that touches `src/mw/` does not silently authorise
edits to `infra/`.

```bash
circle task ready       # what is claimable, with each goal and gate
circle task list        # the whole graph, and the machine-checkable ratio
```

---

## Step 5 — Generate the brief and read it

```bash
circle brief generate
```

**What this is for.** This is the comprehension gate, and it is the reason the
framework exists. It writes `.circle/items/default/brief.md` containing:

- every task's goal, and whether a command or a human proves it
- the blast radius, as a table and a mermaid diagram
- the plan in readiness order
- risks **derived from the plan**, not imagined — a task closing on human
  judgement, an empty definitions registry, a radius that reaches CI or a
  migration

It is around 60 lines. Everything in it is a fact your repository already knows;
nothing is model-written. A plausible-but-false diagram would manufacture
confidence at exactly the moment you are trusting the summary, so the structure
is assembled deterministically or not at all.

The same file doubles as your PR body. GitHub renders the mermaid natively, so
the summary you approved is the one reviewers see.

**Read it properly.** If you cannot answer *what does this touch* and *what could
go wrong* from the brief alone, the plan is not ready — and that is a finding
about the plan, not about the brief.

---

## Step 6 — Approve

```bash
circle brief approve
```

Until you do, every `Edit` and `Write` from Claude Code is denied at the tool
layer. Not discouraged — denied.

Three things worth knowing:

**Self-approval is explicit.** If you wrote the plan, approving it needs
`--allow-self`, and it is recorded as `self_approved`. The person who prompted
the agent approving their own plan is a formality, not comprehension. Set
`[gate] require_second_approver = true` to forbid it outright.

**The approval is bound to the plan.** It covers a hash of every task's goal,
verification, paths and dependencies. Change any of them and the approval voids
itself:

```
circle: the approved brief is stale — the plan changed since approval.
```

That is intended. Regenerate, re-read, re-approve.

**The radius is enforced per write.** A path no task declared is denied, and the
reason lists what *was* approved.

---

## Step 7 — The loop

```bash
circle task claim feat-1
# ... implement ...
circle quality run test:unit
circle task close feat-1
```

`quality run` records each result as an event. `task close` reads those events
and **refuses unless the task's gate passed after its last commit**:

```
circle: feat-1 cannot close
  gate test:unit has no passing run since 2026-09-02T10:31:00Z
  run: circle quality run test:unit
```

A green from before the work proves nothing about the work. This is what moves
the 20-point closure component of the score from claimed to proven.

```bash
circle status            # the score, with every component's evidence
circle score explain     # every signal behind it
circle timeline show     # commits and changed files, from git
```

`circle timeline show` reads git, never the agent's account of what it did. A
commit the agent reported that git does not have is shown as **phantom** rather
than trusted.

---

## Step 8 — With Claude Code

```bash
claude --plugin-dir /path/to/circle/plugin
```

The plugin ships five skills and, more importantly, the hooks that enforce all of
the above. A skill is text the model may ignore; a hook is not.

Why a plugin rather than files in `.claude/`: many repositories gitignore
`.claude/`, so skills committed there do not travel — which breaks the whole
"share the workflow via the repo" premise. Plugin hooks also run the moment the
plugin is enabled, whereas project `settings.json` hooks wait for a workspace
trust dialog.

Start a session with `/circle:start`. It aborts if the contracts are invalid, so
a session cannot begin on a repository that cannot run itself.

---

## Running several agents at once

```bash
claude --worktree feat-1
```

Circle's `WorktreeCreate` hook sets `COMPOSE_PROJECT_NAME`, allocates free host
ports from a ledger shared across every worktree of the repo, and writes a
`docker-compose.override.yml`. Your compose file is untouched.

```bash
circle worktree list     # live port leases
```

Three worktrees run at once with no collisions. Networks and volumes namespace
themselves from the project name. On removal the stack comes down with `-v`, so
no orphan volumes are left behind.

---

## Starting from nothing

Circle assumes a project that already runs. If yours does not yet — no compose
file, no tests, maybe only a requirements document — it does not block you. It
says so and gets out of the way.

```
INCUBATING  nothing to run and nothing to prove yet

This is expected in a new project, and writes are not blocked.
Come back when the project has:
  · something that starts   → set execution.compose and up/down
  · a first test            → set a gate under [quality]
```

**Incubating means nothing is declared and nothing is on disk to declare.** In
that state `circle init` will not write execution commands it cannot honour, the
gate stays open, and preflight exits 0 while still listing the gaps.

It is deliberately narrow. Three things end it:

| Change | What happens |
|---|---|
| A compose file appears | Incubation ends. Declare it, or preflight blocks with the exact line to paste |
| A gate is declared | Incubation ends |
| The contract names a file that does not exist | **Never incubating.** A contract that claims an execution model it cannot honour is a lie, not a young project |

One thing does still block once your project runs: **having something to run and
no way to prove it works.** That is not bureaucracy. Without a declared gate, no
task can name a verification, so no task can be created, so the approval gate
never engages — Circle would be installed and inert. One line is enough to
start:

```toml
[quality.test]
unit = "your test command"
```

**What Circle will not do is choose your stack.** It reads what you have and
writes it down; it does not propose a framework, a directory layout or a compose
file from a requirements document. That is a deliberate boundary — every
spec-driven tool already generates scaffolding, and none of them solves the
problem Circle exists for.

What it *does* do on day zero is register your requirements document as
`definitions`, so the one artifact a new project has is not wasted:

```
✓ definitions   REQUIREMENTS.md → 1
```

`init` looks for `REQUIREMENTS.md`, `PRD.md`, `SPEC.md`, `DESIGN.md` and the
usual `docs/` subdirectories. Re-run `circle init --force` whenever the project
grows something new to detect.

---

## Troubleshooting

**`circle: no .circle/project.toml`** — you are outside the repo, or have not run
`circle init`. Circle resolves the root from your working directory.

**Preflight blocks on a path that exists** — check the glob. `docs` and `docs/`
both work; `docs/*` matches only the immediate children.

**A gate is "not runnable yet"** — the binary is not on `PATH`. Declare
`quality.bootstrap` with the install command; preflight will quote it back.

**Writes are denied and you do not know why** — the reason is in the denial
message. There are exactly three: no approved brief, a stale approval, or a path
outside the radius.

**`task close` refuses** — run the gate. If it passed a while ago, run it again:
it must have passed *after* your last commit.

**A commit shows as `phantom`** — the agent reported a commit git does not have.
This is a known Claude Code failure mode and the timeline surfaces it rather than
trusting it.

**Ports collide across worktrees** — the compose file publishes a port form
Circle could not parse (a range, or udp). `circle worktree provision` reports it
loudly: those ports are not isolated.

---

## What to do when it gets in your way

If preflight blocks something you consider trivial, you can bypass it:

```bash
circle preflight --force --reason "why this is justified"
```

The reason is mandatory, the bypass is recorded, and it **caps the status score
at 60** and blocks PR creation. It is an escape hatch with a cost, not a free
pass — and if you find yourself using it often, that is a finding about the
framework worth reporting.
