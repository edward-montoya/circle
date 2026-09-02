package brief

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/domain"
)

func repo(t *testing.T) *contract.Repo {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, contract.Dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, contract.Dir, "project.toml"),
		[]byte("[project]\nname = \"t\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := contract.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func sampleBrief(author string) domain.Brief {
	tasks := []domain.Task{{
		ID: "a", Title: "one", Paths: []string{"src/**"},
		Goal:   domain.Goal{Statement: "does a"},
		Verify: domain.Verification{Kind: domain.VerifyUnit, Gate: "test:unit"},
	}}
	radius := domain.ComputeRadius(tasks)
	return domain.Brief{
		Item: "default", Title: "t", Author: author,
		Tasks: tasks, Radius: radius,
		PlanHash: domain.ComputePlanHash(tasks, radius),
	}
}

func TestApprovalLifecycle(t *testing.T) {
	r := repo(t)
	b := sampleBrief("author@x")

	if _, err := LoadApproval(r, "default", b.PlanHash); !errors.Is(err, ErrNoApproval) {
		t.Fatalf("before approval: err = %v, want ErrNoApproval", err)
	}

	if _, err := Approve(r, b, "reviewer@x", "", false, false); err != nil {
		t.Fatal(err)
	}
	a, err := LoadApproval(r, "default", b.PlanHash)
	if err != nil {
		t.Fatal(err)
	}
	if !a.Valid || a.SelfApproved {
		t.Fatalf("got %+v, want valid and not self-approved", a)
	}

	// The radius file is the hot path the gate reads on every write.
	body, err := os.ReadFile(r.StateDir("approved-radius"))
	if err != nil {
		t.Fatalf("the gate's radius file was not written: %v", err)
	}
	if !strings.Contains(string(body), "src/**") {
		t.Errorf("radius = %q, want src/**", body)
	}
}

// The failure this guards against is quiet: the plan moves, and a signature
// that no longer covers what is about to be written keeps the gate open.
func TestApprovalGoesStaleWhenThePlanChanges(t *testing.T) {
	r := repo(t)
	b := sampleBrief("author@x")
	if _, err := Approve(r, b, "reviewer@x", "", false, false); err != nil {
		t.Fatal(err)
	}

	changed := sampleBrief("author@x")
	changed.Tasks[0].Goal.Statement = "does something else entirely"
	changed.PlanHash = domain.ComputePlanHash(changed.Tasks, changed.Radius)

	a, err := LoadApproval(r, "default", changed.PlanHash)
	if err != nil {
		t.Fatal(err)
	}
	if a.Valid {
		t.Fatal("an approval must not survive a change to the plan it covered")
	}
	if !strings.Contains(a.Reason, "changed since approval") {
		t.Errorf("reason = %q, want it to name the drift", a.Reason)
	}
}

// Self-approval by the person who prompted the agent is a formality rather than
// comprehension, so it is permitted but never silent (D-29).
func TestSelfApprovalMustBeExplicit(t *testing.T) {
	r := repo(t)
	b := sampleBrief("eng@acme.co")

	_, err := Approve(r, b, "eng@acme.co", "", false, false)
	if err == nil {
		t.Fatal("self-approval must not succeed by default")
	}
	if !strings.Contains(err.Error(), "--allow-self") {
		t.Errorf("err = %v, want it to name the flag", err)
	}

	a, err := Approve(r, b, "eng@acme.co", "", true, false)
	if err != nil {
		t.Fatal(err)
	}
	if !a.SelfApproved {
		t.Error("an explicit self-approval must still be recorded as one")
	}
}

func TestRequireSecondApproverBlocksTheAuthorEntirely(t *testing.T) {
	r := repo(t)
	b := sampleBrief("eng@acme.co")
	// Even --allow-self must not defeat the contract setting.
	if _, err := Approve(r, b, "eng@acme.co", "", true, true); err == nil {
		t.Fatal("require_second_approver must beat --allow-self")
	}
	if _, err := Approve(r, b, "other@acme.co", "", false, true); err != nil {
		t.Fatalf("a second reviewer must still be able to approve: %v", err)
	}
}

func TestApproveRequiresAnIdentity(t *testing.T) {
	if _, err := Approve(repo(t), sampleBrief("a@x"), "", "", false, false); err == nil {
		t.Fatal("an approval with no approver is not a record of anything")
	}
}

func TestRejectClosesTheGateAgain(t *testing.T) {
	r := repo(t)
	b := sampleBrief("author@x")
	if _, err := Approve(r, b, "reviewer@x", "", false, false); err != nil {
		t.Fatal(err)
	}
	if err := Reject(r, b, "reviewer@x", "the radius is too wide"); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadApproval(r, "default", b.PlanHash); !errors.Is(err, ErrNoApproval) {
		t.Error("rejection must remove the approval")
	}
	if _, err := os.Stat(r.StateDir("approved-radius")); !os.IsNotExist(err) {
		t.Error("rejection must clear the radius the gate reads")
	}
	// A rejection is recorded rather than merely absent, so a refused plan is
	// visible to the next session.
	if _, err := os.Stat(filepath.Join(itemDir(r, "default"), "rejection.json")); err != nil {
		t.Error("the rejection itself must be recorded")
	}
}

func TestRejectDemandsAReason(t *testing.T) {
	if err := Reject(repo(t), sampleBrief("a@x"), "r@x", "  "); err == nil {
		t.Fatal("a rejection with no reason cannot be acted on")
	}
}

func TestPendingListsOnlyUnapprovedBriefs(t *testing.T) {
	r := repo(t)
	b := sampleBrief("author@x")
	dir := itemDir(r, "default")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "brief.md"), []byte("# x"), 0o644); err != nil {
		t.Fatal(err)
	}

	pending, err := Pending(r)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0] != "default" {
		t.Fatalf("Pending() = %v, want [default]", pending)
	}

	if _, err := Approve(r, b, "reviewer@x", "", false, false); err != nil {
		t.Fatal(err)
	}
	if pending, _ = Pending(r); len(pending) != 0 {
		t.Fatalf("Pending() = %v, want empty after approval", pending)
	}
}
