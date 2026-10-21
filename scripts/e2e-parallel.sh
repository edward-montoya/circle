#!/usr/bin/env bash
# End-to-end: two items in parallel worktrees, driven with the JSON payloads
# Claude Code sends to hooks, from plan to done to a clean merge into main.
#
#   go build -o bin/circle ./cmd/circle && scripts/e2e-parallel.sh bin
#
# Exits non-zero at the first step that does not behave.
set -euo pipefail
export PATH="$(cd "${1:-bin}" && pwd):$PATH"
unset CIRCLE_SESSION CIRCLE_ITEM
W=$(mktemp -d); cd "$W"
step() { printf '\n\033[1m== %s\033[0m\n' "$*"; }
expect_fail() { if "$@" >/tmp/e2e.out 2>&1; then echo "UNEXPECTED SUCCESS: $*"; cat /tmp/e2e.out; exit 1; else echo "refused as expected: $*"; fi; }
hook_write() { # session file
  printf '{"session_id":"%s","cwd":"%s","tool_name":"Write","tool_input":{"file_path":"%s"}}' "$1" "$PWD" "$PWD/$2" | circle gate check --hook; }

git init -q -b main repo && cd repo
git config user.email dev@example.com; git config user.name Dev
mkdir -p src && printf 'services:\n  api:\n    build: .\n' > compose.yml
printf '# Shop\n' > README.md
circle init >/dev/null
cat > .circle/project.toml <<'T'
[project]
name = "shop"

[knowledge]
docs = ["README.md"]

[execution]
compose = "compose.yml"
app_services = ["api"]
up = "true"
down = "true"
T
circle quality add test:unit "true" >/dev/null
circle preflight --quiet && echo "preflight ok"
git add -A && git commit -qm "adopt circle"

step "two items, planned and approved independently"
circle item new orders --title "Cancel orders" >/dev/null
circle item new billing --title "Usage billing" >/dev/null
circle task create --item orders --id orders-1 --title "cancel endpoint" --goal "an unshipped order can be cancelled" --verify-kind unit --verify-gate test:unit --paths "src/orders/**" >/dev/null
circle task create --item billing --id billing-1 --title "meter usage" --goal "usage is metered per account" --verify-kind unit --verify-gate test:unit --paths "src/billing/**" >/dev/null
circle brief generate --item orders >/dev/null && circle brief approve --item orders --allow-self >/dev/null
circle brief generate --item billing >/dev/null && circle brief approve --item billing --allow-self >/dev/null
circle item list
git add -A && git commit -qm "plan orders and billing"

step "a worktree per item binds itself"
git worktree add -q ../orders -b orders
git worktree add -q ../billing -b billing
(cd ../orders && echo "orders worktree → $(circle session current)")
(cd ../billing && echo "billing worktree → $(circle session current)")

step "SessionStart exports the session id; the write hook enforces each item's own radius"
export CLAUDE_ENV_FILE=$(mktemp)
(cd ../orders && printf '{"session_id":"s-orders","cwd":"%s","source":"startup"}' "$PWD" | circle session start --hook)
grep -q "CIRCLE_SESSION=s-orders" "$CLAUDE_ENV_FILE" && echo "env file carries the session id"
(cd ../orders && out=$(hook_write s-orders src/orders/cancel.go); [ -z "$out" ] && echo "orders → src/orders allowed")
(cd ../orders && hook_write s-orders src/billing/meter.go | grep -q deny && echo "orders → src/billing denied")
(cd ../billing && out=$(hook_write s-billing src/billing/meter.go); [ -z "$out" ] && echo "billing → src/billing allowed")

step "claims are visible across worktrees"
(cd ../orders && circle task claim orders-1)
(cd ../billing && expect_fail circle task claim orders-1)
(cd ../billing && circle task claim billing-1)

step "a new concept is fed back at the write that adds it"
cd ../orders
mkdir -p src/orders && printf 'package orders\n\ntype Cancellation struct{}\n' > src/orders/cancel.go
printf '{"cwd":"%s","tool_name":"Write","tool_input":{"file_path":"%s"}}' "$PWD" "$PWD/src/orders/cancel.go" | circle domain sync --hook | grep -q '"decision":"block"' && echo "Cancellation flagged"
printf '# Orders\n\n## Language\n\n**Cancellation**:\nAn order withdrawn before it ships.\n\n## Modules\n\n- `src/orders`\n' > src/orders/DOMAIN.md
out=$(printf '{"cwd":"%s","tool_name":"Write","tool_input":{"file_path":"%s"}}' "$PWD" "$PWD/src/orders/cancel.go" | circle domain sync --hook)
[ -z "$out" ] && echo "documented: no longer flagged"
grep -q "Cancellation" .circle/DOMAIN-INDEX.md && echo "index lists the term"

step "build, close, review, done — in the orders worktree"
circle timeline open --item orders >/dev/null
git add -A && git commit -qm "orders-1: cancellation"
circle quality run --task orders-1 --expect green >/dev/null
circle task close orders-1
circle item show orders
printf -- '- [nit] none worth blocking\n' > /tmp/spec.md; cp /tmp/spec.md /tmp/std.md
circle review record --axis spec --file /tmp/spec.md >/dev/null
circle review record --axis standards --file /tmp/std.md >/dev/null
circle review close
circle next

step "billing is untouched by all of it"
cd ../billing && circle item show billing
circle next
mkdir -p src/billing
printf 'package billing

type Meter struct{}
' > src/billing/meter.go
printf '# Billing

## Language

**Meter**:
Counts an account usage.

## Modules

- `src/billing`
' > src/billing/DOMAIN.md
circle domain index >/dev/null
circle timeline open --item billing >/dev/null
git add -A && git commit -qm "billing-1: meter"
circle quality run >/dev/null && circle task close billing-1 >/dev/null
circle review record --axis spec --file /tmp/spec.md >/dev/null
circle review record --axis standards --file /tmp/std.md >/dev/null
circle review close
git add -A && git commit -qm "billing: done"
cd ../orders && circle domain index >/dev/null && git add -A && git commit -qm "orders: done"

step "both branches merge into main without a conflict"
cd ../repo
git merge -q --no-edit orders
git merge -q --no-edit billing && echo "merged orders and billing cleanly"
circle item list
circle domain index --check || { circle domain index; echo "index regenerated after the merge"; }
circle domain index --check
grep -c "^| \*\*" .circle/DOMAIN-INDEX.md | xargs echo "domains in the merged index:"
circle preflight --quiet && echo "preflight ok on main"
echo; echo "E2E PASSED"
