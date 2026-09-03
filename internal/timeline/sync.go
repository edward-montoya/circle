package timeline

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/domain"
)

// path is durable, not runtime: the commit set is regenerable from git, but the
// attribution — which task, which session, which decision — is not (D-32).
func path(r *contract.Repo, item string) string {
	return r.StateDir("timelines", item+".json")
}

// ErrNoTimeline means no timeline has been opened for this item, so there is no
// baseline to date anything against.
var ErrNoTimeline = errors.New("no timeline recorded")

func Load(r *contract.Repo, item string) (domain.Timeline, error) {
	var t domain.Timeline
	b, err := os.ReadFile(path(r, item))
	if err != nil {
		if os.IsNotExist(err) {
			return domain.Timeline{Item: item}, nil
		}
		return t, err
	}
	return t, json.Unmarshal(b, &t)
}

func Save(r *contract.Repo, t domain.Timeline) error {
	p := path(r, t.Item)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, append(b, '\n'), 0o644)
}

// Open records the baseline for an item.
//
// Per (repo, worktree, branch): under the isolation model each item lives on its
// own branch in its own worktree, so HEAD in the main checkout never moves.
func Open(r *contract.Repo, item string) (domain.Timeline, error) {
	sha, err := HeadSHA(r.Root)
	if err != nil {
		return domain.Timeline{}, err
	}
	br, _ := Branch(r.Root)
	t := domain.Timeline{
		Item: item, Baseline: sha, Branch: br, Repo: r.Root,
		OpenedAt: time.Now().UTC(),
		Entries: []domain.Entry{{
			Seq: 0, SHA: sha, Short: short(sha), Subject: "baseline · session opened",
			At: time.Now().UTC(), State: domain.EntryBaseline,
		}},
	}
	return t, Save(r, t)
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// Sync reconciles the recorded timeline against git.
//
// Git is the truth set; anything recorded but absent is surfaced as a phantom
// rather than quietly dropped, and anything present but unattributed is still
// listed. The agent's account is never allowed to overrule the repository.
func Sync(r *contract.Repo, item string, resolveRewrites bool) (domain.Timeline, error) {
	t, err := Load(r, item)
	if err != nil {
		return t, err
	}
	if t.Baseline == "" {
		return Open(r, item)
	}

	live, err := Commits(r.Root, t.Baseline)
	if err != nil {
		return t, err
	}
	for i := range live {
		live[i].Files, _ = FilesOf(r.Root, live[i].SHA)
	}
	liveBySHA := map[string]bool{}
	for _, e := range live {
		liveBySHA[e.SHA] = true
	}

	// Carry forward attribution and non-commit entries from the old record.
	prevBySHA := map[string]domain.Entry{}
	var carried []domain.Entry
	for _, e := range t.Entries {
		switch e.State {
		case domain.EntryBaseline, domain.EntryGate:
			carried = append(carried, e)
			continue
		case domain.EntryUncommitted:
			continue // always recomputed
		}
		prevBySHA[e.SHA] = e
		if !liveBySHA[e.SHA] {
			// Recorded once, gone from git now. Superseded, never deleted.
			e.State = domain.EntryRewritten
			if e.SupersededBy == "" && resolveRewrites && Exists(r.Root, e.SHA) {
				e.SupersededBy = ResolveRewrite(r.Root, e, live)
			}
			if !Exists(r.Root, e.SHA) && e.SupersededBy == "" {
				// Never existed at all: the agent claimed a commit git has not got.
				e.State = domain.EntryPhantom
			}
			carried = append(carried, e)
		}
	}

	for i, e := range live {
		if prev, ok := prevBySHA[e.SHA]; ok {
			live[i].Item, live[i].Task, live[i].Session = prev.Item, prev.Task, prev.Session
		}
	}

	entries := append(carried, live...)

	if files, err := Uncommitted(r.Root); err == nil && len(files) > 0 {
		entries = append(entries, domain.Entry{
			Subject: "uncommitted at handoff", At: time.Now().UTC(),
			State: domain.EntryUncommitted, Files: files,
		})
	}

	for i := range entries {
		entries[i].Seq = i
	}
	t.Entries = entries
	return t, Save(r, t)
}

// RecordPhantom notes a commit the agent claimed. Sync decides whether it is real.
func RecordPhantom(r *contract.Repo, item, sha, subject string) error {
	t, err := Load(r, item)
	if err != nil {
		return err
	}
	t.Entries = append(t.Entries, domain.Entry{
		SHA: sha, Short: short(sha), Subject: subject,
		At: time.Now().UTC(), State: domain.EntryPhantom,
	})
	return Save(r, t)
}

// LastCommitAt is the timestamp of the newest live commit, used to decide
// whether a gate run is newer than the work it claims to verify.
func LastCommitAt(t domain.Timeline) time.Time {
	var newest time.Time
	for _, e := range t.Entries {
		if e.State == domain.EntryLive && e.At.After(newest) {
			newest = e.At
		}
	}
	return newest
}

// TaskCommitAt is the timestamp of the newest live commit attributed to taskID,
// or the zero time when none is.
//
// Attribution is best-effort and never authoritative (D-31), which is why this
// reports "nothing attributed" rather than an error: the caller falls back to a
// wider floor instead of refusing work git cannot tie to a task.
func TaskCommitAt(t domain.Timeline, taskID string) time.Time {
	var newest time.Time
	if taskID == "" {
		return newest
	}
	for _, e := range t.Entries {
		if e.State == domain.EntryLive && e.Task == taskID && e.At.After(newest) {
			newest = e.At
		}
	}
	return newest
}

// Floor is the instant a passing gate run must beat for taskID to close.
//
// Narrowest first: commits attributed to this task, then any live commit on the
// item, then the baseline the timeline opened at. Each fallback is no earlier
// than the one before it, so widening never lets a task close against a floor
// that predates its own work.
//
// A missing timeline is an error, not a zero time. Returning the zero time here
// is satisfied by every gate run ever recorded, which silently downgrades "a
// gate passed since your last commit" to "a gate passed at some point" — the
// guarantee this function exists to make, quietly deleted. Refusing to guess is
// the safe failure.
func Floor(r *contract.Repo, item, taskID string) (time.Time, error) {
	b, err := os.ReadFile(path(r, item))
	if err != nil {
		if os.IsNotExist(err) {
			return time.Time{}, ErrNoTimeline
		}
		return time.Time{}, err
	}
	var t domain.Timeline
	if err := json.Unmarshal(b, &t); err != nil {
		return time.Time{}, err
	}
	if at := TaskCommitAt(t, taskID); !at.IsZero() {
		return at, nil
	}
	if at := LastCommitAt(t); !at.IsZero() {
		return at, nil
	}
	// No commits yet. The session baseline is still a real floor: it proves the
	// gate ran after this stretch of work began, which is the most that can be
	// claimed when nothing has been committed.
	if t.OpenedAt.IsZero() {
		return time.Time{}, ErrNoTimeline
	}
	return t.OpenedAt, nil
}
