// Package events is the append-only record everything else reads.
//
// The status score, the timeline's attribution and the gate's memory all come
// from here. A number with no event behind it is an inference, and the framework
// does not report inferences.
package events

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/domain"
)

// file lives under runtime/ because it is regenerable in principle and enormous
// in practice. Attribution that is NOT regenerable — approvals, task closures —
// is written to the durable side by its own command.
func file(r *contract.Repo) string {
	return r.RuntimeDir("events", "events.jsonl")
}

// Append writes one event. Callers ignore the error on purpose in hot paths:
// failing to record must never fail the thing being recorded.
func Append(r *contract.Repo, ev domain.Event) error {
	p := file(r)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return err
}

// All reads the whole log, oldest first. Malformed lines are skipped rather
// than fatal: a truncated write during a crash must not take the history down.
func All(r *contract.Repo) ([]domain.Event, error) {
	f, err := os.Open(file(r))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var out []domain.Event
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		var ev domain.Event
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			continue
		}
		out = append(out, ev)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out, sc.Err()
}

// LastGateRun finds the most recent execution of a named gate.
//
// This is what closes a task: the TaskCompleted hook needs to know not just
// that the gate passed, but that it passed *after* the work — a stale green
// from before the last commit proves nothing.
func LastGateRun(evs []domain.Event, gate string) (domain.Event, bool) {
	for i := len(evs) - 1; i >= 0; i-- {
		if evs[i].Type == domain.EvGateRun && evs[i].Gate == gate {
			return evs[i], true
		}
	}
	return domain.Event{}, false
}

// GatePassedSince reports whether a gate has a passing run strictly after t.
func GatePassedSince(evs []domain.Event, gate string, t time.Time) (domain.Event, bool) {
	for i := len(evs) - 1; i >= 0; i-- {
		ev := evs[i]
		if ev.Type == domain.EvGateRun && ev.Gate == gate && ev.At.After(t) {
			if ev.ExitCode == 0 {
				return ev, true
			}
			return ev, false // most recent run failed; do not look further back
		}
	}
	return domain.Event{}, false
}

// GateSummary counts the latest result per gate.
func GateSummary(evs []domain.Event, gates []domain.GateSpec) (pass, total int) {
	latest := map[string]domain.Event{}
	for _, ev := range evs {
		if ev.Type == domain.EvGateRun {
			latest[ev.Gate] = ev
		}
	}
	for _, g := range gates {
		total++
		if ev, ok := latest[g.Name]; ok && ev.ExitCode == 0 {
			pass++
		}
	}
	return
}
