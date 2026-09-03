package events

import (
	"testing"
	"time"

	"github.com/edwardmontoya/circle/internal/domain"
)

func run(gate string, at time.Time, exit int) domain.Event {
	return domain.Event{Type: domain.EvGateRun, Gate: gate, At: at, ExitCode: exit}
}

var t0 = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

func TestGatePassedSince(t *testing.T) {
	cases := []struct {
		name  string
		evs   []domain.Event
		since time.Time
		want  bool
	}{
		{
			name:  "a pass after the floor counts",
			evs:   []domain.Event{run("lint", t0.Add(time.Hour), 0)},
			since: t0,
			want:  true,
		},
		{
			name: "a pass before the floor does not",
			// The whole point of the rule: green from before the work proves
			// nothing about the work.
			evs:   []domain.Event{run("lint", t0.Add(-time.Hour), 0)},
			since: t0,
			want:  false,
		},
		{
			name: "the most recent run decides, even with an older pass behind it",
			evs: []domain.Event{
				run("lint", t0.Add(time.Hour), 0),
				run("lint", t0.Add(2*time.Hour), 1),
			},
			since: t0,
			want:  false,
		},
		{
			name: "a failure before a later pass does not veto it",
			evs: []domain.Event{
				run("lint", t0.Add(time.Hour), 1),
				run("lint", t0.Add(2*time.Hour), 0),
			},
			since: t0,
			want:  true,
		},
		{
			name:  "another gate's pass does not count",
			evs:   []domain.Event{run("test", t0.Add(time.Hour), 0)},
			since: t0,
			want:  false,
		},
		{
			name:  "no runs at all",
			evs:   nil,
			since: t0,
			want:  false,
		},
		{
			name: "non-gate events are ignored",
			evs: []domain.Event{
				{Type: domain.EvPreflight, At: t0.Add(time.Hour)},
				run("lint", t0.Add(2*time.Hour), 0),
			},
			since: t0,
			want:  true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, got := GatePassedSince(c.evs, "lint", c.since)
			if got != c.want {
				t.Errorf("GatePassedSince = %v, want %v", got, c.want)
			}
		})
	}
}

// The zero floor is what a missing timeline used to produce. Pinned here so the
// consequence is visible: every historical pass satisfies it, which is why the
// caller must refuse rather than pass the zero time in.
func TestGatePassedSinceIsVacuousAtTheZeroTime(t *testing.T) {
	evs := []domain.Event{run("lint", t0.Add(-100*time.Hour), 0)}
	if _, ok := GatePassedSince(evs, "lint", time.Time{}); !ok {
		t.Fatal("expected the zero floor to admit an ancient pass")
	}
}

func TestLastGateRunFindsTheNewest(t *testing.T) {
	evs := []domain.Event{
		run("lint", t0, 0),
		run("lint", t0.Add(time.Hour), 1),
	}
	ev, ok := LastGateRun(evs, "lint")
	if !ok {
		t.Fatal("want a run")
	}
	if ev.ExitCode != 1 {
		t.Errorf("exit = %d, want the newest run's 1", ev.ExitCode)
	}
	if _, ok := LastGateRun(evs, "absent"); ok {
		t.Error("an undeclared gate has no runs")
	}
}

func TestGateSummaryCountsLatestPerGate(t *testing.T) {
	evs := []domain.Event{
		run("lint", t0, 1),
		run("lint", t0.Add(time.Hour), 0), // recovered
		run("test", t0, 0),
		run("test", t0.Add(time.Hour), 1), // regressed
	}
	gates := []domain.GateSpec{{Name: "lint"}, {Name: "test"}, {Name: "never-run"}}
	pass, total := GateSummary(evs, gates)
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if pass != 1 {
		t.Errorf("pass = %d, want 1 (lint only)", pass)
	}
}
