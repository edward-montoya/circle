# The cold-start trial

You have been asked to try Circle on a repository you have never seen. This page
is the whole protocol. It takes about an hour.

**Read this page and nothing else.** In particular, do not read
`docs/GETTING-STARTED.md` — the point of the trial is to find out whether the
repository can teach you what you need, and reading the manual first destroys
the measurement.

---

## What is being tested

One claim:

> A developer who has never worked on a repository can clone it, run it, test it,
> and land a correct change **without asking anyone how the project works**.

Not whether you like the tool. Not whether the output is pretty. Only whether the
repository told you enough.

**A failed trial is a successful trial.** If you get stuck, that is the result —
it is more valuable than a smooth run, because it names something real. Do not
push through by guessing, and do not be polite about it.

---

## The rules

1. **You may not ask anyone how to run or test the project.** Not the person who
   sent you here, not a teammate, not a chat. If you need to ask, **stop** and
   write down the question. That question is the finding.
2. You may use Claude Code, any documentation inside the repository, and the
   internet for general knowledge (language, framework, Docker).
3. Work as you normally would otherwise.
4. **Start a stopwatch.** You will be asked for three timings.

---

## Setup

You will be given: a repository URL, and one task to complete.

```bash
git clone <the repository you were given>
cd <it>

# Build the circle binary
git clone https://github.com/edward-montoya/circle.git /tmp/circle
cd /tmp/circle && go build -o bin/circle ./cmd/circle
export PATH="$PATH:/tmp/circle/bin"

cd -   # back to the repository under trial
```

Then start. `circle` has a `--help`, and `circle` on its own lists every command.

---

## What to record

Copy this into a file as you go. Fill it in **while** you work, not afterwards —
reconstructed timings are guesses.

```
Participant:
Repository:
Date:

TIMINGS
  Clone → app running locally:        ___ min
  Clone → test suite green:           ___ min
  Clone → change ready to commit:     ___ min

QUESTIONS I COULD NOT ANSWER FROM THE REPOSITORY
  (every time you wanted to ask a human, write it here — this is the
   most important field on the page)
  1.
  2.

WHERE I LOOKED FIRST
  (README? .circle/project.toml? the code? Claude Code? something else?)

THE GATE
  Was a write ever denied?              yes / no
  Did the denial message tell you what to do?   yes / no
  Was it helpful or obstructive?        helpful / obstructive / both
  Did you try to work around it?        yes / no    — how?

THE BRIEF
  Did you read .circle/items/*/brief.md before approving?   yes / no
  Could you answer, from the brief alone:
    what does this change touch?        yes / partly / no
    what could go wrong?                yes / partly / no
  How long did reading it take?         ___ min

OUTCOME
  Change complete and tests passing?    yes / no
  If no — where exactly did you stop?

ANYTHING THAT WAS WRONG, CONFUSING, OR ANNOYING
  (be blunt; polite feedback is not useful here)
```

---

## If the repository has a `CLAUDE.md`

Some trial repositories carry a large `CLAUDE.md` alongside the Circle contract.
That is deliberate: we are testing whether Circle adds anything a good
`CLAUDE.md` does not already provide.

Please note **which one you actually used** when you needed to know something.
Honestly. "I ignored the contract and read CLAUDE.md" is a genuinely useful
answer and will not upset anyone.

---

## When you are done

Send back the filled-in record above. That is all — no summary needed, no
suggestions required.

If you got stuck and stopped early, send it anyway. Especially then.

---

## For whoever is running the trial

Do not coach. The moment you explain something, that participant's run is spent
and cannot be recovered.

Run the same repository first for every participant; ordering effects are real,
and consistent ordering at least makes them uniform.

The success signal is **2 of 3 participants completing a correct change with zero
out-of-band questions**. Below that, the premise is not carrying its weight yet
and no amount of additional building fixes it.

Four measurements the trial exists to produce, beyond pass/fail:

1. **The real machine-checkable ratio**, against the assumed ≥70% target.
2. **Time to hand-author or correct a contract** — the case for better detection.
3. **How many participants the workspace-trust dialog silently disarms**, leaving
   them with no enforcement and no idea.
4. **Whether `CLAUDE.md` or the contract was used** on the repository that has
   both. This is the most important number on the page.

A scripted cold start (`claude -p` in a clean container) is a useful smoke test
but **is not this trial**: `-p` skips the trust dialog, so it cannot detect
finding 3, and no script can report a question it wanted to ask a human.
