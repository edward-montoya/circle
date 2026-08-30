package timeline

import (
	"encoding/json"
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
