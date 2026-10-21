package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/edwardmontoya/circle/internal/brief"
	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/domain"
	"github.com/edwardmontoya/circle/internal/domaindoc"
	"github.com/edwardmontoya/circle/internal/events"
	"github.com/edwardmontoya/circle/internal/session"
	"github.com/edwardmontoya/circle/internal/task"
	"github.com/edwardmontoya/circle/internal/timeline"
)

func init() {
	register(Command{"gate check", "Hook + skill-injection target. Exit 2 denies", runGateCheck})
}

// hookPayload is the subset of the PreToolUse input we read.
//
// cwd, never CLAUDE_PROJECT_DIR: inside a worktree the project dir still points
// at the main checkout while cwd follows Claude, so trusting the former
// validates the wrong repository during exactly the parallel work worktrees
// exist to enable.
type hookPayload struct {
	Cwd       string `json:"cwd"`
	SessionID string `json:"session_id"`
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		FilePath string `json:"file_path"`
	} `json:"tool_input"`
}

// denyOutput is the documented PreToolUse decision shape. Exit 0 with this on
// stdout blocks the call and gives Claude a reason it can act on, which a bare
// exit 2 cannot.
type denyOutput struct {
	HookSpecificOutput struct {
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision"`
		PermissionDecisionReason string `json:"permissionDecisionReason"`
	} `json:"hookSpecificOutput"`
}

func deny(w io.Writer, reason string) int {
	var out denyOutput
	out.HookSpecificOutput.HookEventName = "PreToolUse"
	out.HookSpecificOutput.PermissionDecision = "deny"
	out.HookSpecificOutput.PermissionDecisionReason = reason
	b, _ := json.Marshal(out)
	fmt.Fprintln(w, string(b))
	return ExitOK // the decision is in the payload, not the code
}

func runGateCheck(e Env, args []string) int {
	f := fs("gate check", e)
	skill := f.String("skill", "", "skill being invoked (for the abort message)")
	itemFlag := f.String("item", "", "item to check against; defaults to this session's")
	hook := f.Bool("hook", false, "read a PreToolUse payload from stdin and emit a decision")
	tool := f.String("tool", "", "tool name, when not reading a payload")
	path := f.String("path", "", "target path, when not reading a payload")
	if err := parse(f, args); err != nil {
		return ExitError
	}

	cwd := e.Cwd
	toolName, target := *tool, *path
	sid := ""

	if *hook {
		b, err := io.ReadAll(e.Stdin)
		if err != nil {
			return deny(e.Stdout, "circle: could not read the hook payload")
		}
		var p hookPayload
		if err := json.Unmarshal(b, &p); err != nil {
			return deny(e.Stdout, "circle: hook payload was not valid JSON")
		}
		if p.Cwd == "" {
			// Refusing to guess is the safe failure. A gate that quietly stops
			// gating is worse than no gate: it manufactures confidence at the
			// moment someone is trusting it.
			return deny(e.Stdout, "circle: hook payload carried no cwd; refusing to guess which repo to validate")
		}
		cwd, toolName, target, sid = p.Cwd, p.ToolName, p.ToolInput.FilePath, p.SessionID
	}

	repo, err := contract.Open(cwd)
	if err != nil {
		// Not a Circle repo: nothing to enforce. Allow.
		if *hook {
			return ExitOK
		}
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitPrecond
	}

	res := contract.Validate(repo)
	// A project on day zero has nothing to run and nothing to prove, so there is
	// nothing to enforce. Blocking here would mean a new repository cannot write
	// its first file, and the only escape would be --force from minute one —
	// which teaches the user the gate is noise before it has ever been useful.
	if res.Blocked() && !res.Incubating {
		reason := "circle: preflight is failing, so writes are blocked. Run `circle preflight --explain` and fix the contract."
		if *skill != "" {
			reason = fmt.Sprintf("circle: %s cannot run — the contracts are invalid. Run `circle preflight --explain`.", *skill)
		}
		ev := domain.NewEvent(domain.EvGateDeny)
		ev.Reason, ev.Path = reason, target
		_ = events.Append(repo, ev)

		if *hook {
			return deny(e.Stdout, reason)
		}
		fmt.Fprintln(e.Stderr, reason)
		return ExitGate // aborts an injected !`...` and the whole skill with it
	}

	// Contracts are valid. Now the approval gate: no file is written that a
	// human has not signed off on. This is the constraint the whole framework
	// exists for, so it is checked on every single write rather than trusted
	// once at session start.
	item, src := session.Resolve(repo, *itemFlag, session.ID(sid))
	reason, blocked := approvalGate(repo, item, src, target)
	if b, ok := session.Lookup(repo.Root, session.ID(sid)); ok && b.Decision != "" && b.Item == "" && *itemFlag == "" {
		reason, blocked = decisionGate(repo, b.Decision, target)
	}
	if blocked {
		ev := domain.NewEvent(domain.EvGateDeny)
		ev.Reason, ev.Path, ev.Item, ev.Session = reason, target, item, sid
		_ = events.Append(repo, ev)
		if *hook {
			return deny(e.Stdout, reason)
		}
		fmt.Fprintln(e.Stderr, reason)
		return ExitGate
	}

	_ = toolName
	return ExitOK
}

// approvalGate decides whether this write may proceed for the session's item.
//
// Questions, in order: is the path Circle's own state, does the item have a plan
// at all, is there an approval, is it still bound to the current plan, and does
// the path fall inside what was approved.
func approvalGate(repo *contract.Repo, item string, src session.Source, target string) (string, bool) {
	rel := ""
	if target != "" {
		r, err := filepath.Rel(repo.Root, target)
		if err != nil || strings.HasPrefix(r, "..") {
			r = target
		}
		rel = filepath.ToSlash(r)
	}

	// Circle's own state is writable, so recording a shape or a spec is never
	// blocked by the gate those files feed. The records the gate itself trusts
	// are the exception: those change only through the commands that validate
	// and record them.
	if strings.HasPrefix(rel, contract.Dir+"/") {
		if protectedState(rel) {
			return fmt.Sprintf("circle: %s is written by circle commands, not by hand — "+
				"use `circle task`, `circle brief approve` or `circle review close`.", rel), true
		}
		return "", false
	}

	b, err := brief.Build(repo, item)
	if err != nil {
		return "circle: the plan for item " + item + " could not be read: " + err.Error(), true
	}

	if len(b.Tasks) == 0 {
		if item == task.DefaultItem {
			// A session bound to nothing, in a repository where items have plans,
			// would otherwise write freely into code some approved plan covers.
			if planned := plannedItems(repo); len(planned) > 0 && src == session.FromDefault {
				return fmt.Sprintf("circle: this session is not bound to an item, and %s %s plans. "+
					"Bind one — `circle session bind --item <id>` — or start `claude --worktree <id>`.",
					strings.Join(planned, ", "), plural(len(planned), "has", "have")), true
			}
			// A repository with no tasks has nothing to approve, so the gate stays
			// open — otherwise adopting Circle would block the very edits needed
			// to configure it.
			return "", false
		}
		// A named item with no plan is being shaped. Shaping writes language and
		// decisions — the domain model — and nothing else.
		if domaindoc.IsDomainDoc(rel) {
			return "", false
		}
		return fmt.Sprintf("circle: item %s has no plan yet, so only DOMAIN.md files and ADRs are writable. "+
			"Plan it (`/circle:tasks`) and get the brief approved before writing %s.", item, rel), true
	}

	a, err := brief.LoadApproval(repo, item, b.PlanHash)
	switch {
	case errors.Is(err, brief.ErrNoApproval):
		return fmt.Sprintf("circle: no approved brief for item %s. Implementation is blocked until it is read — "+
			"`circle brief generate` then `circle brief approve`.", item), true
	case err != nil:
		return "circle: the approval record could not be read: " + err.Error(), true
	case !a.Valid:
		// Stale approvals are the quiet failure this guards against: the plan
		// moved, and the signature no longer covers what is about to be written.
		return "circle: the approved brief for item " + item + " is stale — " + a.Reason +
			". Regenerate it and have it re-approved.", true
	}

	if rel == "" || len(a.Radius) == 0 {
		return "", false
	}
	// Domain docs ride along with any approved plan: a new concept is meant to
	// be written down the moment it appears, wherever its DOMAIN.md lives.
	if domain.InRadius(rel, a.Radius) || domaindoc.IsDomainDoc(rel) {
		return "", false
	}
	return fmt.Sprintf(
		"circle: %s is outside item %s's approved blast radius (%s). "+
			"Record an addendum, or widen the plan and have it re-approved.",
		rel, item, strings.Join(a.Radius, " ")), true
}

// decisionGate governs a session working an initiative decision. A decision is
// a question, not a deliverable: its session writes answers, research notes and
// domain language — and code only on a prototype/<name> branch, where it is
// thrown away by design and needs no approval.
func decisionGate(repo *contract.Repo, decision, target string) (string, bool) {
	if target == "" {
		return "", false
	}
	rel, err := filepath.Rel(repo.Root, target)
	if err != nil || strings.HasPrefix(rel, "..") {
		rel = target
	}
	rel = filepath.ToSlash(rel)
	if strings.HasPrefix(rel, contract.Dir+"/") {
		if protectedState(rel) {
			return "circle: " + rel + " is written by circle commands, not by hand.", true
		}
		return "", false
	}
	if domaindoc.IsDomainDoc(rel) {
		return "", false
	}
	if br, _ := timeline.Branch(repo.Root); strings.HasPrefix(strings.TrimPrefix(br, "refs/heads/"), "prototype/") {
		return "", false
	}
	return fmt.Sprintf("circle: this session works decision %s, which produces decisions, not code. "+
		"Prototype on a prototype/<name> branch, or turn the decision into an item once the way is clear.", decision), true
}

// protectedState lists the .circle/ files the gate trusts. Writing them by hand
// would forge the evidence the gate reads.
func protectedState(rel string) bool {
	base := filepath.Base(rel)
	switch base {
	case "approval.json", "approved-radius", "done.json", "tasks.jsonl", "DOMAIN-INDEX.md":
		return true
	}
	return false
}

// plannedItems lists the named items that have tasks.
func plannedItems(repo *contract.Repo) []string {
	ts, _ := task.Load(repo)
	seen := map[string]bool{}
	var out []string
	for _, t := range ts {
		id := task.ItemOf(t)
		if id != task.DefaultItem && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
