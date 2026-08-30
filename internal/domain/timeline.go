package domain

import "time"

// EntryState is the life of a recorded commit.
//
// Rewritten and Phantom exist because git is the truth and the agent is not.
// Agents amend constantly, and a squash-merge erases every session commit from
// the default branch — an entry that is deleted rather than superseded makes the
// timeline empty itself exactly when it is most wanted (D-33).
type EntryState string

const (
	EntryLive        EntryState = "live"        // in git, reachable
	EntryRewritten   EntryState = "rewritten"   // amended/rebased/squashed away
	EntryPhantom     EntryState = "phantom"     // the agent claimed it; git has not got it
	EntryUncommitted EntryState = "uncommitted" // working tree at handoff
	EntryBaseline    EntryState = "baseline"    // where the session started
	EntryGate        EntryState = "gate"        // an approval or denial, not a commit
)

// ChangeStatus mirrors git's name-status letters.
type ChangeStatus string

const (
	Added    ChangeStatus = "A"
	Modified ChangeStatus = "M"
	Deleted  ChangeStatus = "D"
	Renamed  ChangeStatus = "R"
)

// FileChange is one path touched by one commit. No diff content, ever — the
// requirement was commit number and file list, and diffs already have `gh pr diff`.
type FileChange struct {
	Status ChangeStatus `json:"status"`
	Path   string       `json:"path"`
	From   string       `json:"from,omitempty"` // rename source
}

// Entry is one row of the change timeline.
type Entry struct {
	Seq     int          `json:"seq"`
	SHA     string       `json:"sha,omitempty"`
	Short   string       `json:"short,omitempty"`
	Subject string       `json:"subject"`
	Author  string       `json:"author,omitempty"`
	At      time.Time    `json:"at"`
	Branch  string       `json:"branch,omitempty"`
	State   EntryState   `json:"state"`
	Files   []FileChange `json:"files,omitempty"`

	// SupersededBy carries the successor when a commit was amended or rebased.
	// The entry is kept, not deleted.
	SupersededBy string `json:"superseded_by,omitempty"`

	// Attribution is best-effort and never authoritative. Git decides what
	// happened; this only records who was nearby when it did (D-31).
	Item    string `json:"item,omitempty"`
	Task    string `json:"task,omitempty"`
	Session string `json:"session,omitempty"`
}

// Timeline is the change record for one item.
type Timeline struct {
	Item     string    `json:"item"`
	Baseline string    `json:"baseline"`
	Branch   string    `json:"branch"`
	Repo     string    `json:"repo"`
	OpenedAt time.Time `json:"opened_at"`
	Entries  []Entry   `json:"entries"`
}

// Counts summarises the timeline for a status line.
func (t Timeline) Counts() (live, rewritten, phantom, uncommitted int) {
	for _, e := range t.Entries {
		switch e.State {
		case EntryLive:
			live++
		case EntryRewritten:
			rewritten++
		case EntryPhantom:
			phantom++
		case EntryUncommitted:
			uncommitted++
		}
	}
	return
}

// TouchedFiles is the set of paths across live and uncommitted entries.
// Rewritten entries are excluded: their changes survive in the successor, and
// counting both would double-count every amend.
func (t Timeline) TouchedFiles() []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range t.Entries {
		if e.State != EntryLive && e.State != EntryUncommitted {
			continue
		}
		for _, f := range e.Files {
			if !seen[f.Path] {
				seen[f.Path] = true
				out = append(out, f.Path)
			}
		}
	}
	return out
}
