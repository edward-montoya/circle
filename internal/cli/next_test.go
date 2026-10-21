package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func nextFor(t *testing.T, root string) string {
	t.Helper()
	return mustRun(t, root, "next")
}

// circle next walks the lifecycle from recorded state alone.
func TestNextFollowsTheLifecycle(t *testing.T) {
	root := newGitRepo(t)
	if out := nextFor(t, root); !strings.Contains(out, "/circle:shape") {
		t.Fatalf("an empty repo starts by shaping:\n%s", out)
	}
	mustRun(t, root, "item", "new", "alpha", "--title", "alpha")
	if out := nextFor(t, root); !strings.Contains(out, "circle session bind --item alpha") {
		t.Fatalf("an unbound session with open items should bind first:\n%s", out)
	}
	t.Setenv("CIRCLE_ITEM", "alpha")
	steps := []struct {
		write, want, boundary string
	}{
		{"", "/circle:shape", "continue"},
		{"shape.md", "/circle:spec", "do not clear or compact"},
		{"spec.md", "/circle:tasks", "continue"},
	}
	for _, s := range steps {
		if s.write != "" {
			_ = os.WriteFile(filepath.Join(root, ".circle", "items", "alpha", s.write), []byte("x\n"), 0o644)
		}
		if out := nextFor(t, root); !strings.Contains(out, s.want) || !strings.Contains(out, s.boundary) {
			t.Fatalf("after %q expected %s / %s:\n%s", s.write, s.want, s.boundary, out)
		}
	}
	mustRun(t, root, "task", "create", "--id", "alpha-1", "--title", "t", "--goal", "g",
		"--verify-kind", "unit", "--verify-gate", "test:unit", "--paths", "src/a/**")
	if out := nextFor(t, root); !strings.Contains(out, "/circle:brief") {
		t.Fatalf("planned → brief:\n%s", out)
	}
	approve(t, root, "alpha")
	if out := nextFor(t, root); !strings.Contains(out, "circle task claim alpha-1") || !strings.Contains(out, "clear") {
		t.Fatalf("approved → implement, claiming the first ready task:\n%s", out)
	}
	mustRun(t, root, "quality", "run")
	mustRun(t, root, "task", "close", "alpha-1")
	if out := nextFor(t, root); !strings.Contains(out, "/circle:review") || !strings.Contains(out, "fresh context") {
		t.Fatalf("reviewing → review in a fresh context:\n%s", out)
	}
}

func TestHandoffWriteIsGroundedInGit(t *testing.T) {
	root := newGitRepo(t)
	planItem(t, root, "alpha", "src/a/**")
	mustRun(t, root, "timeline", "open", "--item", "alpha")
	commitFile(t, root, "src/a/x.go", "package a\n", "alpha-1: start")
	_ = os.WriteFile(filepath.Join(root, "src", "a", "y.go"), []byte("package a\n"), 0o644)
	out := mustRun(t, root, "handoff", "write", "--item", "alpha", "--for", "finish alpha-1")
	for _, want := range []string{"alpha-1: start", "src/a/y.go", "`alpha-1` slice of alpha", "TODO", "finish alpha-1"} {
		if !strings.Contains(out, want) {
			t.Errorf("handoff missing %q:\n%s", want, out)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".circle", "items", "alpha", "handoff.md")); err != nil {
		t.Fatal("handoff.md not written")
	}
}

func TestSkillTelemetryFromHooks(t *testing.T) {
	root := newGitRepo(t)
	runStdin(t, root, `{"session_id":"s","tool_input":{"skill":"circle:tdd"}}`, "event", "record", "--type", "skill.run", "--hook")
	runStdin(t, root, `{"session_id":"s","prompt":"/circle:shape add refunds"}`, "event", "record", "--type", "skill.run", "--hook")
	runStdin(t, root, `{"session_id":"s","prompt":"just a question"}`, "event", "record", "--type", "skill.run", "--hook")
	out := mustRun(t, root, "event", "list", "--all", "--types", "skill.run")
	if !strings.Contains(out, "circle:tdd") || !strings.Contains(out, "circle:shape") || strings.Count(out, "skill.run") != 2 {
		t.Fatalf("expected two skill runs:\n%s", out)
	}
}

func TestPreflightContextNamesTheItemAndSlots(t *testing.T) {
	root := newGitRepo(t)
	toml := filepath.Join(root, ".circle", "project.toml")
	b, _ := os.ReadFile(toml)
	_ = os.WriteFile(toml, append(b, []byte("\n[lifecycle]\ntdd = \"acme:property-tests\"\n")...), 0o644)
	out := mustRun(t, root, "preflight", "--context")
	if !strings.Contains(out, "ITEM       default") || !strings.Contains(out, "tdd=acme:property-tests") || !strings.Contains(out, "grill=circle:grill") {
		t.Fatalf("context block:\n%s", out)
	}
}
