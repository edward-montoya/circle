package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/domain"
	"github.com/edwardmontoya/circle/internal/events"
	"github.com/edwardmontoya/circle/internal/task"
	"github.com/edwardmontoya/circle/internal/timeline"
)

func closeTask(t *testing.T, root, id string, args ...string) (int, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	e := Env{Stdout: &stdout, Stderr: &stderr, Stdin: strings.NewReader(""), Cwd: root}
	code := runTaskClose(e, append([]string{id}, args...))
	return code, stdout.String() + stderr.String()
}

func seedTask(t *testing.T, root string, kind domain.VerifyKind) *contract.Repo {
	t.Helper()
	repo, err := contract.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	tk := domain.Task{
		ID: "task-1", Parent: "default", Title: "t", State: domain.StateReady,
		Goal: domain.Goal{Statement: "do the thing"},
		Verify: domain.Verification{
			Kind: kind, Gate: "lint",
			Justification: "no command can prove taste", Reviewer: "someone@example.com",
		},
	}
	if err := task.Save(repo, tk); err != nil {
		t.Fatal(err)
	}
	return repo
}

func seedTimeline(t *testing.T, r *contract.Repo, tl domain.Timeline) {
	t.Helper()
	if err := timeline.Save(r, tl); err != nil {
		t.Fatal(err)
	}
}

// Without a timeline there is no baseline, so "a gate passed since your last
// commit" cannot be evaluated. It used to evaluate to the zero time and admit
// any pass in history; now it refuses.
func TestTaskCloseRefusesWithoutATimeline(t *testing.T) {
	root := newRepo(t, runnable, map[string]string{"docker-compose.yml": composeFile})
	repo := seedTask(t, root, domain.VerifyUnit)

	// A passing run from long before any work, which is exactly what the old
	// zero-time floor would have accepted.
	ev := domain.NewEvent(domain.EvGateRun)
	ev.Gate, ev.ExitCode = "lint", 0
	ev.At = time.Now().UTC().Add(-100 * time.Hour)
	if err := events.Append(repo, ev); err != nil {
		t.Fatal(err)
	}

	code, out := closeTask(t, root, "task-1")
	if code != ExitPrecond {
		t.Fatalf("code = %d, want ExitPrecond(%d); output: %s", code, ExitPrecond, out)
	}
	if !strings.Contains(out, "no timeline") {
		t.Errorf("output should name the missing timeline, got: %s", out)
	}
}

func TestTaskCloseRejectsAGateThatPredatesTheWork(t *testing.T) {
	root := newRepo(t, runnable, map[string]string{"docker-compose.yml": composeFile})
	repo := seedTask(t, root, domain.VerifyUnit)

	base := time.Now().UTC().Add(-10 * time.Hour)
	seedTimeline(t, repo, domain.Timeline{
		Item: "default", OpenedAt: base,
		Entries: []domain.Entry{
			{SHA: "a", At: base.Add(5 * time.Hour), State: domain.EntryLive, Task: "task-1"},
		},
	})
	ev := domain.NewEvent(domain.EvGateRun)
	ev.Gate, ev.ExitCode = "lint", 0
	ev.At = base.Add(time.Hour) // before the commit
	if err := events.Append(repo, ev); err != nil {
		t.Fatal(err)
	}

	code, out := closeTask(t, root, "task-1")
	if code != ExitQuality {
		t.Fatalf("code = %d, want ExitQuality(%d); output: %s", code, ExitQuality, out)
	}
}

func TestTaskCloseAcceptsAGateAfterTheWork(t *testing.T) {
	root := newRepo(t, runnable, map[string]string{"docker-compose.yml": composeFile})
	repo := seedTask(t, root, domain.VerifyUnit)

	base := time.Now().UTC().Add(-10 * time.Hour)
	seedTimeline(t, repo, domain.Timeline{
		Item: "default", OpenedAt: base,
		Entries: []domain.Entry{
			{SHA: "a", At: base.Add(time.Hour), State: domain.EntryLive, Task: "task-1"},
		},
	})
	ev := domain.NewEvent(domain.EvGateRun)
	ev.Gate, ev.ExitCode = "lint", 0
	ev.At = base.Add(2 * time.Hour)
	if err := events.Append(repo, ev); err != nil {
		t.Fatal(err)
	}

	code, out := closeTask(t, root, "task-1")
	if code != ExitOK {
		t.Fatalf("code = %d, want ExitOK; output: %s", code, out)
	}
	ts, err := task.Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	if ts[0].State != domain.StateClosed {
		t.Errorf("state = %q, want closed", ts[0].State)
	}
	if ts[0].ClosedBy == "" {
		t.Error("a closed task must cite the evidence that closed it")
	}
	if u := task.Unverified(ts); len(u) != 0 {
		t.Errorf("closed task should not read as unverified: %+v", u)
	}
}

// A manual task closes on a reviewer's judgement, so there is no gate run to
// cite. It must still say whose judgement it was rather than printing an empty
// "evidence:" and implying machine proof.
func TestTaskCloseNamesTheReviewerForManualVerification(t *testing.T) {
	root := newRepo(t, runnable, map[string]string{"docker-compose.yml": composeFile})
	repo := seedTask(t, root, domain.VerifyManual)

	code, out := closeTask(t, root, "task-1")
	if code != ExitOK {
		t.Fatalf("code = %d, want ExitOK; output: %s", code, out)
	}
	if strings.Contains(out, "evidence: \n") {
		t.Error("empty evidence line implies proof that does not exist")
	}
	ts, err := task.Load(repo)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ts[0].ClosedBy, "someone@example.com") {
		t.Errorf("ClosedBy = %q, want the reviewer named", ts[0].ClosedBy)
	}
}

func TestTaskCloseVerifyOnlyDoesNotClose(t *testing.T) {
	root := newRepo(t, runnable, map[string]string{"docker-compose.yml": composeFile})
	repo := seedTask(t, root, domain.VerifyUnit)

	base := time.Now().UTC().Add(-10 * time.Hour)
	seedTimeline(t, repo, domain.Timeline{
		Item: "default", OpenedAt: base,
		Entries: []domain.Entry{
			{SHA: "a", At: base.Add(time.Hour), State: domain.EntryLive, Task: "task-1"},
		},
	})
	ev := domain.NewEvent(domain.EvGateRun)
	ev.Gate, ev.ExitCode = "lint", 0
	ev.At = base.Add(2 * time.Hour)
	if err := events.Append(repo, ev); err != nil {
		t.Fatal(err)
	}

	code, out := closeTask(t, root, "task-1", "--verify-only")
	if code != ExitOK {
		t.Fatalf("code = %d, want ExitOK; output: %s", code, out)
	}
	ts, _ := task.Load(repo)
	if ts[0].State == domain.StateClosed {
		t.Error("--verify-only must report, not close")
	}
}

// Guards the path resolution the gate and the closure rule share.
func TestClosureFloorReadsTheDefaultTimeline(t *testing.T) {
	root := newRepo(t, runnable, map[string]string{"docker-compose.yml": composeFile})
	repo, err := contract.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := closureFloor(repo, "task-1"); err == nil {
		t.Fatal("no timeline must be an error, not a zero time")
	}
	base := time.Now().UTC().Add(-time.Hour)
	seedTimeline(t, repo, domain.Timeline{Item: "default", OpenedAt: base})
	at, err := closureFloor(repo, "task-1")
	if err != nil {
		t.Fatal(err)
	}
	if !at.Equal(base) {
		t.Errorf("at = %v, want the baseline %v", at, base)
	}
}
