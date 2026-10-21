// Package initiative charts work too big for one session: a destination, and
// the decisions that stand between here and there, resolved one per session.
//
// A decision is a question, not a deliverable. The map is done when nothing is
// left to decide; it then produces items, which go through the ordinary flow
// and the ordinary gate.
package initiative

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/edwardmontoya/circle/internal/contract"
)

// Initiative is one charted effort.
type Initiative struct {
	ID          string    `toml:"id"`
	Title       string    `toml:"title"`
	Destination string    `toml:"destination"`
	Author      string    `toml:"author,omitempty"`
	Created     time.Time `toml:"created"`
}

// Type is how a decision gets resolved. Closed set: the model picks from it.
type Type string

const (
	Research  Type = "research"  // AFK: read primary sources
	Prototype Type = "prototype" // HITL: make something concrete to react to
	Grill     Type = "grill"     // HITL: conversation, the default
	Work      Type = "task"      // either: work that must happen before deciding
)

// Types lists the valid decision types.
var Types = []Type{Research, Prototype, Grill, Work}

// State is where a decision stands.
type State string

const (
	Open       State = "open"
	Resolved   State = "resolved"
	OutOfScope State = "out-of-scope"
)

// Decision is one question on the map.
type Decision struct {
	ID         string    `toml:"id"`
	Initiative string    `toml:"initiative"`
	Title      string    `toml:"title"`
	Question   string    `toml:"question"`
	Type       Type      `toml:"type"`
	BlockedBy  []string  `toml:"blocked_by,omitempty"`
	State      State     `toml:"state"`
	Gist       string    `toml:"gist,omitempty"`
	Why        string    `toml:"why,omitempty"` // for out of scope
	Created    time.Time `toml:"created"`
	Closed     time.Time `toml:"closed,omitempty"`
}

var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// ErrExists is returned when creating an initiative that exists.
var ErrExists = errors.New("initiative already exists")

// Dir is the initiative's directory.
func Dir(r *contract.Repo, id string) string { return r.StateDir("initiatives", id) }

func decisionDir(r *contract.Repo, id string) string { return filepath.Join(Dir(r, id), "decisions") }

// New creates an initiative and an empty map.md for the parts a human writes.
func New(r *contract.Repo, in Initiative) error {
	if !idPattern.MatchString(in.ID) {
		return fmt.Errorf("initiative id %q: use lowercase letters, digits and dashes", in.ID)
	}
	if strings.TrimSpace(in.Destination) == "" {
		return errors.New("an initiative needs a destination: what reaching the end of the map looks like")
	}
	if _, err := os.Stat(Dir(r, in.ID)); err == nil {
		return ErrExists
	}
	if err := os.MkdirAll(decisionDir(r, in.ID), 0o755); err != nil {
		return err
	}
	in.Created = time.Now().UTC()
	if err := writeTOML(filepath.Join(Dir(r, in.ID), "initiative.toml"), in); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(Dir(r, in.ID), "map.md"), []byte(mapTemplate), 0o644)
}

const mapTemplate = `## Notes

<!-- the domain; which skills every session should use; standing preferences -->

## Not yet specified

<!-- in scope, but too dim to state as a question yet -->

## Out of scope

<!-- ruled beyond the destination; never graduates. Filled by circle decision scope-out -->
`

// Load reads one initiative.
func Load(r *contract.Repo, id string) (Initiative, error) {
	var in Initiative
	if _, err := toml.DecodeFile(filepath.Join(Dir(r, id), "initiative.toml"), &in); err != nil {
		if os.IsNotExist(err) {
			return in, fmt.Errorf("no initiative %q", id)
		}
		return in, err
	}
	return in, nil
}

// List returns every initiative, by id.
func List(r *contract.Repo) ([]Initiative, error) {
	matches, _ := filepath.Glob(r.StateDir("initiatives", "*", "initiative.toml"))
	sort.Strings(matches)
	var out []Initiative
	for _, m := range matches {
		in, err := Load(r, filepath.Base(filepath.Dir(m)))
		if err != nil {
			return nil, err
		}
		out = append(out, in)
	}
	return out, nil
}

// NewDecision adds a question to the map. Its id is a hash, so two sessions
// charting on two branches cannot mint the same one.
func NewDecision(r *contract.Repo, d Decision) (Decision, error) {
	if _, err := Load(r, d.Initiative); err != nil {
		return d, err
	}
	valid := false
	for _, t := range Types {
		valid = valid || t == d.Type
	}
	if !valid {
		return d, fmt.Errorf("decision type %q: one of research, prototype, grill, task", d.Type)
	}
	if strings.TrimSpace(d.Question) == "" {
		return d, errors.New("a decision needs its question")
	}
	if d.Title == "" {
		d.Title = d.Question
	}
	for _, b := range d.BlockedBy {
		if _, err := GetDecision(r, b); err != nil {
			return d, fmt.Errorf("blocked by %s: %w", b, err)
		}
	}
	h := sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%d", d.Initiative, d.Question, time.Now().UnixNano())))
	d.ID = fmt.Sprintf("%s-%x", d.Initiative, h[:2])
	d.State, d.Created = Open, time.Now().UTC()
	return d, SaveDecision(r, d)
}

// SaveDecision writes a decision.
func SaveDecision(r *contract.Repo, d Decision) error {
	return writeTOML(filepath.Join(decisionDir(r, d.Initiative), d.ID+".toml"), d)
}

// AnswerPath is where a resolved decision's answer lives. The answer is the
// decision's detail; the map only ever gists it.
func AnswerPath(r *contract.Repo, d Decision) string {
	return filepath.Join(decisionDir(r, d.Initiative), d.ID+".answer.md")
}

// Decisions lists an initiative's decisions, oldest first.
func Decisions(r *contract.Repo, initiative string) ([]Decision, error) {
	matches, _ := filepath.Glob(filepath.Join(decisionDir(r, initiative), "*.toml"))
	var out []Decision
	for _, m := range matches {
		var d Decision
		if _, err := toml.DecodeFile(m, &d); err != nil {
			return nil, fmt.Errorf("%s: %w", m, err)
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created.Before(out[j].Created) })
	return out, nil
}

// GetDecision finds a decision across every initiative.
func GetDecision(r *contract.Repo, id string) (Decision, error) {
	ins, err := List(r)
	if err != nil {
		return Decision{}, err
	}
	for _, in := range ins {
		ds, _ := Decisions(r, in.ID)
		for _, d := range ds {
			if d.ID == id {
				return d, nil
			}
		}
	}
	return Decision{}, fmt.Errorf("no decision %q", id)
}

// Blocked reports whether a decision waits on one still open.
func Blocked(d Decision, byID map[string]Decision) bool {
	for _, b := range d.BlockedBy {
		if dep, ok := byID[b]; !ok || dep.State == Open {
			return true
		}
	}
	return false
}

// ByID indexes decisions.
func ByID(ds []Decision) map[string]Decision {
	m := make(map[string]Decision, len(ds))
	for _, d := range ds {
		m[d.ID] = d
	}
	return m
}

// Frontier is the edge of the known: open, unblocked decisions. Claims are
// filtered by the caller, which owns the shared ledger.
func Frontier(ds []Decision) []Decision {
	idx := ByID(ds)
	var out []Decision
	for _, d := range ds {
		if d.State == Open && !Blocked(d, idx) {
			out = append(out, d)
		}
	}
	return out
}

func writeTOML(path string, v any) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(v)
}
