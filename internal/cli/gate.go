package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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

	// Contracts are valid. Now the second question: is this path inside the
	// approved blast radius? A brief approved for src/mw/** does not silently
	// authorise edits to infra/.
	if target != "" {
		if reason, blocked := outsideRadius(repo, target); blocked {
			ev := domain.NewEvent(domain.EvGateDeny)
			ev.Reason, ev.Path = reason, target
			_ = events.Append(repo, ev)
			if *hook {
				return deny(e.Stdout, reason)
			}
			fmt.Fprintln(e.Stderr, reason)
			return ExitGate
		}
	}

	_ = toolName
	return ExitOK
}

// outsideRadius enforces an approved brief's declared paths, when one exists.
//
// Absent an approval file the gate stays open: v0.1 gates on preflight, and the
// brief gate arrives with Phase 5. This is written now so the enforcement point
// is single, not so it is active everywhere.
func outsideRadius(repo *contract.Repo, target string) (string, bool) {
	b, err := os.ReadFile(repo.StateDir("approved-radius"))
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(repo.Root, target)
	if err != nil {
		rel = target
	}
	for _, pat := range strings.Fields(string(b)) {
		if ok, _ := filepath.Match(pat, rel); ok {
			return "", false
		}
		// A trailing /** means the whole subtree.
		if strings.HasSuffix(pat, "/**") &&
			strings.HasPrefix(rel, strings.TrimSuffix(pat, "/**")+"/") {
			return "", false
		}
	}
	return fmt.Sprintf("circle: %s is outside the approved blast radius. Record an addendum or regenerate the brief.", rel), true
}
