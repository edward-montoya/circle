# Circle

**Teach Claude how to run, test and validate your project once. Commit it. The repo enforces it.**

Every session with a coding agent starts the same way: re-explaining how to run
the app, how to test it, and what "done" means. That knowledge lives in someone's
head or in scrollback — never in the repository — so it cannot be replicated by a
teammate, reused next session, or trusted by the agent.

Circle puts it in the repo as three contracts, then **enforces them with hooks**.
A skill can be ignored; a hook cannot.

```
No file is edited while the contracts are invalid.
No file is edited without a brief a human approved.
No task closes without a passing gate dated after its last commit.
```

"Edited" means the agent's file-editing tools — `Edit`, `Write`, `NotebookEdit`
— which is where a coding agent does essentially all of its writing. It does not
cover a file written by a shell command (`cat > f`, `sed -i`), because the
PreToolUse hook that enforces this is registered against those three tools and
not against `Bash`. Closing that gap means deciding which shell commands count
as writes, and a partial answer presented as a total one is worse than a stated
limit — so the limit is stated. See [#1](#known-gaps).

---

## Status

**Pre-v0.1.** Six of seven phases built and verified against two real
repositories. The cold-start trial that would confirm the central premise has not
been run — see [`docs/TRIAL.md`](docs/TRIAL.md) if you are a participant.

Everything below works today. Nothing below is stable API.

### Known gaps

A framework whose whole claim is enforcement has to be exact about where the
enforcement stops.

1. **Shell writes are not gated.** The PreToolUse hook matches `Edit`, `Write`
   and `NotebookEdit`. A file written by `Bash` — `cat > f`, `sed -i`, a script
   — bypasses both the contract check and the approval check. Deciding which
   commands count as writes is the open design question; until it is answered,
   the guarantee above says "edited" rather than "written".
2. **The approval record is a plain file.** `.circle/items/<item>/approval.json`
   is JSON on disk with no signature. The gate refuses to let the agent's own
   editing tools write it, but anything else on the machine can. It records who
   approved what; it does not prove it.

---

## Install

Circle is a single Go binary with one dependency. It needs Go 1.22+, `git`, and
— if your project uses Compose — `docker`.

```bash
git clone https://github.com/edward-montoya/circle.git
cd circle
go build -o bin/circle ./cmd/circle
```

Put it on your `PATH`:

```bash
export PATH="$PATH:$(pwd)/bin"        # add to ~/.zshrc or ~/.bashrc to persist
circle --version
```

Then check your machine has what Circle needs:

```bash
circle doctor
```

`git` is required. `docker`, `gh` and `jq` are optional — Circle degrades to
local-only without them rather than failing.

---

## Quick start

Five commands, in your own repository. This is the real output from a fresh repo.

### 1. Scaffold the contract

```bash
cd ~/your-project
circle init
```

```
Circle  scanning /Users/you/your-project

  ✓ compose      docker-compose.yml · 1 app · 1 infrastructure
      api                    build: present → app candidate
      db                     image only     → infrastructure
  ✓ quality      1 gate(s) detected
      test:unit    pytest

  ✓ wrote        .circle/project.toml — commit this
```

`init` reads your compose file, `package.json`, `pyproject.toml` or `go.mod` and
writes what it found. **It never invents tooling you do not have** — if there is
no linter, the contract has no lint gate, and preflight will say so rather than
pretend.

Open `.circle/project.toml` and correct anything it got wrong. It is meant to be
hand-edited.

### 2. Validate it

```bash
circle preflight
```

```
PREFLIGHT  /Users/you/your-project

EXECUTION
  ✓ compose       docker-compose.yml · 2 services
  ✓ app_services  api
QUALITY
  ⚠ test:unit     pytest not runnable yet
     └ run: pip install -e ".[dev]"
KNOWLEDGE
  ✓ docs          README.md → 1
  ⚠ definitions   empty — nothing registered
     └ circle knowledge add <path> --as definitions

PREFLIGHT PASSED  3 advisory
```

Every failing line names the file that caused it and the command that fixes it.
`⚠` is advisory and does not block; `✗` does.

Fix what it reports:

```bash
circle knowledge add docs/ --as definitions
circle preflight            # exit 0 when the contracts are honest
```

**Commit `.circle/project.toml` now.** That single file is what makes the
workflow replicable — a teammate who clones the repo gets it.

### 3. Break the work into tasks

Every task must carry a goal and the command that proves it. This is enforced,
not encouraged:

```bash
circle task create --id feat-1 \
  --title "add a health endpoint" \
  --goal "GET /health returns 200 with the build SHA." \
  --verify-kind unit --verify-gate test:unit \
  --paths "src/**,tests/**"
```

```
created feat-1  add a health endpoint
  goal    GET /health returns 200 with the build SHA.
  verify  unit via test:unit
```

Leave out `--goal` and the task is **not created**:

```
✗ goal.statement    empty
   └ one sentence: what does done mean for this task alone?

A task without a goal is a prompt. A task without a verification
closes on the agent's word. Neither is created.
```

`--paths` is the task's blast radius — the only files it may write to.

### 4. Generate the brief, and read it

```bash
circle brief generate
```

Writes `.circle/items/default/brief.md`: the goal of every task, the blast
radius as a mermaid diagram, the plan in readiness order, and risks derived from
the plan rather than imagined. It is about 60 lines and doubles as your PR body —
GitHub renders the diagram natively.

**Read it.** This is the point of the framework.

### 5. Approve, then work

```bash
circle brief approve
```

Until you do, every `Edit` and `Write` from Claude Code is denied:

```
circle: no approved brief. Implementation is blocked until a human reads it.
```

After approving, writes inside the radius are allowed and writes outside it are
not. **Change the plan and the approval voids itself** — the approval is bound to
a hash of every task's goal, verification, paths and dependencies.

Then the loop:

```bash
circle task claim feat-1
# ... implement ...
circle quality run test:unit      # records the result as an event
circle task close feat-1          # refuses unless that gate passed since your last commit
circle status                     # one score, every point traced to a command
```

### When the documents lie

`circle preflight` checks that registered paths exist. `circle knowledge verify`
checks whether they are **true** — a PRD describing a Go service with Postgres,
registered in a Node and Mongo repository, resolves fine and gets a green tick.

That is worse than having no definitions: an empty registry is honest, a stale
one is a lie with a checkmark, and the agent will plan against a system that does
not exist. Detection is deterministic and advisory; the brief strikes the
document through and raises a HIGH risk. See
[When the documents disagree with the code](docs/GETTING-STARTED.md#when-the-documents-disagree-with-the-code).

### Starting from nothing?

Circle assumes a project that already runs. If yours does not yet, it reports
`INCUBATING`, leaves the gate open, and tells you what to come back for — rather
than blocking a new repository from writing its first file. It also registers
your `REQUIREMENTS.md` as `definitions`.

It will not choose your stack. See
[Starting from nothing](docs/GETTING-STARTED.md#starting-from-nothing).

---

## Use it with Claude Code

Install the plugin to get the skills and — more importantly — the enforcement
hooks:

```bash
claude --plugin-dir /path/to/circle/plugin
```

Then in a session:

| Skill | Does |
|---|---|
| `/circle:start` | Loads the contracts. Aborts if they are invalid |
| `/circle:tasks` | Breaks work into tasks, each with a goal and a gate |
| `/circle:brief` | Renders the gate a human must approve |
| `/circle:run` | Brings the stack up from the execution contract |
| `/circle:verify` | Runs the gates and reports what is actually proven |
| `/circle:handoff` | Reports what landed, from git rather than from memory |

The hooks matter more than the skills. They are why the rules hold when the
model would rather not follow them. See [`plugin/README.md`](plugin/README.md).

---

## The three contracts

One file, `.circle/project.toml`, committed with your repo:

| Contract | Answers | Feeds |
|---|---|---|
| **knowledge** | Where is the truth about this project? | Traceability — what the agent actually read |
| **execution** | How do I run this, in isolation? | `/circle:run`, worktree isolation |
| **quality** | How do I prove it is correct? | Every gate, and the status score |

```toml
[knowledge]
docs        = ["README.md", "docs/"]
definitions = ["docs/adr/"]          # what SHOULD be built
validations = ["tests/"]

[execution]
compose      = "docker-compose.yml"
app_services = ["api", "web"]        # containers holding your code
up           = "docker compose up --build -d"
down         = "docker compose down"

[quality]
bootstrap = "pip install -e \".[dev]\""   # makes the gates runnable on a clean clone
lint      = "ruff check ."

[quality.test]
unit = "pytest"
```

---

## Why the status score is a measurement

Four components, fixed weights, arithmetic in Go. **No model contributes to it.**

| Component | Weight | Evidence |
|---|---|---|
| Quality gates | 40 | Recorded exit codes from `circle quality run` |
| Goal criteria | 30 | Tasks whose verification gate passed |
| Task closure | 20 | Tasks closed with a gate dated after their last commit |
| PR & CI | 10 | `gh` |

`circle score explain` shows every signal behind the number. If it ever reports
less than 100% deterministic, something inferred a value — that is a bug.

---

## Running several agents at once

Circle layers Compose isolation over Claude Code's native worktrees:

```bash
claude --worktree feat-1
```

A `WorktreeCreate` hook sets `COMPOSE_PROJECT_NAME`, allocates free host ports
from a shared ledger, and writes a `docker-compose.override.yml`. Your compose
file is never modified. Three worktrees run simultaneously with no collisions;
networks and volumes namespace themselves.

---

## Command reference

Run `circle` with no arguments for the full list. The ones you will use:

```
circle init                     scaffold and detect the contracts
circle doctor                   check git, docker, gh
circle preflight [--explain]    validate. exit 2 when they are not honest
circle knowledge add <p> --as <class>
circle quality list | run [gate]
circle task create | ready | claim | close | list | show
circle brief generate | show | approve | reject | verify
circle timeline show | sync | drift
circle status | score explain
circle serve                    read-only observation app on :7777
```

### Exit codes are the API

`circle gate check` exiting 2 is simultaneously what makes a `PreToolUse` hook
deny a write and what aborts an injected `` !`command` `` inside a skill.

| Code | Meaning |
|---|---|
| 0 | Success / gate passed |
| 2 | **Gate failed** — deny the write, abort the skill |
| 3 | Precondition missing — no `.circle/`, not a git repo |
| 4 | External tool missing or failed |
| 5 | A quality gate exited non-zero |
| 7 | Files touched outside the approved blast radius |

---

## Documentation

| | |
|---|---|
| [`docs/GETTING-STARTED.md`](docs/GETTING-STARTED.md) | The full walkthrough, with what each step is for |
| [`docs/TRIAL.md`](docs/TRIAL.md) | If you were asked to trial this |
| [`circle-ai.prd.md`](circle-ai.prd.md) | The plan of record and every decision, with rationale |
| [`circle-ai.review.md`](circle-ai.review.md) | Platform and competitive validation |
| [`circle-ai.cli.md`](circle-ai.cli.md) | CLI surface and hook wiring |
| [`trials/FINDINGS.md`](trials/FINDINGS.md) | 18 findings from building it, including the bugs |

## Where this sits

Every spec-driven framework — Spec Kit, OpenSpec, Kiro, BMAD — is
*definition-side*: spec → code. None encodes how to **run** the system, how to
**prove** it works, or where the truth lives, and none makes those contracts
executable. That gap is what this fills.

## License

[Apache-2.0](LICENSE).
