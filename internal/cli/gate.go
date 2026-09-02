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
	"github.com/edwardmontoya/circle/internal/events"
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
	hook := f.Bool("hook", false, "read a PreToolUse payload from stdin and emit a decision")
	tool := f.String("tool", "", "tool name, when not reading a payload")
	path := f.String("path", "", "target path, when not reading a payload")
	if err := f.Parse(args); err != nil {
		return ExitError
	}

	cwd := e.Cwd
	toolName, target := *tool, *path

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
		cwd, toolName, target = p.Cwd, p.ToolName, p.ToolInput.FilePath
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
	if res.Blocked() {
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
	if reason, blocked := approvalGate(repo, target); blocked {
		ev := domain.NewEvent(domain.EvGateDeny)
		ev.Reason, ev.Path = reason, target
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

// approvalGate decides whether this write may proceed.
//
// Three questions, in order: is there an approval at all, is it still bound to
// the current plan, and does the path fall inside what was approved.
//
// A repository with no tasks has nothing to approve, so the gate stays open —
// otherwise adopting Circle would block the very edits needed to configure it.
func approvalGate(repo *contract.Repo, target string) (string, bool) {
	b, err := brief.Build(repo, "default")
	if err != nil || len(b.Tasks) == 0 {
		return "", false
	}

	a, err := brief.LoadApproval(repo, "default", b.PlanHash)
	switch {
	case errors.Is(err, brief.ErrNoApproval):
		return "circle: no approved brief. Implementation is blocked until a human reads it — " +
			"`circle brief generate` then `circle brief approve`.", true
	case err != nil:
		return "circle: the approval record could not be read: " + err.Error(), true
	case !a.Valid:
		// Stale approvals are the quiet failure this guards against: the plan
		// moved, and the signature no longer covers what is about to be written.
		return "circle: the approved brief is stale — " + a.Reason +
			". Regenerate it and have it re-approved.", true
	}

	if target == "" || len(a.Radius) == 0 {
		return "", false
	}
	rel, relErr := filepath.Rel(repo.Root, target)
	if relErr != nil || strings.HasPrefix(rel, "..") {
		rel = target
	}
	// Circle's own state is always writable: recording an event or closing a
	// task must never be blocked by the gate those actions serve.
	if strings.HasPrefix(rel, contract.Dir+string(filepath.Separator)) {
		return "", false
	}
	if domain.InRadius(rel, a.Radius) {
		return "", false
	}
	return fmt.Sprintf(
		"circle: %s is outside the approved blast radius (%s). "+
			"Record an addendum, or widen the plan and have it re-approved.",
		rel, strings.Join(a.Radius, " ")), true
}
