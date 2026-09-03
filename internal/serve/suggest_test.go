package serve

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/edwardmontoya/circle/internal/brief"
	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/domain"
	"github.com/edwardmontoya/circle/internal/task"
)

const runnable = `[project]
name = "t"
contract_version = 1
[execution]
compose = "docker-compose.yml"
app_services = ["api"]
up = "up"
down = "down"
[quality]
lint = "true"
`

func newRepo(t *testing.T) *contract.Repo {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, contract.Dir), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("docker-compose.yml", "services:\n  api:\n    build: .\n")
	write(filepath.Join(contract.Dir, "project.toml"), runnable)

	r, err := contract.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func addTask(t *testing.T, r *contract.Repo, id string, state domain.TaskState) {
	t.Helper()
	if err := task.Save(r, domain.Task{
		ID: id, Parent: "default", Title: id, State: state,
		Goal:   domain.Goal{Statement: "do the thing"},
		Verify: domain.Verification{Kind: domain.VerifyUnit, Gate: "lint"},
		Paths:  []string{"src/**"},
	}); err != nil {
		t.Fatal(err)
	}
}

func approve(t *testing.T, r *contract.Repo) {
	t.Helper()
	b, err := brief.Build(r, "default")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := brief.Approve(r, b, "reviewer@example.com", "ok", true, false); err != nil {
		t.Fatal(err)
	}
}

// The page is titled "What needs a human". It could not see the one state that
// literally requires one — an approval that no longer covers the plan — and
// rendered "everything green" while the gate denied every write in the repo.
func TestSuggestsReapprovalWhenThePlanMoved(t *testing.T) {
	r := newRepo(t)
	addTask(t, r, "task-1", domain.StateReady)
	approve(t, r)

	if got := approvalSuggestions(r); len(got) != 0 {
		t.Fatalf("a fresh approval needs nothing: %v", got)
	}

	// A new task changes the plan hash, which is exactly what invalidates the
	// approval — by design, and the reason this must be surfaced.
	addTask(t, r, "task-2", domain.StateReady)

	got := approvalSuggestions(r)
	if !slices.Contains(got, "circle brief approve default") {
		t.Fatalf("suggestions = %v, want re-approval once the plan moved", got)
	}
}

func TestSuggestsApprovalWhenNoneExists(t *testing.T) {
	r := newRepo(t)
	addTask(t, r, "task-1", domain.StateReady)

	got := approvalSuggestions(r)
	if !slices.Contains(got, "circle brief generate") {
		t.Fatalf("suggestions = %v, want a brief to be generated", got)
	}
}

func TestSuggestsNothingWhenThereIsNoPlan(t *testing.T) {
	// Adopting Circle must not open with a demand to approve an empty plan.
	if got := approvalSuggestions(newRepo(t)); len(got) != 0 {
		t.Fatalf("suggestions = %v, want none with no tasks", got)
	}
}

// A claimed task is work someone started and stopped. It is not "ready", so the
// ready-loop skipped it and it produced no suggestion at all.
func TestSuggestsClosingAClaimedTask(t *testing.T) {
	r := newRepo(t)
	addTask(t, r, "task-1", domain.StateClaimed)
	approve(t, r)

	got := build(r).Suggest
	if !slices.Contains(got, "circle task close task-1") {
		t.Fatalf("suggestions = %v, want the claimed task surfaced", got)
	}
}

func TestSuggestsClaimingAReadyTask(t *testing.T) {
	r := newRepo(t)
	addTask(t, r, "task-1", domain.StateReady)
	approve(t, r)

	got := build(r).Suggest
	if !slices.Contains(got, "circle task claim task-1") {
		t.Fatalf("suggestions = %v, want the ready task surfaced", got)
	}
}

// The blocking problem must be listed before the ones that cannot be acted on
// until it is fixed.
func TestBlockedContractIsSuggestedFirst(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, contract.Dir), 0o755); err != nil {
		t.Fatal(err)
	}
	// Names a compose file that is not there.
	if err := os.WriteFile(filepath.Join(root, contract.Dir, "project.toml"),
		[]byte(runnable), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := contract.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	addTask(t, r, "task-1", domain.StateReady)

	got := build(r).Suggest
	if len(got) == 0 || got[0] != "circle preflight --explain" {
		t.Fatalf("suggestions = %v, want the blocking contract first", got)
	}
}
