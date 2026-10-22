#!/usr/bin/env bash
# Cold-start trial — the automatable half of Phase 7.
#
#   trials/cold-start/run.sh <app-repo> [runs]
#
# Each run starts from a fresh clone of <app-repo>: no runtime state, no claims,
# no session, no memory. Two phases, both driven by headless Claude Code sessions
# with the Circle plugin and its hooks:
#
#   A. Cold start. An agent that has never seen the repository is asked to run
#      the application and prove it works, from what Circle and the repository
#      give it. Asserted from recorded state, not from the agent's report: every
#      declared gate has a passing run during the session, the stack answers,
#      and nothing is left running.
#   B. Two items in parallel. Two small items are planned and approved (the
#      operator's step, done here by script), then implemented concurrently by
#      two agents in two worktrees. Asserted: both tasks closed, both branches
#      green, both merge into main without a conflict, main green, no blocking
#      trace finding on either item.
#
# Machine facts — a proxy CA directory, say — are not knowledge an agent should
# be handed. Pass them as TRIAL_LOCAL="KEY=VALUE KEY=VALUE"; they are set with
# `circle local set` before the agent starts, as an operator would.
#
# Exits non-zero if any assertion fails in any run. Results land in
# trials/cold-start/results/<timestamp>/.
set -uo pipefail

APP=${1:?usage: run.sh <app-repo> [runs]}
RUNS=${2:-3}
HERE=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$HERE/../.." && pwd)
PLUGIN=$ROOT/plugin
OUT=$HERE/results/$(date -u +%Y%m%dT%H%M%SZ)
mkdir -p "$OUT"
export PATH=$ROOT/bin:$PATH
export CIRCLE_REFUSE=text # a refused skill must say so in the result, not go silent
unset CIRCLE_SESSION CIRCLE_ITEM

command -v circle >/dev/null || { echo "build circle first: go build -o bin/circle ./cmd/circle"; exit 3; }
command -v claude >/dev/null || { echo "claude CLI not found"; exit 3; }
docker info >/dev/null 2>&1 || { echo "docker daemon not reachable"; exit 3; }

failures=0
summary=$OUT/summary.md
printf '| Run | Phase | Check | Result |\n|---|---|---|---|\n' > "$summary"

check() { # run phase name condition...
  local run=$1 phase=$2 name=$3; shift 3
  if "$@" >/dev/null 2>&1; then
    printf '| %s | %s | %s | ✓ |\n' "$run" "$phase" "$name" >> "$summary"
  else
    printf '| %s | %s | %s | ✗ |\n' "$run" "$phase" "$name" >> "$summary"
    failures=$((failures + 1))
  fi
}

agent() { # dir log prompt
  ( cd "$1" && env -u CLAUDE_CODE_SESSION_ID -u CLAUDE_CODE_CHILD_SESSION -u CLAUDE_CODE_REMOTE_SESSION_ID \
      claude -p "$3" --plugin-dir "$PLUGIN" --permission-mode acceptEdits \
      --allowedTools "Bash Read Write Edit MultiEdit Glob Grep Skill Agent TodoWrite" \
      --max-turns 80 --output-format json > "$2" 2> "$2.err" )
}

# all_gates_passed_since <dir> <epoch>: every declared gate has a passing run
# recorded after the given time.
all_gates_passed_since() {
  ( cd "$1" && python3 - "$2" <<'PY'
import json, subprocess, sys, datetime
since = datetime.datetime.fromtimestamp(int(sys.argv[1]), datetime.timezone.utc)
gates = [l.split()[0] for l in subprocess.run(["circle", "quality", "list"], capture_output=True, text=True).stdout.splitlines()
         if l.strip() and not l.startswith("bootstrap")]
passed = set()
for line in open(".circle/runtime/events/events.jsonl"):
    ev = json.loads(line)
    at = datetime.datetime.fromisoformat(ev["at"].replace("Z", "+00:00"))
    if ev.get("type") == "gate.run" and ev.get("exit_code") == 0 and at >= since:
        passed.add(ev.get("gate"))
missing = [g for g in gates if g not in passed]
print("missing:", missing)
sys.exit(1 if missing or not gates else 0)
PY
  )
}

no_containers_left() { [ -z "$(cd "$1" && docker compose ps -q 2>/dev/null)" ]; }

plan_item() { # dir id title goal gate paths
  ( cd "$1" && circle item new "$2" --title "$3" >/dev/null &&
    circle task create --item "$2" --id "$2-1" --title "$3" --goal "$4" \
      --verify-kind unit --verify-gate "$5" --paths "$6" >/dev/null &&
    circle brief generate --item "$2" >/dev/null &&
    circle brief approve --item "$2" --allow-self --note "cold-start trial" >/dev/null )
}

for run in $(seq 1 "$RUNS"); do
  W=$(mktemp -d)
  echo "== run $run in $W"
  git clone -q "$APP" "$W/app" || { echo "clone failed"; exit 3; }
  cd "$W/app" && git config user.email trial@example.com && git config user.name "Cold-start trial"
  for kv in ${TRIAL_LOCAL:-}; do circle local set "${kv%%=*}" "${kv#*=}" >/dev/null; done

  # ---- Phase A: cold start --------------------------------------------------
  start=$(date +%s)
  agent "$W/app" "$OUT/run$run-A.json" "/circle:start

You have never seen this repository, and no one will answer questions. Using only what Circle and the repository give you: bring the application up the way the contract says, check that it answers, run every quality gate, and bring it down again. Report the URL you checked, what it returned, and each gate's result. Change no files."
  check "$run" A "agent finished" python3 -c "import json,sys; sys.exit(0 if json.load(open('$OUT/run$run-A.json')).get('subtype')=='success' else 1)"
  check "$run" A "preflight passes" circle preflight --quiet
  check "$run" A "every gate passed during the session" all_gates_passed_since "$W/app" "$start"
  check "$run" A "nothing left running" no_containers_left "$W/app"
  check "$run" A "no file changed" test -z "$(git status --porcelain -- . ':!.circle')"

  # ---- Phase B: two items in parallel --------------------------------------
  plan_item "$W/app" days-index "Days with mail" \
    "GET /api/days returns the Digest Days that have mail, newest first, as JSON." \
    test:integration "app/server.py,tests/integration/test_days.py"
  plan_item "$W/app" remind-actions "Reminders are Action Items" \
    "A sentence with 'remind' or 'ASAP' in a message body is an Action Item." \
    test:unit "app/digest.py,tests/unit/test_reminders.py"
  git add -A && git commit -qm "trial: plan days-index and remind-actions"
  for id in days-index remind-actions; do git worktree add -q "$W/$id" -b "$id"; done

  prompt="/circle:implement

Headless: no human is present. Work exactly one task — the one this worktree's item has ready — test-first, commit with the task id at the start of the message, run its gate after the last commit, close it, and commit the closure. Stay inside the task's paths."
  agent "$W/days-index" "$OUT/run$run-B1.json" "$prompt" &
  agent "$W/remind-actions" "$OUT/run$run-B2.json" "$prompt" &
  wait

  for id in days-index remind-actions; do
    check "$run" B "$id: task closed" sh -c "cd '$W/$id' && circle item show $id | grep -q reviewing"
    check "$run" B "$id: gates green on its branch" sh -c "cd '$W/$id' && circle quality run"
    ( cd "$W/$id" && git add -A && git commit -qm "$id: trial bookkeeping" ) >/dev/null 2>&1
  done
  check "$run" B "both merge without a conflict" sh -c "cd '$W/app' && git merge -q --no-edit days-index && git merge -q --no-edit remind-actions"
  ( cd "$W/app" && circle domain index >/dev/null && git add -A && git commit -qm "trial: domain index" ) >/dev/null 2>&1
  check "$run" B "main green after both merges" sh -c "cd '$W/app' && circle quality run"
  for id in days-index remind-actions; do
    check "$run" B "$id: no blocking trace finding" sh -c "cd '$W/app' && ! circle review drift --item $id | grep -q blocking"
  done
  ( cd "$W/app" && docker compose down >/dev/null 2>&1 )
  cd "$ROOT"
done

python3 - "$OUT" >> "$summary" <<'PY'
import json, glob, sys
cost = turns = 0
for f in glob.glob(sys.argv[1] + "/*.json"):
    try:
        d = json.load(open(f))
    except Exception:
        continue
    cost += d.get("total_cost_usd") or 0
    turns += d.get("num_turns") or 0
print(f"\n{turns} turns, ${cost:.2f} of model usage.")
PY
cat "$summary"
echo "failures: $failures"
[ "$failures" -eq 0 ]
