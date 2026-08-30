package task

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/domain"
	"github.com/edwardmontoya/circle/internal/events"
)

func newRepo(t *testing.T) *contract.Repo {
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

func mk(id string, state domain.TaskState, deps ...string) domain.Task {
	return domain.Task{
		ID: id, State: state, DependsOn: deps,
		Goal:   domain.Goal{Statement: "does a thing"},
		Verify: domain.Verification{Kind: domain.VerifyUnit, Gate: "test:unit"},
	}
}

// The store is append-only so two writers cannot lose each other's work; Load
// resolves the latest record per id.
func TestSaveAppendsAndLoadResolvesLatest(t *testing.T) {
	r := newRepo(t)
	if err := Save(r, mk("a", domain.StateReady)); err != nil {
		t.Fatal(err)
	}
	closed := mk("a", domain.StateClosed)
	closed.ClosedBy = "test:unit@now"
	if err := Save(r, closed); err != nil {
		t.Fatal(err)
	}
	got, err := Load(r)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d tasks, want 1 — the latest record should win", len(got))
	}
	if got[0].State != domain.StateClosed {
		t.Errorf("state = %q, want closed", got[0].State)
	}
}

// A torn line from a crash mid-write must not take the whole graph down.
func TestLoadSkipsMalformedLines(t *testing.T) {
	r := newRepo(t)
	_ = Save(r, mk("a", domain.StateReady))
	f, err := os.OpenFile(file(r), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("{not json\n")
	_ = f.Close()
	_ = Save(r, mk("b", domain.StateReady))

	got, err := Load(r)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d tasks, want 2 — a torn line must be skipped, not fatal", len(got))
	}
}

func TestReady(t *testing.T) {
	ts := []domain.Task{
		mk("a", domain.StateClosed),
		mk("b", domain.StateReady, "a"),     // dependency closed -> claimable
		mk("c", domain.StateReady, "b"),     // dependency open   -> blocked
		mk("d", domain.StateClaimed),        // already taken
		mk("e", domain.StateReady, "ghost"), // dependency does not exist
	}
	got := Ready(ts)
	if len(got) != 1 || got[0].ID != "b" {
		ids := make([]string, len(got))
		for i, x := range got {
			ids[i] = x.ID
		}
		t.Fatalf("Ready() = %v, want [b]", ids)
	}
}

// Unverified should always be empty. A non-empty result means the
// TaskCompleted hook was bypassed, and it is the cheapest integrity check
// in the system.
func TestUnverified(t *testing.T) {
	withEvidence := mk("a", domain.StateClosed)
	withEvidence.ClosedBy = "test:unit@now"

	manual := mk("c", domain.StateClosed)
	manual.Verify = domain.Verification{
		Kind: domain.VerifyManual, Justification: "wording", Reviewer: "r",
	}

	ts := []domain.Task{
		withEvidence,
		mk("b", domain.StateClosed), // closed, automated, no evidence
		manual,                      // manual closure needs no gate
		mk("d", domain.StateReady),
	}
	got := Unverified(ts)
	if len(got) != 1 || got[0].ID != "b" {
		t.Fatalf("Unverified() = %v, want [b]", got)
	}
}

// The rule that makes closure a measurement: a gate run predating the work
// proves nothing, so only a pass strictly after the last commit counts.
func TestGatePassedSinceIgnoresStaleGreens(t *testing.T) {
	base := time.Now().UTC()
	work := base.Add(10 * time.Minute)

	stale := domain.NewEvent(domain.EvGateRun)
	stale.Gate, stale.ExitCode, stale.At = "test:unit", 0, base

	fresh := domain.NewEvent(domain.EvGateRun)
	fresh.Gate, fresh.ExitCode, fresh.At = "test:unit", 0, work.Add(time.Minute)

	if _, ok := events.GatePassedSince([]domain.Event{stale}, "test:unit", work); ok {
		t.Error("a gate that passed BEFORE the work must not close a task")
	}
	if _, ok := events.GatePassedSince([]domain.Event{stale, fresh}, "test:unit", work); !ok {
		t.Error("a gate that passed after the work should close the task")
	}
}

func TestGatePassedSinceRespectsTheMostRecentFailure(t *testing.T) {
	work := time.Now().UTC()
	pass := domain.NewEvent(domain.EvGateRun)
	pass.Gate, pass.ExitCode, pass.At = "test:unit", 0, work.Add(time.Minute)
	fail := domain.NewEvent(domain.EvGateRun)
	fail.Gate, fail.ExitCode, fail.At = "test:unit", 1, work.Add(2*time.Minute)

	// A later failure must not be masked by an earlier pass.
	if _, ok := events.GatePassedSince([]domain.Event{pass, fail}, "test:unit", work); ok {
		t.Error("the most recent run failed; the task must not close")
	}
}

func TestEventsRoundTrip(t *testing.T) {
	r := newRepo(t)
	ev := domain.NewEvent(domain.EvGateRun)
	ev.Gate, ev.ExitCode = "test:unit", 0
	if err := events.Append(r, ev); err != nil {
		t.Fatal(err)
	}
	got, err := events.All(r)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Gate != "test:unit" || !got[0].Passed() {
		t.Fatalf("round trip lost the event: %+v", got)
	}
}

func TestEventsAllOnMissingLogIsEmptyNotAnError(t *testing.T) {
	got, err := events.All(newRepo(t))
	if err != nil || len(got) != 0 {
		t.Fatalf("got (%v, %v), want (empty, nil)", got, err)
	}
}

func TestGateSummary(t *testing.T) {
	gates := []domain.GateSpec{{Name: "lint"}, {Name: "test:unit"}}
	mkEv := func(gate string, code int) domain.Event {
		e := domain.NewEvent(domain.EvGateRun)
		e.Gate, e.ExitCode = gate, code
		return e
	}
	// The latest run per gate wins: an earlier failure is superseded by a pass.
	evs := []domain.Event{mkEv("lint", 1), mkEv("lint", 0), mkEv("test:unit", 1)}
	pass, total := events.GateSummary(evs, gates)
	if pass != 1 || total != 2 {
		t.Fatalf("got %d/%d, want 1/2", pass, total)
	}
}
