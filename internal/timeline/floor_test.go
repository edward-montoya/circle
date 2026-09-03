package timeline

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

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

func write(t *testing.T, r *contract.Repo, tl domain.Timeline) {
	t.Helper()
	p := filepath.Join(r.StateDir("timelines"), tl.Item+".json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := json.MarshalIndent(tl, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// The zero time is beaten by every gate run ever recorded, so answering it for a
// missing timeline turns "a gate passed since your last commit" into "a gate
// passed at some point" — silently, in exactly the repositories where the
// timeline was never opened.
func TestFloorRefusesWhenNoTimelineExists(t *testing.T) {
	r := repo(t)
	at, err := Floor(r, "default", "task-1")
	if !errors.Is(err, ErrNoTimeline) {
		t.Fatalf("err = %v, want ErrNoTimeline", err)
	}
	if !at.IsZero() {
		t.Errorf("at = %v, want the zero time alongside the error", at)
	}
}

func TestFloorPrefersTheTasksOwnCommits(t *testing.T) {
	r := repo(t)
	base := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	write(t, r, domain.Timeline{
		Item: "default", OpenedAt: base,
		Entries: []domain.Entry{
			{SHA: "a", At: base.Add(1 * time.Hour), State: domain.EntryLive, Task: "task-1"},
			{SHA: "b", At: base.Add(2 * time.Hour), State: domain.EntryLive, Task: "task-2"},
		},
	})
	at, err := Floor(r, "default", "task-1")
	if err != nil {
		t.Fatal(err)
	}
	// The old implementation ignored the task and returned b's timestamp, so
	// closing task-1 waited on a gate run newer than an unrelated commit.
	if want := base.Add(1 * time.Hour); !at.Equal(want) {
		t.Errorf("at = %v, want %v (task-1's own commit)", at, want)
	}
}

func TestFloorFallsBackToTheItemWhenAttributionIsMissing(t *testing.T) {
	r := repo(t)
	base := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	write(t, r, domain.Timeline{
		Item: "default", OpenedAt: base,
		Entries: []domain.Entry{
			{SHA: "a", At: base.Add(1 * time.Hour), State: domain.EntryLive},
			{SHA: "b", At: base.Add(2 * time.Hour), State: domain.EntryLive},
		},
	})
	at, err := Floor(r, "default", "task-1")
	if err != nil {
		t.Fatal(err)
	}
	if want := base.Add(2 * time.Hour); !at.Equal(want) {
		t.Errorf("at = %v, want %v (newest live commit)", at, want)
	}
}

// Nothing committed yet is not nothing to prove: the gate must still have run
// after this stretch of work began.
func TestFloorUsesTheBaselineWhenNothingIsCommitted(t *testing.T) {
	r := repo(t)
	base := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	write(t, r, domain.Timeline{
		Item: "default", OpenedAt: base,
		Entries: []domain.Entry{
			{SHA: "a", At: base, State: domain.EntryBaseline},
		},
	})
	at, err := Floor(r, "default", "task-1")
	if err != nil {
		t.Fatal(err)
	}
	if !at.Equal(base) {
		t.Errorf("at = %v, want the session baseline %v", at, base)
	}
}

// Rewritten entries are superseded, not live; counting them would date a task
// against work that no longer exists in git.
func TestFloorIgnoresRewrittenCommits(t *testing.T) {
	r := repo(t)
	base := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	write(t, r, domain.Timeline{
		Item: "default", OpenedAt: base,
		Entries: []domain.Entry{
			{SHA: "a", At: base.Add(1 * time.Hour), State: domain.EntryLive, Task: "task-1"},
			{SHA: "b", At: base.Add(9 * time.Hour), State: domain.EntryRewritten, Task: "task-1"},
		},
	})
	at, err := Floor(r, "default", "task-1")
	if err != nil {
		t.Fatal(err)
	}
	if want := base.Add(1 * time.Hour); !at.Equal(want) {
		t.Errorf("at = %v, want %v", at, want)
	}
}
