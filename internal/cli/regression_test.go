package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/domain"
	"github.com/edwardmontoya/circle/internal/task"
)

func hookOut(t *testing.T, root, payload string) string {
	t.Helper()
	_, out := runStdin(t, root, payload, "gate", "check", "--hook")
	return out
}

func toolPayload(root, sid, tool, field, rel string) string {
	return `{"session_id":"` + sid + `","cwd":"` + root + `","tool_name":"` + tool + `","tool_input":{"` + field + `":"` + filepath.Join(root, rel) + `"}}`
}

func TestBrokenContractIsNotAnOpenGate(t *testing.T) {
	root := newGitRepo(t)
	toml := filepath.Join(root, ".circle", "project.toml")
	_ = os.WriteFile(toml, []byte("[project\nname = \n"), 0o644)
	if out := hookOut(t, root, toolPayload(root, "s", "Write", "file_path", "src/zz.go")); !strings.Contains(out, "does not parse") {
		t.Fatalf("a broken contract must deny writes:\n%s", out)
	}
	if out := hookOut(t, root, toolPayload(root, "s", "Write", "file_path", ".circle/project.toml")); strings.Contains(out, "deny") {
		t.Fatalf("the repair must stay writable:\n%s", out)
	}
}

func TestDefaultBindingIsNotAWayAround(t *testing.T) {
	root := newGitRepo(t)
	planItem(t, root, "alpha", "src/a/**")
	approve(t, root, "alpha")
	t.Setenv("CIRCLE_SESSION", "s9")
	mustRun(t, root, "session", "bind", "--item", "default")
	if out := hookOut(t, root, toolPayload(root, "s9", "Write", "file_path", "src/a/x.go")); !strings.Contains(out, "deny") {
		t.Fatalf("binding to default opened the gate:\n%s", out)
	}
	if code, _ := run(t, root, "session", "bind", "--decision", "made-up"); code != ExitPrecond {
		t.Fatal("binding to a decision that does not exist must be refused")
	}
}

func TestNotebookAndUnknownPayloadsAreGated(t *testing.T) {
	root := newGitRepo(t)
	planItem(t, root, "alpha", "src/a/**")
	approve(t, root, "alpha")
	t.Setenv("CIRCLE_ITEM", "alpha")
	if out := hookOut(t, root, toolPayload(root, "s", "NotebookEdit", "notebook_path", "src/b/x.ipynb")); !strings.Contains(out, "outside") {
		t.Fatalf("a notebook outside the radius must be denied:\n%s", out)
	}
	if out := hookOut(t, root, `{"cwd":"`+root+`","tool_name":"Mystery","tool_input":{}}`); !strings.Contains(out, "named no file") {
		t.Fatalf("a payload with no file must be denied:\n%s", out)
	}
}

func TestProtectedStateIgnoresCaseAndCoversEvidence(t *testing.T) {
	for _, rel := range []string{".circle/items/x/Approval.json", ".circle/runtime/events/events.jsonl", ".circle/items/x/review/review.json"} {
		if !protectedState(rel) {
			t.Errorf("%s should be protected", rel)
		}
	}
	if protectedState(".circle/items/x/spec.md") {
		t.Error("an item's own markdown is the item's to write")
	}
}

func TestClosedTaskIsNotOfferedInAnotherWorktree(t *testing.T) {
	root := newGitRepo(t)
	planItem(t, root, "alpha", "src/a/**")
	approve(t, root, "alpha")
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", "plan")
	wt := filepath.Join(t.TempDir(), "alpha")
	git(t, root, "worktree", "add", "-q", wt, "-b", "alpha")
	mustRun(t, root, "task", "claim", "alpha-1")
	mustRun(t, root, "quality", "run")
	mustRun(t, root, "task", "close", "alpha-1")
	if out := mustRun(t, wt, "task", "ready"); !strings.Contains(out, "no claimable tasks") {
		t.Fatalf("closed in main, offered again in the worktree:\n%s", out)
	}
	if code, out := run(t, wt, "task", "claim", "alpha-1"); code != ExitGate || !strings.Contains(out, "already closed") {
		t.Fatalf("claiming a task closed elsewhere: %d\n%s", code, out)
	}
}

func TestTOMLEditorIsTolerantAndAtomic(t *testing.T) {
	in := "# top\n[ quality.test ]  # gates\nunit = \"a\"\n\n[knowledge]\ndocs = [\n  \"x\",\n  \"y\",\n]\n"
	out := editTOML(in, "quality.test", "integration", `"b"`)
	if strings.Count(out, "quality.test") != 1 || !strings.Contains(out, `integration = "b"`) {
		t.Fatalf("a commented, spaced header must still match:\n%s", out)
	}
	out = editTOML(in, "knowledge", "docs", `["z"]`)
	if strings.Contains(out, `"y"`) || !strings.Contains(out, `docs = ["z"]`) {
		t.Fatalf("a multi-line array must be replaced whole:\n%s", out)
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "project.toml")
	_ = os.WriteFile(p, []byte(in), 0o644)
	if err := setTOMLKey(p, "quality.test", "x", `"unterminated`); err == nil {
		t.Fatal("an edit that does not parse must be refused")
	}
	if b, _ := os.ReadFile(p); string(b) != in {
		t.Fatal("a refused edit must leave the file untouched")
	}
}

func TestSmallCLIRegressions(t *testing.T) {
	root := newGitRepo(t)
	if code, _ := run(t, root, "event", "list", "--limit", "-1"); code != ExitError {
		t.Error("a negative limit must be an error, not a panic")
	}
	mustRun(t, root, "quality", "add", "test:aaa", "false")
	if code, out := run(t, root, "quality", "run", "test:aaa", "test:unit", "--expect", "red", "--fail-fast"); code != ExitOK {
		t.Errorf("red on the first gate with --fail-fast is the expected red:\n%s", out)
	}
	mustRun(t, root, "initiative", "new", "one", "--destination", "d")
	mustRun(t, root, "initiative", "new", "two", "--destination", "d")
	other := newDecision(t, root, "--initiative", "two", "--question", "q2")
	if code, _ := run(t, root, "decision", "new", "--initiative", "one", "--question", "q1", "--blocked-by", other); code != ExitError {
		t.Error("a blocker from another initiative must be refused")
	}
}

func TestLegacyTasksStayWorkable(t *testing.T) {
	root := newGitRepo(t)
	r := mustRepo(t, root)
	legacy := domain.Task{ID: "auth.1", Title: "t", State: domain.StateClaimed, Paths: []string{"src/auth/**"},
		Goal: domain.Goal{Statement: "g"}, Verify: domain.Verification{Kind: domain.VerifyUnit, Gate: "test:unit"}}
	if err := task.Save(r, legacy); err != nil {
		t.Fatal(err)
	}
	ts, _ := task.Load(r)
	if len(task.Ready(ts)) != 1 {
		t.Error("a task claimed before the shared ledger must be claimable again")
	}
	mustRun(t, root, "item", "new", "auth", "--title", "auth")
	if out := mustRun(t, root, "item", "show", "auth"); !strings.Contains(out, "planned") || !strings.Contains(out, "auth.1") {
		t.Errorf("the item's state must count the same tasks its brief does:\n%s", out)
	}
}

func mustRepo(t *testing.T, root string) *contract.Repo {
	t.Helper()
	r, err := contract.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// A gate proves the current tree only if what it runs in was rebuilt from it:
// the bootstrap runs before the gates.
func TestBootstrapRunsBeforeGates(t *testing.T) {
	root := newGitRepo(t)
	toml := filepath.Join(root, ".circle", "project.toml")
	b, _ := os.ReadFile(toml)
	s := strings.Replace(string(b), "[quality.test]", "[quality]\nbootstrap = \"touch built\"\n\n[quality.test]", 1)
	s = strings.Replace(s, `unit = "true"`, `unit = "test -f built"`, 1)
	_ = os.WriteFile(toml, []byte(s), 0o644)
	mustRun(t, root, "quality", "run", "test:unit")
	_ = os.Remove(filepath.Join(root, "built"))
	if code, _ := run(t, root, "quality", "run", "test:unit", "--no-bootstrap"); code != ExitQuality {
		t.Fatal("--no-bootstrap must skip it")
	}
	_ = os.WriteFile(toml, []byte(strings.Replace(s, `"touch built"`, `"false"`, 1)), 0o644)
	if code, out := run(t, root, "quality", "run", "test:unit"); code != ExitQuality || !strings.Contains(out, "bootstrap failed") {
		t.Fatalf("a failed bootstrap must fail the run: %d\n%s", code, out)
	}
}
