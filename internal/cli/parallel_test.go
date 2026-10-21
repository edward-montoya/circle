package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// run executes a registered command the way Main does, against an explicit cwd.
func run(t *testing.T, cwd string, args ...string) (int, string) {
	t.Helper()
	var out bytes.Buffer
	e := Env{Stdout: &out, Stderr: &out, Stdin: strings.NewReader(""), Cwd: cwd}
	for n := 2; n >= 1; n-- {
		if len(args) < n {
			continue
		}
		name := strings.Join(args[:n], " ")
		for _, c := range commands {
			if c.Name == name {
				return c.Run(e, args[n:]), out.String()
			}
		}
	}
	t.Fatalf("no command %q", args)
	return 0, ""
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// newGitRepo is a committed Circle repository with one passing gate.
func newGitRepo(t *testing.T) string {
	t.Helper()
	t.Setenv("CIRCLE_ITEM", "")
	t.Setenv("CIRCLE_SESSION", "")
	root := t.TempDir()
	git(t, root, "init", "-q", "-b", "main")
	git(t, root, "config", "user.email", "dev@example.com")
	git(t, root, "config", "user.name", "Dev")
	if err := os.MkdirAll(filepath.Join(root, ".circle"), 0o755); err != nil {
		t.Fatal(err)
	}
	contract := "[project]\nname = \"p\"\n\n" +
		"[execution]\ncompose = \"compose.yml\"\napp_services = [\"api\"]\nup = \"true\"\ndown = \"true\"\n\n" +
		"[quality.test]\nunit = \"true\"\n"
	if err := os.WriteFile(filepath.Join(root, ".circle", "project.toml"), []byte(contract), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "compose.yml"), []byte("services:\n  api:\n    build: .\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", ".")
	git(t, root, "commit", "-q", "-m", "init")
	return root
}

func mustRun(t *testing.T, cwd string, args ...string) string {
	t.Helper()
	code, out := run(t, cwd, args...)
	if code != ExitOK {
		t.Fatalf("circle %v exited %d:\n%s", args, code, out)
	}
	return out
}

func planItem(t *testing.T, root, id, paths string) {
	t.Helper()
	mustRun(t, root, "item", "new", id, "--title", "item "+id)
	mustRun(t, root, "task", "create", "--item", id, "--id", id+"-1", "--title", "slice of "+id,
		"--goal", "it works", "--verify-kind", "unit", "--verify-gate", "test:unit", "--paths", paths)
}

func approve(t *testing.T, root, id string) {
	t.Helper()
	mustRun(t, root, "brief", "generate", "--item", id)
	mustRun(t, root, "brief", "approve", "--item", id, "--allow-self")
}

func writeAllowed(t *testing.T, root, item, rel string) (bool, string) {
	t.Helper()
	args := []string{"gate", "check", "--tool", "Write", "--path", filepath.Join(root, rel)}
	if item != "" {
		args = append(args, "--item", item)
	}
	code, out := run(t, root, args...)
	return code == ExitOK, out
}

// Two items planned and approved in one repository must not disturb each
// other. Before items had their own plans, creating beta's task changed the
// one repository-wide plan hash and voided alpha's approval.
func TestTwoItemsKeepTheirOwnApprovals(t *testing.T) {
	root := newGitRepo(t)
	planItem(t, root, "alpha", "src/a/**")
	approve(t, root, "alpha")

	planItem(t, root, "beta", "src/b/**")
	if ok, out := writeAllowed(t, root, "alpha", "src/a/x.go"); !ok {
		t.Fatalf("planning beta voided alpha's approval:\n%s", out)
	}
	if ok, _ := writeAllowed(t, root, "beta", "src/b/x.go"); ok {
		t.Fatal("beta has no approval yet, but its write was allowed")
	}

	approve(t, root, "beta")
	if ok, out := writeAllowed(t, root, "beta", "src/b/x.go"); !ok {
		t.Fatalf("beta approved, write denied:\n%s", out)
	}
	if ok, _ := writeAllowed(t, root, "alpha", "src/b/x.go"); ok {
		t.Fatal("a session on alpha wrote inside beta's radius")
	}
}

// A session bound to nothing must not write code once items have plans: it
// would be writing outside every plan a human approved.
func TestUnboundSessionIsDeniedWhenItemsHavePlans(t *testing.T) {
	root := newGitRepo(t)
	if ok, out := writeAllowed(t, root, "", "src/a/x.go"); !ok {
		t.Fatalf("a repository with no plans must stay open:\n%s", out)
	}
	planItem(t, root, "alpha", "src/a/**")
	approve(t, root, "alpha")
	ok, out := writeAllowed(t, root, "", "src/a/x.go")
	if ok || !strings.Contains(out, "not bound") {
		t.Fatalf("unbound write should be denied with a binding hint, got ok=%v:\n%s", ok, out)
	}
	t.Setenv("CIRCLE_ITEM", "alpha")
	if ok, out := writeAllowed(t, root, "", "src/a/x.go"); !ok {
		t.Fatalf("$CIRCLE_ITEM should bind the session:\n%s", out)
	}
}

// An item being shaped may write its domain model and nothing else.
func TestShapingItemWritesOnlyTheDomainModel(t *testing.T) {
	root := newGitRepo(t)
	mustRun(t, root, "item", "new", "gamma", "--title", "gamma")
	for _, rel := range []string{"src/orders/DOMAIN.md", "docs/adr/0001-x.md", ".circle/items/gamma/shape.md"} {
		if ok, out := writeAllowed(t, root, "gamma", rel); !ok {
			t.Errorf("%s should be writable while shaping:\n%s", rel, out)
		}
	}
	if ok, _ := writeAllowed(t, root, "gamma", "src/orders/order.go"); ok {
		t.Error("code was writable for an item with no plan")
	}
	if ok, _ := writeAllowed(t, root, "gamma", ".circle/items/gamma/approval.json"); ok {
		t.Error("an approval record was writable by hand")
	}
}

// A claim made in one worktree is visible in every other, so two parallel
// implementers cannot take the same task.
func TestClaimsAreSharedAcrossWorktrees(t *testing.T) {
	root := newGitRepo(t)
	planItem(t, root, "alpha", "src/a/**")
	approve(t, root, "alpha")
	git(t, root, "add", ".")
	git(t, root, "commit", "-q", "-m", "plan alpha")

	wt := filepath.Join(t.TempDir(), "alpha")
	git(t, root, "worktree", "add", "-q", wt, "-b", "alpha")

	// The worktree's name binds it to the item, with no flag and no env.
	if out := mustRun(t, wt, "session", "current"); strings.TrimSpace(out) != "alpha" {
		t.Fatalf("worktree named alpha resolved to %q", out)
	}

	mustRun(t, root, "task", "claim", "alpha-1")
	if out := mustRun(t, wt, "task", "ready"); !strings.Contains(out, "no claimable tasks") {
		t.Fatalf("the worktree still offers a task the main checkout holds:\n%s", out)
	}
	if code, out := run(t, wt, "task", "claim", "alpha-1"); code != ExitGate || !strings.Contains(out, "already claimed") {
		t.Fatalf("second claim from another worktree: code %d\n%s", code, out)
	}
	if out := mustRun(t, root, "item", "show", "alpha"); !strings.Contains(out, "building") {
		t.Fatalf("a held task should put the item in building:\n%s", out)
	}

	// Closing releases the claim and, with every task closed, the item moves on.
	mustRun(t, root, "quality", "run", "test:unit")
	mustRun(t, root, "task", "close", "alpha-1")
	if out := mustRun(t, root, "item", "show", "alpha"); !strings.Contains(out, "reviewing") {
		t.Fatalf("every task closed should put the item in reviewing:\n%s", out)
	}
}

// The skill gate: an orchestrator injects `circle item require`, and a wrong
// state exits 2, which aborts the skill.
func TestItemRequireGatesOnDerivedState(t *testing.T) {
	root := newGitRepo(t)
	mustRun(t, root, "item", "new", "delta", "--title", "delta")
	if code, _ := run(t, root, "item", "require", "--item", "delta", "--state", "shaping"); code != ExitOK {
		t.Fatal("a new item should be shaping")
	}
	if code, _ := run(t, root, "item", "require", "--item", "delta", "--state", "approved"); code != ExitGate {
		t.Fatal("require must exit 2 on the wrong state")
	}
	if err := os.WriteFile(filepath.Join(root, ".circle", "items", "delta", "spec.md"), []byte("# spec\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _ := run(t, root, "item", "require", "--item", "delta", "--state", "shaped", "--state", "specified"); code != ExitOK {
		t.Fatal("a spec on disk should make the item specified, and either listed state should pass")
	}
}

// Tasks created without an id get one that cannot collide across branches.
func TestGeneratedTaskIDs(t *testing.T) {
	root := newGitRepo(t)
	mustRun(t, root, "item", "new", "eps", "--title", "eps")
	out := mustRun(t, root, "task", "create", "--item", "eps", "--title", "a", "--goal", "g",
		"--verify-kind", "unit", "--verify-gate", "test:unit", "--paths", "src/**")
	if !strings.Contains(out, "created eps-") {
		t.Fatalf("expected a generated eps-<hex> id:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(root, ".circle", "items", "eps", "tasks.jsonl")); err != nil {
		t.Fatal("the task should be stored in the item's own file")
	}
}

// runStdin is run with a hook payload on stdin.
func runStdin(t *testing.T, cwd, stdin string, args ...string) (int, string) {
	t.Helper()
	var out bytes.Buffer
	e := Env{Stdout: &out, Stderr: &out, Stdin: strings.NewReader(stdin), Cwd: cwd}
	name := strings.Join(args[:2], " ")
	for _, c := range commands {
		if c.Name == name {
			return c.Run(e, args[2:]), out.String()
		}
	}
	t.Fatalf("no command %q", name)
	return 0, ""
}

// The hook path end to end: SessionStart exports the session id for the agent's
// commands, the agent binds, and the write hook resolves the item from the
// session id in its own payload.
func TestSessionBindingDrivesTheWriteHook(t *testing.T) {
	root := newGitRepo(t)
	planItem(t, root, "alpha", "src/a/**")
	approve(t, root, "alpha")
	planItem(t, root, "beta", "src/b/**")
	approve(t, root, "beta")

	envFile := filepath.Join(t.TempDir(), "env")
	t.Setenv("CLAUDE_ENV_FILE", envFile)
	payload := `{"session_id":"s-123","cwd":"` + root + `","source":"startup"}`
	if code, out := runStdin(t, root, payload, "session", "start", "--hook"); code != ExitOK || !strings.Contains(out, "not bound") {
		t.Fatalf("session start: code %d\n%s", code, out)
	}
	if b, _ := os.ReadFile(envFile); !strings.Contains(string(b), "CIRCLE_SESSION=s-123") {
		t.Fatalf("the session id was not exported to the agent's shell: %q", b)
	}

	t.Setenv("CIRCLE_SESSION", "s-123")
	mustRun(t, root, "session", "bind", "--item", "beta")

	hook := func(rel string) string {
		p := `{"session_id":"s-123","cwd":"` + root + `","tool_name":"Write","tool_input":{"file_path":"` +
			filepath.Join(root, rel) + `"}}`
		_, out := runStdin(t, root, p, "gate", "check", "--hook")
		return out
	}
	if out := hook("src/b/x.go"); strings.Contains(out, `"deny"`) {
		t.Fatalf("bound to beta, a write inside beta's radius was denied:\n%s", out)
	}
	if out := hook("src/a/x.go"); !strings.Contains(out, `"deny"`) {
		t.Fatalf("bound to beta, a write into alpha's radius was allowed:\n%s", out)
	}
}

func TestQualityExpectRedAndGreen(t *testing.T) {
	root := newGitRepo(t)
	mustRun(t, root, "quality", "add", "test:failing", "false")
	if code, out := run(t, root, "quality", "run", "test:failing", "--expect", "red", "--task", "x"); code != ExitOK {
		t.Fatalf("a failing gate is the red expected:\n%s", out)
	}
	if code, _ := run(t, root, "quality", "run", "test:unit", "--expect", "red"); code != ExitQuality {
		t.Fatal("a green run when red was expected must fail")
	}
	if code, _ := run(t, root, "quality", "run", "test:unit", "--expect", "green"); code != ExitOK {
		t.Fatal("green as expected")
	}
}

// quality add edits project.toml in place: comments survive, sections are
// created when missing, and an existing key is rewritten rather than duplicated.
func TestQualityAddKeepsTheContractReadable(t *testing.T) {
	root := newGitRepo(t)
	toml := filepath.Join(root, ".circle", "project.toml")
	b, _ := os.ReadFile(toml)
	_ = os.WriteFile(toml, append([]byte("# my comment\n"), b...), 0o644)
	mustRun(t, root, "quality", "add", "lint", "golangci-lint run")
	mustRun(t, root, "quality", "add", "test:unit", "go test ./...")
	mustRun(t, root, "quality", "add", "check-imports", "./scripts/imports.sh")
	got, _ := os.ReadFile(toml)
	s := string(got)
	for _, want := range []string{"# my comment", `lint = "golangci-lint run"`, `unit = "go test ./..."`, "[quality.extra]", `check-imports = "./scripts/imports.sh"`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
	if strings.Count(s, "unit =") != 1 {
		t.Errorf("test:unit duplicated:\n%s", s)
	}
	if out := mustRun(t, root, "quality", "list"); !strings.Contains(out, "check-imports") {
		t.Errorf("new gate not listed:\n%s", out)
	}
}

func TestSpecCheckCommand(t *testing.T) {
	root := newGitRepo(t)
	mustRun(t, root, "item", "new", "zeta", "--title", "zeta")
	spec := "# z\n\n## Problem\np\n\n## Solution\ns\n\n## Behaviours\n1. As a user, I want x, so that y.\n\n" +
		"## Seams\n| Seam | Behaviours | Gate | Misses |\n|---|---|---|---|\n| api | 1 | test:nope | ui |\n\n" +
		"## Decisions\n- d\n\n## Out of scope\n- o\n\n## Risks\n- r\n"
	path := filepath.Join(root, ".circle", "items", "zeta", "spec.md")
	_ = os.WriteFile(path, []byte(spec), 0o644)
	if code, out := run(t, root, "spec", "check", "--item", "zeta"); code != ExitGate || !strings.Contains(out, "test:nope") {
		t.Fatalf("undeclared gate should fail spec check: %d\n%s", code, out)
	}
	_ = os.WriteFile(path, []byte(strings.Replace(spec, "test:nope", "test:unit", 1)), 0o644)
	mustRun(t, root, "spec", "check", "--item", "zeta")
	if out := mustRun(t, root, "item", "show", "zeta", "--with", "spec"); !strings.Contains(out, "===== spec.md =====") || !strings.Contains(out, "specified") {
		t.Fatalf("item show --with spec:\n%s", out)
	}
}
