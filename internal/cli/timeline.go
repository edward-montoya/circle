package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/domain"
	"github.com/edwardmontoya/circle/internal/timeline"
)

func init() {
	register(Command{"timeline open", "Record the baseline. SessionStart hook target", runTimelineOpen})
	register(Command{"timeline sync", "Reconcile against git; resolve rewrites", runTimelineSync})
	register(Command{"timeline show", "Commits and changed files. No diffs", runTimelineShow})
	register(Command{"timeline drift", "Files touched outside the approved radius", runTimelineDrift})
}

func itemFlag(f interface {
	String(string, string, string) *string
}) *string {
	return f.String("item", "default", "work item id")
}

func runTimelineOpen(e Env, args []string) int {
	f := fs("timeline open", e)
	item := itemFlag(f)
	if err := parse(f, args); err != nil {
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	t, err := timeline.Open(repo, *item)
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitExternal
	}
	fmt.Fprintf(e.Stdout, "baseline %s on %s\n", t.Baseline[:7], t.Branch)
	return ExitOK
}

func runTimelineSync(e Env, args []string) int {
	f := fs("timeline sync", e)
	item := itemFlag(f)
	noResolve := f.Bool("no-resolve-rewrites", false, "skip patch-id/reflog resolution")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	t, err := timeline.Sync(repo, *item, !*noResolve)
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitExternal
	}
	live, rew, phantom, unc := t.Counts()
	fmt.Fprintf(e.Stdout, "%d live · %d rewritten · %d phantom · %d uncommitted\n",
		live, rew, phantom, unc)
	return ExitOK
}

func runTimelineShow(e Env, args []string) int {
	f := fs("timeline show", e)
	item := itemFlag(f)
	format := f.String("format", "human", "human|json|md")
	noFiles := f.Bool("no-files", false, "commits only")
	state := f.String("state", "", "filter: live|rewritten|phantom|uncommitted")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	if f.NArg() > 0 {
		*item = f.Arg(0)
	}
	t, err := timeline.Sync(repo, *item, true)
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitExternal
	}

	entries := t.Entries
	if *state != "" {
		var keep []domain.Entry
		for _, en := range entries {
			if string(en.State) == *state {
				keep = append(keep, en)
			}
		}
		entries = keep
	}

	switch *format {
	case "json":
		b, _ := json.MarshalIndent(t, "", "  ")
		fmt.Fprintln(e.Stdout, string(b))
	case "md":
		renderTimelineMD(e, t, entries, *noFiles)
	default:
		renderTimelineHuman(e, t, entries, *noFiles)
	}
	return ExitOK
}

func renderTimelineHuman(e Env, t domain.Timeline, entries []domain.Entry, noFiles bool) {
	p := NewPalette(e.Stdout)
	fmt.Fprintf(e.Stdout, "\n%sTIMELINE%s  %s\n", p.B, p.N, t.Item)
	if t.Baseline != "" {
		fmt.Fprintf(e.Stdout, "%sbaseline %s · %s · opened %s%s\n\n",
			p.D, shortSHA(t.Baseline), t.Branch, t.OpenedAt.Format("2006-01-02 15:04"), p.N)
	}
	for _, en := range entries {
		mark, colour := "●", p.G
		switch en.State {
		case domain.EntryRewritten:
			mark, colour = "◌", p.D
		case domain.EntryPhantom:
			mark, colour = "✕", p.R
		case domain.EntryUncommitted:
			mark, colour = "◍", p.Y
		case domain.EntryBaseline, domain.EntryGate:
			mark, colour = "●", p.D
		}
		sha := en.Short
		if sha == "" {
			sha = "working tree"
		}
		fmt.Fprintf(e.Stdout, "  %s%s%s  %s%-12s%s %s\n",
			colour, mark, p.N, p.B, sha, p.N, en.Subject)
		if en.SupersededBy != "" {
			fmt.Fprintf(e.Stdout, "       %s└ superseded by %s%s\n", p.D, shortSHA(en.SupersededBy), p.N)
		}
		if !noFiles {
			for _, fc := range en.Files {
				c := p.Y
				switch fc.Status {
				case domain.Added:
					c = p.G
				case domain.Deleted:
					c = p.R
				}
				line := fmt.Sprintf("       %s%s%s  %s", c, fc.Status, p.N, fc.Path)
				if fc.From != "" {
					line += fmt.Sprintf(" %s← %s%s", p.D, fc.From, p.N)
				}
				fmt.Fprintln(e.Stdout, line)
			}
		}
	}
	live, rew, ph, unc := t.Counts()
	fmt.Fprintf(e.Stdout, "\n  %d live · %d rewritten · %d phantom · %d uncommitted · %d files touched\n\n",
		live, rew, ph, unc, len(t.TouchedFiles()))
	if ph > 0 {
		fmt.Fprintf(e.Stdout, "  %s✕ %d commit(s) the agent reported that git does not have%s\n\n", p.R, ph, p.N)
	}
}

// renderTimelineMD is what lands in brief.md and therefore in the PR body.
func renderTimelineMD(e Env, t domain.Timeline, entries []domain.Entry, noFiles bool) {
	fmt.Fprintf(e.Stdout, "### Change timeline — %s\n\n", t.Item)
	fmt.Fprintln(e.Stdout, "| Time | Commit | Subject | Files | State |")
	fmt.Fprintln(e.Stdout, "|---|---|---|---|---|")
	for _, en := range entries {
		var fs []string
		if !noFiles {
			for _, fc := range en.Files {
				fs = append(fs, fmt.Sprintf("`%s` %s", fc.Status, fc.Path))
			}
		}
		sha := en.Short
		if sha == "" {
			sha = "—"
		}
		subject := en.Subject
		if en.SupersededBy != "" {
			subject = fmt.Sprintf("~~%s~~ → `%s`", subject, shortSHA(en.SupersededBy))
		}
		fmt.Fprintf(e.Stdout, "| %s | `%s` | %s | %s | %s |\n",
			en.At.Format("15:04"), sha, subject, strings.Join(fs, "<br>"), en.State)
	}
	live, rew, ph, unc := t.Counts()
	fmt.Fprintf(e.Stdout, "\n%d live · %d rewritten · %d phantom · %d uncommitted\n", live, rew, ph, unc)
}

// runTimelineDrift compares what happened against what was approved.
func runTimelineDrift(e Env, args []string) int {
	f := fs("timeline drift", e)
	item := itemFlag(f)
	failOn := f.Bool("fail-on-drift", false, "exit 7 when anything drifted")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	if f.NArg() > 0 {
		*item = f.Arg(0)
	}
	t, err := timeline.Sync(repo, *item, true)
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitExternal
	}
	radius := approvedRadius(repo)
	if len(radius) == 0 {
		fmt.Fprintln(e.Stdout, "no approved blast radius recorded — nothing to compare against")
		return ExitOK
	}
	p := NewPalette(e.Stdout)
	var drifted []string
	for _, path := range t.TouchedFiles() {
		if !insideRadius(path, radius) {
			drifted = append(drifted, path)
		}
	}
	if len(drifted) == 0 {
		fmt.Fprintf(e.Stdout, "%s✓ every touched file is inside the approved radius%s\n", p.G, p.N)
		return ExitOK
	}
	fmt.Fprintf(e.Stdout, "%s⚠ %d file(s) outside the approved blast radius%s\n\n", p.Y, len(drifted), p.N)
	for _, d := range drifted {
		fmt.Fprintf(e.Stdout, "  %sM%s %s\n", p.Y, p.N, d)
	}
	fmt.Fprintf(e.Stdout, "\n%sapproved: %s%s\n", p.D, strings.Join(radius, " "), p.N)
	fmt.Fprintf(e.Stdout, "%sRecord an addendum, or regenerate the brief.%s\n", p.D, p.N)
	if *failOn {
		return ExitDrift
	}
	return ExitOK
}

func approvedRadius(r *contract.Repo) []string {
	b, err := readFileTrim(r.StateDir("approved-radius"))
	if err != nil {
		return nil
	}
	return strings.Fields(b)
}

func insideRadius(path string, radius []string) bool {
	for _, pat := range radius {
		if ok, _ := filepath.Match(pat, path); ok {
			return true
		}
		if strings.HasSuffix(pat, "/**") &&
			strings.HasPrefix(path, strings.TrimSuffix(pat, "/**")+"/") {
			return true
		}
	}
	return false
}

func shortSHA(s string) string {
	if len(s) > 7 {
		return s[:7]
	}
	return s
}

// lastCommitTime backs the task-closure rule: a gate run must be newer than the
// work it claims to verify, or a stale green would close anything.
func lastCommitTime(r *contract.Repo, taskID string) time.Time {
	t, err := timeline.Load(r, "default")
	if err != nil {
		return time.Time{}
	}
	return timeline.LastCommitAt(t)
}
