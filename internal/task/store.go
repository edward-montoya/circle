// Package task stores the task graph.
//
// JSONL, not beads: beads went entirely to Dolt in early 2026, and a hard
// dependency now means shipping an embedded SQL database to every user of an
// OSS tool (D-4 revised). The file is committed, because the whole premise is
// that state travels with the repository.
package task

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/domain"
)

// DefaultItem is the item a task belongs to when it names none. Its tasks live
// in the repo-level file every repository had before items existed, so those
// repositories keep working without a migration.
const DefaultItem = "default"

// file is where an item's tasks live.
//
// One file per item, not one per repository: two items planned in parallel
// would otherwise share one plan hash, and changing either would void the
// other's approval. It also keeps parallel branches from appending to the same
// file, which is what makes their merges conflict.
func file(r *contract.Repo, item string) string {
	if item == "" || item == DefaultItem {
		return r.StateDir("tasks.jsonl")
	}
	return r.StateDir("items", item, "tasks.jsonl")
}

// Files lists every task file, the default item's first.
func Files(r *contract.Repo) []string {
	out := []string{file(r, DefaultItem)}
	matches, _ := filepath.Glob(r.StateDir("items", "*", "tasks.jsonl"))
	sort.Strings(matches)
	return append(out, matches...)
}

// Load reads every task across every item, newest write per id winning.
func Load(r *contract.Repo) ([]domain.Task, error) {
	byID := map[string]domain.Task{}
	var order []string
	for _, p := range Files(r) {
		item := DefaultItem
		if p != file(r, DefaultItem) {
			item = filepath.Base(filepath.Dir(p))
		}
		if err := readFile(p, item, byID, &order); err != nil {
			return nil, err
		}
	}
	out := make([]domain.Task, 0, len(order))
	for _, id := range order {
		out = append(out, byID[id])
	}
	return out, nil
}

// LoadItem reads one item's tasks.
func LoadItem(r *contract.Repo, item string) ([]domain.Task, error) {
	all, err := Load(r)
	if err != nil {
		return nil, err
	}
	var out []domain.Task
	for _, t := range all {
		if InItem(t, item) {
			out = append(out, t)
		}
	}
	return out, nil
}

// InItem reports whether a task is part of an item's plan. One definition, used
// by the brief, the gate and the item's derived state alike, so they cannot
// disagree about which tasks an approval covers.
//
// Tasks from before items had files named their item through Parent or an id
// prefix; those still count toward the named item.
func InItem(t domain.Task, item string) bool {
	if ItemOf(t) == item {
		return true
	}
	return item != DefaultItem && t.Item == "" &&
		(t.Parent == item || t.ID == item || strings.HasPrefix(t.ID, item+"."))
}

// ItemOf names the item a task belongs to.
func ItemOf(t domain.Task) string {
	if t.Item == "" {
		return DefaultItem
	}
	return t.Item
}

func readFile(p, item string, byID map[string]domain.Task, order *[]string) error {
	f, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		var t domain.Task
		if err := json.Unmarshal(sc.Bytes(), &t); err != nil {
			continue // a torn line must not take the graph down
		}
		// The file a task is stored in decides its item, so a record moved
		// between files by hand cannot claim to belong somewhere else.
		if item == DefaultItem {
			t.Item = ""
		} else {
			t.Item = item
		}
		if _, seen := byID[t.ID]; !seen {
			*order = append(*order, t.ID)
		}
		byID[t.ID] = t
	}
	return sc.Err()
}

// NewID generates an id for a task created without one: the item, then four
// hex characters of a hash of the title and the time.
//
// Two branches can each create a task while neither sees the other's; a
// sequential "feat-3" collides on merge, a hash does not.
func NewID(item, title string) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%d", item, title, time.Now().UnixNano())))
	prefix := item
	if prefix == "" || prefix == DefaultItem {
		prefix = "t"
	}
	return fmt.Sprintf("%s-%x", prefix, h[:4])
}

// Save appends a task record to its item's file. Append-only so a concurrent
// writer cannot lose another's work; Load resolves the latest.
func Save(r *contract.Repo, t domain.Task) error {
	p := file(r, t.Item)
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
		// StateClaimed is from before claims moved to the shared ledger. The
		// ledger decides who holds a task now, so a legacy claim is claimable.
		if t.State != domain.StateReady && t.State != domain.StateClaimed {
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
