// Package task stores the task graph.
//
// JSONL, not beads: beads went entirely to Dolt in early 2026, and a hard
// dependency now means shipping an embedded SQL database to every user of an
// OSS tool (D-4 revised). The file is committed, because the whole premise is
// that state travels with the repository.
package task

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/domain"
)

func file(r *contract.Repo) string { return r.StateDir("tasks.jsonl") }

// Load reads every task, newest write per id winning.
func Load(r *contract.Repo) ([]domain.Task, error) {
	f, err := os.Open(file(r))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	byID := map[string]domain.Task{}
	var order []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		var t domain.Task
		if err := json.Unmarshal(sc.Bytes(), &t); err != nil {
			continue // a torn line must not take the graph down
		}
		if _, seen := byID[t.ID]; !seen {
			order = append(order, t.ID)
		}
		byID[t.ID] = t
	}
	out := make([]domain.Task, 0, len(order))
	for _, id := range order {
		out = append(out, byID[id])
	}
	return out, sc.Err()
}

// Save appends a task record. Append-only so a concurrent writer cannot lose
// another's work; Load resolves the latest.
func Save(r *contract.Repo, t domain.Task) error {
	p := file(r)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(t)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return err
}

// ByID indexes for dependency resolution.
func ByID(ts []domain.Task) map[string]domain.Task {
	m := make(map[string]domain.Task, len(ts))
	for _, t := range ts {
		m[t.ID] = t
	}
	return m
}

// Get finds one task.
func Get(r *contract.Repo, id string) (domain.Task, error) {
	ts, err := Load(r)
	if err != nil {
		return domain.Task{}, err
	}
	for _, t := range ts {
		if t.ID == id {
			return t, nil
		}
	}
	return domain.Task{}, fmt.Errorf("no task %q", id)
}

// Ready lists claimable tasks: open, unclaimed, dependencies closed.
func Ready(ts []domain.Task) []domain.Task {
	idx := ByID(ts)
	var out []domain.Task
	for _, t := range ts {
		if t.State != domain.StateReady {
			continue
		}
		if t.Blocked(idx) {
			continue
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Unverified lists tasks closed without a passing gate behind them.
//
// This should always be empty. A non-empty result means the TaskCompleted hook
// was bypassed, and it is the single cheapest integrity check in the system.
func Unverified(ts []domain.Task) []domain.Task {
	var out []domain.Task
	for _, t := range ts {
		if t.State == domain.StateClosed && t.Verify.Kind.Automated() && t.ClosedBy == "" {
			out = append(out, t)
		}
	}
	return out
}
