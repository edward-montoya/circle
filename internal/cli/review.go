package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/domain"
	"github.com/edwardmontoya/circle/internal/domaindoc"
	"github.com/edwardmontoya/circle/internal/events"
	"github.com/edwardmontoya/circle/internal/item"
	"github.com/edwardmontoya/circle/internal/task"
	"github.com/edwardmontoya/circle/internal/timeline"
)

func init() {
	register(Command{"review drift", "The computed review axis: radius drift, untraced commits, domain gaps", runReviewDrift})
	register(Command{"review record", "Store one judged axis (spec or standards) for the item", runReviewRecord})
	register(Command{"review show", "Every axis recorded so far", runReviewShow})
	register(Command{"review close", "Mark the item done. Refuses on any blocking finding or red gate", runReviewClose})
}

// trace is the review axis nobody judges: every number in it comes from git,
// the approved radius and the domain model.
type trace struct {
	Base       string              `json:"base"`
	Drifted    []string            `json:"drifted"`
	Untraced   []string            `json:"untraced"`
	DomainGaps []domaindoc.Finding `json:"domain_gaps"`
	Commits    int                 `json:"commits"`
	entries    []domain.Entry      `json:"-"`
	tasks      map[string]bool     `json:"-"`
}

// blocking counts what holds the item open. Untraced commits are reported but
// do not block: a commit message is a convention, not a contract.
func (t trace) blocking(domainBlocks bool) int {
	n := len(t.Drifted)
	if domainBlocks {
		n += len(t.DomainGaps)
	}
	return n
}

func computeTrace(repo *contract.Repo, id string) (trace, error) {
	tl, err := timeline.Peek(repo, id, true)
	if err != nil {
		return trace{}, err
	}
	tr := trace{Base: tl.Baseline, tasks: map[string]bool{}}
	// Every task id in the repository, by item: a commit that names another
	// item's task is that item's work, merged in — not this item's drift.
	others := map[string]bool{}
	all, _ := task.Load(repo)
	for _, t := range all {
		if task.InItem(t, id) {
			tr.tasks[t.ID] = true
		} else {
			others[t.ID] = true
		}
	}
	radius := approvedRadius(repo, id)
	drifted := map[string]bool{}
	for _, en := range tl.Entries {
		live := en.State == domain.EntryLive && en.SHA != ""
		if !live && en.State != domain.EntryUncommitted {
			continue
		}
		if live && !namesTask(en.Subject, tr.tasks) && namesTask(en.Subject, others) {
			continue // another item's commit, brought in by a merge
		}
		if live {
			tr.Commits++
			if !bookkeeping(en) && !namesTask(en.Subject, tr.tasks) && !strings.HasPrefix(en.Subject, "Merge ") {
				tr.Untraced = append(tr.Untraced, en.Short+" "+en.Subject)
			}
		}
		for _, f := range en.Files {
			path := f.Path
			if len(radius) > 0 && !domain.InRadius(path, radius) && !domaindoc.IsDomainDoc(path) &&
				!strings.HasPrefix(path, contract.Dir+"/") {
				drifted[path] = true
			}
		}
	}
	for p := range drifted {
		tr.Drifted = append(tr.Drifted, p)
	}
	if tl.Baseline != "" {
		tr.DomainGaps = domaindoc.CheckSince(repo.Root, domaindoc.Discover(repo.Root), tl.Baseline, repo.Contract.Domain.Ignore)
	}
	sort.Strings(tr.Drifted)
	return tr, nil
}

// bookkeeping is a commit that only records planning: Circle's own state — a
// shape, a spec, a plan, an approval — and the domain model written while
// shaping. It is the process, not the work, and there was no task to name.
func bookkeeping(en domain.Entry) bool {
	if len(en.Files) == 0 {
		return false
	}
	for _, f := range en.Files {
		if !strings.HasPrefix(f.Path, contract.Dir+"/") && !domaindoc.IsDomainDoc(f.Path) {
			return false
		}
	}
	return true
}

func namesTask(subject string, tasks map[string]bool) bool {
	for id := range tasks {
		if strings.Contains(subject, id) {
			return true
		}
	}
	return false
}

func runReviewDrift(e Env, args []string) int {
	f := fs("review drift", e)
	flag := f.String("item", "", "item id; defaults to this session's")
	asJSON := f.Bool("json", false, "machine-readable")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	id, _ := currentItem(repo, *flag)
	tr, err := computeTrace(repo, id)
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitExternal
	}
	if *asJSON {
		return writeJSON(e, tr)
	}
	renderTrace(e, id, tr, repo.Contract.Domain.Blocks())
	return ExitOK
}

func renderTrace(e Env, id string, tr trace, domainBlocks bool) {
	p := NewPalette(e.Stdout)
	fmt.Fprintf(e.Stdout, "%sTRACE%s  item %s · %d commit(s) since %s\n", p.B, p.N, id, tr.Commits, shortSHA(tr.Base))
	if len(tr.Drifted) == 0 {
		fmt.Fprintf(e.Stdout, "  %s✓%s every touched file is inside the approved radius\n", p.G, p.N)
	}
	for _, d := range tr.Drifted {
		fmt.Fprintf(e.Stdout, "  %s✗ [blocking]%s %s is outside the approved radius\n", p.R, p.N, d)
	}
	for _, u := range tr.Untraced {
		fmt.Fprintf(e.Stdout, "  %s⚠%s commit names no task: %s\n", p.Y, p.N, u)
	}
	sev := "⚠"
	if domainBlocks {
		sev = "✗ [blocking]"
	}
	for _, g := range tr.DomainGaps {
		fmt.Fprintf(e.Stdout, "  %s%s%s %s\n", p.Y, sev, p.N, g)
	}
	if len(tr.Untraced) == 0 && len(tr.DomainGaps) == 0 && len(tr.Drifted) == 0 {
		fmt.Fprintf(e.Stdout, "  %s✓%s every commit names a task; the domain model covers every new concept\n", p.G, p.N)
	}
}

// axisRecord is one judged axis as stored.
type axisRecord struct {
	Axis     string    `json:"axis"`
	Blocking int       `json:"blocking"`
	Findings int       `json:"findings"`
	At       time.Time `json:"at"`
	// Head is the commit the axis reviewed. A review of an older commit says
	// nothing about the code that landed after it.
	Head string `json:"head,omitempty"`
}

// stale reports whether an axis reviewed a commit other than HEAD.
func (a axisRecord) stale(head string) bool { return head != "" && a.Head != head }

type reviewState struct {
	Axes map[string]axisRecord `json:"axes"`
}

var (
	blockingRe = regexp.MustCompile(`(?i)\[blocking\]|🔴`)
	findingRe  = regexp.MustCompile(`(?i)^\s*[-*]\s*\[(blocking|major|minor|nit)\]`)
)

func reviewDir(repo *contract.Repo, id string) string {
	return filepath.Join(item.Dir(repo, id), "review")
}

func loadReview(repo *contract.Repo, id string) reviewState {
	var s reviewState
	if b, err := os.ReadFile(filepath.Join(reviewDir(repo, id), "review.json")); err == nil {
		_ = json.Unmarshal(b, &s)
	}
	if s.Axes == nil {
		s.Axes = map[string]axisRecord{}
	}
	return s
}

// runReviewRecord stores a judged axis. Findings are markdown list items tagged
// [blocking], [major], [minor] or [nit]; only [blocking] holds the item open.
func runReviewRecord(e Env, args []string) int {
	f := fs("review record", e)
	flag := f.String("item", "", "item id; defaults to this session's")
	axis := f.String("axis", "", "spec or standards")
	file := f.String("file", "", "the findings, as markdown")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	if (*axis != "spec" && *axis != "standards") || *file == "" {
		fmt.Fprintln(e.Stderr, "usage: circle review record --axis spec|standards --file <findings.md>")
		fmt.Fprintln(e.Stderr, "The trace axis is computed (`circle review drift`), never recorded by hand.")
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	id, _ := currentItem(repo, *flag)
	body, err := os.ReadFile(*file)
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	head, _ := timeline.HeadSHA(repo.Root)
	rec := axisRecord{Axis: *axis, At: time.Now().UTC(), Head: head}
	for _, line := range strings.Split(string(body), "\n") {
		if findingRe.MatchString(line) {
			rec.Findings++
		}
		if blockingRe.MatchString(line) {
			rec.Blocking++
		}
	}
	if err := os.MkdirAll(reviewDir(repo, id), 0o755); err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	if err := os.WriteFile(filepath.Join(reviewDir(repo, id), *axis+".md"), body, 0o644); err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	s := loadReview(repo, id)
	s.Axes[*axis] = rec
	b, _ := json.MarshalIndent(s, "", "  ")
	if err := os.WriteFile(filepath.Join(reviewDir(repo, id), "review.json"), append(b, '\n'), 0o644); err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	fmt.Fprintf(e.Stdout, "recorded %s axis for %s · %d finding(s), %d blocking\n", *axis, id, rec.Findings, rec.Blocking)
	return ExitOK
}

func runReviewShow(e Env, args []string) int {
	f := fs("review show", e)
	flag := f.String("item", "", "item id; defaults to this session's")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	id, _ := currentItem(repo, *flag)
	tr, err := computeTrace(repo, id)
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitExternal
	}
	renderTrace(e, id, tr, repo.Contract.Domain.Blocks())
	s := loadReview(repo, id)
	head, _ := timeline.HeadSHA(repo.Root)
	for _, axis := range []string{"spec", "standards"} {
		r, ok := s.Axes[axis]
		if !ok {
			fmt.Fprintf(e.Stdout, "\n%s — not recorded yet\n", strings.ToUpper(axis))
			continue
		}
		if r.stale(head) {
			fmt.Fprintf(e.Stdout, "\n%s — STALE: reviewed %s, HEAD is %s. Review again before trusting it.\n",
				strings.ToUpper(axis), shortSHA(r.Head), shortSHA(head))
		}
		body, _ := os.ReadFile(filepath.Join(reviewDir(repo, id), axis+".md"))
		fmt.Fprintf(e.Stdout, "\n%s — %d finding(s), %d blocking\n%s\n", strings.ToUpper(axis), r.Findings, r.Blocking,
			strings.TrimRight(string(body), "\n"))
	}
	return ExitOK
}

// runReviewClose is the last gate. An item is done when every task is closed,
// both judged axes are recorded with nothing blocking, the computed axis is
// clean, and every gate is green — not when someone says so.
func runReviewClose(e Env, args []string) int {
	f := fs("review close", e)
	flag := f.String("item", "", "item id; defaults to this session's")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	id, _ := currentItem(repo, *flag)
	var reasons []string

	st := item.Derive(repo, id)
	if st.State != item.Reviewing {
		reasons = append(reasons, fmt.Sprintf("the item is %s (%s), not reviewing", st.State, st.Reason))
	}
	s := loadReview(repo, id)
	head, _ := timeline.HeadSHA(repo.Root)
	for _, axis := range []string{"spec", "standards"} {
		r, ok := s.Axes[axis]
		switch {
		case ok && r.stale(head):
			reasons = append(reasons, fmt.Sprintf("the %s axis reviewed %s, not HEAD %s — review the code that landed since",
				axis, shortSHA(r.Head), shortSHA(head)))
		case !ok:
			reasons = append(reasons, fmt.Sprintf("the %s axis is not recorded — `circle review record --axis %s`", axis, axis))
		case r.Blocking > 0:
			reasons = append(reasons, fmt.Sprintf("the %s axis has %d blocking finding(s)", axis, r.Blocking))
		}
	}
	tr, err := computeTrace(repo, id)
	if err != nil {
		reasons = append(reasons, "the trace axis could not be computed: "+err.Error())
	} else if n := tr.blocking(repo.Contract.Domain.Blocks()); n > 0 {
		reasons = append(reasons, fmt.Sprintf("the trace axis has %d blocking finding(s) — `circle review drift`", n))
	}
	evs, _ := events.All(repo)
	if pass, total := events.GateSummary(evs, repo.Contract.Quality.Gates()); pass < total {
		reasons = append(reasons, fmt.Sprintf("%d of %d gates are not green — `circle quality run`", total-pass, total))
	}

	p := NewPalette(e.Stderr)
	if len(reasons) > 0 {
		fmt.Fprintf(e.Stderr, "%scircle: %s cannot close%s\n", p.R, id, p.N)
		for _, r := range reasons {
			fmt.Fprintf(e.Stderr, "  %s✗%s %s\n", p.R, p.N, r)
		}
		return ExitGate
	}

	done := map[string]any{
		"item": id, "at": time.Now().UTC(), "by": gitEmail(repo.Root),
		"base": tr.Base, "commits": tr.Commits, "untraced": len(tr.Untraced), "axes": s.Axes,
	}
	b, _ := json.MarshalIndent(done, "", "  ")
	if err := os.WriteFile(filepath.Join(item.Dir(repo, id), "done.json"), append(b, '\n'), 0o644); err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	ev := domain.NewEvent(domain.EvItemDone)
	ev.Item, ev.Actor = id, gitEmail(repo.Root)
	_ = events.Append(repo, ev)
	fmt.Fprintf(e.Stdout, "%s✓%s %s is done · next: /circle:handoff, then /circle:retro in this session\n",
		NewPalette(e.Stdout).G, NewPalette(e.Stdout).N, id)
	return ExitOK
}
