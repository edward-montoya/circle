package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/edwardmontoya/circle/internal/brief"
	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/domain"
	"github.com/edwardmontoya/circle/internal/task"
)

// runnable is a contract with something to run and something to prove, so the
// repository is past incubation and the preflight checks all pass.
const runnable = `[project]
name = "t"
contract_version = 1
[execution]
compose = "docker-compose.yml"
app_services = ["api"]
up = "docker compose up"
down = "docker compose down"
[quality]
lint = "true"
`

const composeFile = `services:
  api:
    build: .
`

// newRepo writes a repository to a temp dir and returns its root.
func newRepo(t *testing.T, contractTOML string, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, contract.Dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, contract.Dir, "project.toml"),
		[]byte(contractTOML), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// gateResult is what the hook actually emits: an exit code plus, when the write
// is refused, a deny decision on stdout.
type gateResult struct {
	code   int
	denied bool
	reason string
}

func runGate(t *testing.T, root, toolPath string) gateResult {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"cwd":        root,
		"tool_name":  "Write",
		"tool_input": map[string]string{"file_path": toolPath},
	})
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	e := Env{Stdout: &stdout, Stderr: &stderr, Stdin: bytes.NewReader(payload), Cwd: root}
	code := runGateCheck(e, []string{"--hook"})

	res := gateResult{code: code}
	if s := strings.TrimSpace(stdout.String()); s != "" {
		var out denyOutput
		if err := json.Unmarshal([]byte(s), &out); err != nil {
			t.Fatalf("stdout was not a decision payload: %q", s)
		}
		res.denied = out.HookSpecificOutput.PermissionDecision == "deny"
		res.reason = out.HookSpecificOutput.PermissionDecisionReason
	}
	return res
}

// seedApprovedTask creates one task and approves the brief covering it, which is
// the only state in which the gate lets an ordinary write through.
func seedApprovedTask(t *testing.T, root string, paths []string) {
	t.Helper()
	repo, err := contract.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	tk := domain.Task{
		ID: "task-1", Parent: "default", Title: "t", State: domain.StateReady,
		Goal:   domain.Goal{Statement: "do the thing"},
		Verify: domain.Verification{Kind: domain.VerifyUnit, Gate: "lint"},
		Paths:  paths,
	}
	if err := task.Save(repo, tk); err != nil {
		t.Fatal(err)
	}
	b, err := brief.Build(repo, "default")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := brief.Approve(repo, b, "reviewer@example.com", "ok", true, false); err != nil {
		t.Fatal(err)
	}
}

func TestGateAllowsOutsideACircleRepo(t *testing.T) {
	// Nothing to enforce is not the same as something to refuse.
	res := runGate(t, t.TempDir(), "main.go")
	if res.denied || res.code != ExitOK {
		t.Fatalf("want allow, got %+v", res)
	}
}

func TestGateRefusesToGuessWithoutCwd(t *testing.T) {
	var stdout, stderr bytes.Buffer
	e := Env{
		Stdout: &stdout, Stderr: &stderr,
		Stdin: strings.NewReader(`{"tool_name":"Write"}`), Cwd: t.TempDir(),
	}
	runGateCheck(e, []string{"--hook"})
	if !strings.Contains(stdout.String(), "refusing to guess") {
		t.Fatalf("a payload with no cwd must deny, got %q", stdout.String())
	}
}

func TestGateDeniesWhileTheContractIsInvalid(t *testing.T) {
	// Declares a compose file that is not there.
	root := newRepo(t, runnable, nil)
	res := runGate(t, root, filepath.Join(root, "main.go"))
	if !res.denied {
		t.Fatal("an invalid contract must block writes")
	}
	if !strings.Contains(res.reason, "preflight is failing") {
		t.Errorf("reason = %q", res.reason)
	}
}

// A young project is excused its missing execution and quality contracts. It is
// not excused a knowledge path that resolves to nothing — that is wrong on day
// zero for the same reason it is wrong later. Regression for incubation being
// used as a veto over unrelated failures.
func TestGateDeniesKnowledgeFailureWhileIncubating(t *testing.T) {
	root := newRepo(t, `[project]
name = "t"
contract_version = 1
[knowledge]
docs = ["docs/nope.md"]
`, nil)
	res := runGate(t, root, filepath.Join(root, "main.go"))
	if !res.denied {
		t.Fatal("a knowledge path resolving to nothing must block, incubating or not")
	}
}

func TestGateAllowsAYoungProjectToWriteItsFirstFile(t *testing.T) {
	root := newRepo(t, "[project]\nname = \"t\"\ncontract_version = 1\n", nil)
	res := runGate(t, root, filepath.Join(root, "main.go"))
	if res.denied {
		t.Fatalf("day zero must not block: %q", res.reason)
	}
}

func TestGateDeniesWithoutAnApprovedBrief(t *testing.T) {
	root := newRepo(t, runnable, map[string]string{"docker-compose.yml": composeFile})
	repo, err := contract.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	tk := domain.Task{
		ID: "task-1", Parent: "default", Title: "t", State: domain.StateReady,
		Goal:   domain.Goal{Statement: "do the thing"},
		Verify: domain.Verification{Kind: domain.VerifyUnit, Gate: "lint"},
	}
	if err := task.Save(repo, tk); err != nil {
		t.Fatal(err)
	}
	res := runGate(t, root, filepath.Join(root, "main.go"))
	if !res.denied || !strings.Contains(res.reason, "no approved brief") {
		t.Fatalf("want a no-approval denial, got %+v", res)
	}
}

func TestGateEnforcesTheApprovedRadius(t *testing.T) {
	root := newRepo(t, runnable, map[string]string{"docker-compose.yml": composeFile})
	seedApprovedTask(t, root, []string{"src/**"})

	if res := runGate(t, root, filepath.Join(root, "src", "a.go")); res.denied {
		t.Fatalf("inside the radius must be allowed: %q", res.reason)
	}
	res := runGate(t, root, filepath.Join(root, "other", "b.go"))
	if !res.denied || !strings.Contains(res.reason, "blast radius") {
		t.Fatalf("outside the radius must be denied, got %+v", res)
	}
}

// The gate is defined by files on disk. If the agent being gated may rewrite
// them, the gate is advisory. Regression for the blanket .circle/ exemption.
func TestGateProtectsItsOwnAuthority(t *testing.T) {
	root := newRepo(t, runnable, map[string]string{"docker-compose.yml": composeFile})
	seedApprovedTask(t, root, []string{"src/**"})

	for _, rel := range []string{
		filepath.Join(contract.Dir, "items", "default", "approval.json"),
		filepath.Join(contract.Dir, "approved-radius"),
		filepath.Join(contract.Dir, "project.toml"),
		filepath.Join(contract.Dir, "tasks.jsonl"),
	} {
		res := runGate(t, root, filepath.Join(root, rel))
		if !res.denied {
			t.Errorf("%s is outside the approved radius and must be denied", rel)
		}
	}
}

// Volatile bookkeeping stays writable: it is gitignored, regenerable and carries
// no integrity claim, so refusing it would only create friction.
func TestGateAllowsRuntimeBookkeeping(t *testing.T) {
	root := newRepo(t, runnable, map[string]string{"docker-compose.yml": composeFile})
	seedApprovedTask(t, root, []string{"src/**"})

	rel := filepath.Join(contract.Dir, "runtime", "events", "events.jsonl")
	if res := runGate(t, root, filepath.Join(root, rel)); res.denied {
		t.Fatalf("runtime state must stay writable: %q", res.reason)
	}
}
