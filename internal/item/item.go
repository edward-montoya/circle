// Package item is the unit a human approves: a feature, a fix, a refactor.
//
// An item's lifecycle state is derived, never stored. Every state is a fact
// already on disk — a shape file, a spec, tasks, a valid approval, claims,
// closures — so storing it as well would be a second copy that can disagree
// with the first, and the gate would have to decide which to believe.
package item

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/edwardmontoya/circle/internal/brief"
	"github.com/edwardmontoya/circle/internal/claims"
	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/domain"
	"github.com/edwardmontoya/circle/internal/task"
)

// Item is what `circle item new` records.
type Item struct {
	ID         string    `toml:"id"         json:"id"`
	Title      string    `toml:"title"      json:"title"`
	Kind       string    `toml:"kind"       json:"kind"`
	Initiative string    `toml:"initiative" json:"initiative,omitempty"`
	Author     string    `toml:"author"     json:"author,omitempty"`
	Created    time.Time `toml:"created"    json:"created"`
}

// Kinds is the closed set an item may be.
var Kinds = []string{"feature", "fix", "refactor"}

// State is where an item stands in the lifecycle.
type State string

const (
	Shaping   State = "shaping"
	Shaped    State = "shaped"
	Specified State = "specified"
	Planned   State = "planned"
	Approved  State = "approved"
	Building  State = "building"
	Reviewing State = "reviewing"
	Done      State = "done"
)

// States lists the lifecycle in order, for messages and validation.
var States = []State{Shaping, Shaped, Specified, Planned, Approved, Building, Reviewing, Done}

// ValidState reports whether s names a lifecycle state.
func ValidState(s string) bool {
	for _, st := range States {
		if string(st) == s {
			return true
		}
	}
	return false
}

var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// ErrExists is returned when creating an item that already exists.
var ErrExists = errors.New("item already exists")

// Dir is the item's directory under .circle/items/.
func Dir(r *contract.Repo, id string) string { return r.StateDir("items", id) }

func file(r *contract.Repo, id string) string { return filepath.Join(Dir(r, id), "item.toml") }

// New creates an item.
func New(r *contract.Repo, it Item) error {
	if !idPattern.MatchString(it.ID) {
		return fmt.Errorf("item id %q: use lowercase letters, digits and dashes", it.ID)
	}
	if it.ID == task.DefaultItem {
		return fmt.Errorf("%q is reserved for tasks that name no item", it.ID)
	}
	if it.Kind == "" {
		it.Kind = "feature"
	}
	known := false
	for _, k := range Kinds {
		known = known || k == it.Kind
	}
	if !known {
		return fmt.Errorf("item kind %q: one of feature, fix, refactor", it.Kind)
	}
	if _, err := os.Stat(file(r, it.ID)); err == nil {
		return ErrExists
	}
	if err := os.MkdirAll(Dir(r, it.ID), 0o755); err != nil {
		return err
	}
	it.Created = time.Now().UTC()
	f, err := os.Create(file(r, it.ID))
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(it)
}

// Load reads one item. The default item has no item.toml and is synthesised,
// so the commands treat it like any other.
func Load(r *contract.Repo, id string) (Item, error) {
	if id == task.DefaultItem {
		return Item{ID: id, Title: r.Contract.Project.Name, Kind: "feature"}, nil
	}
	var it Item
	if _, err := toml.DecodeFile(file(r, id), &it); err != nil {
		if os.IsNotExist(err) {
			return it, fmt.Errorf("no item %q — create it with `circle item new %s`", id, id)
		}
		return it, err
	}
	it.ID = id
	return it, nil
}

// List returns every created item, plus the default item when it has tasks.
func List(r *contract.Repo) ([]Item, error) {
	var out []Item
	if ts, _ := task.LoadItem(r, task.DefaultItem); len(ts) > 0 {
		def, _ := Load(r, task.DefaultItem)
		out = append(out, def)
	}
	matches, _ := filepath.Glob(r.StateDir("items", "*", "item.toml"))
	sort.Strings(matches)
	for _, m := range matches {
		it, err := Load(r, filepath.Base(filepath.Dir(m)))
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, nil
}

// Status is a derived state and the evidence for it.
type Status struct {
	State  State
	Reason string
	Tasks  []domain.Task
	Closed int
	Held   int
	// ClosedElsewhere counts tasks another worktree has closed whose closure
	// has not reached this checkout's task file yet — known from the ledger.
	ClosedElsewhere int
}

// Derive computes an item's state from what is on disk.
//
// Checked from the end of the lifecycle backwards, so the first fact that holds
// wins: a stale approval puts a half-built item back to planned, because the
// plan it was building against is no longer the one a human signed.
func Derive(r *contract.Repo, id string) Status {
	if exists(Dir(r, id), "done.json") {
		return Status{State: Done, Reason: "review closed"}
	}
	ts, _ := task.LoadItem(r, id)
	if len(ts) > 0 {
		s := Status{Tasks: ts}
		held := claims.All(r.Root)
		for _, t := range ts {
			if t.State == domain.StateClosed {
				s.Closed++
			}
			if c, ok := held[t.ID]; ok && t.State != domain.StateClosed {
				if c.Held() {
					s.Held++
				} else {
					s.ClosedElsewhere++
				}
			}
		}
		b, err := brief.Build(r, id)
		if err != nil {
			s.State, s.Reason = Planned, err.Error()
			return s
		}
		a, err := brief.LoadApproval(r, id, b.PlanHash)
		switch {
		case errors.Is(err, brief.ErrNoApproval):
			s.State, s.Reason = Planned, "waiting on approval"
		case err != nil:
			s.State, s.Reason = Planned, err.Error()
		case !a.Valid:
			s.State, s.Reason = Planned, "approval is stale: "+a.Reason
		case s.Closed == len(ts):
			s.State, s.Reason = Reviewing, "every task closed"
		case s.Closed > 0 || s.Held > 0:
			s.State = Building
			s.Reason = fmt.Sprintf("%d of %d closed, %d held", s.Closed, len(ts), s.Held)
		default:
			s.State, s.Reason = Approved, "approved by "+a.Approver
		}
		return s
	}
	switch {
	case exists(Dir(r, id), "spec.md"):
		return Status{State: Specified, Reason: "spec.md written, no tasks yet"}
	case exists(Dir(r, id), "shape.md"):
		return Status{State: Shaped, Reason: "shape.md written, no spec yet"}
	}
	return Status{State: Shaping, Reason: "nothing written yet"}
}

func exists(parts ...string) bool {
	_, err := os.Stat(filepath.Join(parts...))
	return err == nil
}
