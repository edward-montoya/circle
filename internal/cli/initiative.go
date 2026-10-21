package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/edwardmontoya/circle/internal/claims"
	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/domain"
	"github.com/edwardmontoya/circle/internal/events"
	"github.com/edwardmontoya/circle/internal/initiative"
	"github.com/edwardmontoya/circle/internal/item"
	"github.com/edwardmontoya/circle/internal/session"
)

func init() {
	register(Command{"initiative new", "Chart an effort too big for one session", runInitiativeNew})
	register(Command{"initiative list", "Every initiative and how far its map has come", runInitiativeList})
	register(Command{"initiative show", "The map: destination, decisions so far, frontier, fog, items", runInitiativeShow})
	register(Command{"decision new", "Add a question to an initiative's map", runDecisionNew})
	register(Command{"decision link", "Wire blocking edges once the decisions exist", runDecisionLink})
	register(Command{"decision frontier", "Open, unblocked, unclaimed decisions", runDecisionFrontier})
	register(Command{"decision claim", "Take a decision. Visible to every worktree", runDecisionClaim})
	register(Command{"decision show", "One decision, with its answer", runDecisionShow})
	register(Command{"decision resolve", "Record the answer and close the decision", runDecisionResolve})
	register(Command{"decision scope-out", "Rule a decision beyond the destination", runDecisionScopeOut})
}

func runInitiativeNew(e Env, args []string) int {
	f := fs("initiative new", e)
	title := f.String("title", "", "one line")
	dest := f.String("destination", "", "REQUIRED: what reaching the end of this map looks like")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	if f.NArg() == 0 {
		fmt.Fprintln(e.Stderr, `usage: circle initiative new <id> --destination "..." [--title "..."]`)
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	in := initiative.Initiative{ID: f.Arg(0), Title: *title, Destination: *dest, Author: gitEmail(repo.Root)}
	if in.Title == "" {
		in.Title = in.ID
	}
	if err := initiative.New(repo, in); err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		if errors.Is(err, initiative.ErrExists) {
			return ExitPrecond
		}
		return ExitError
	}
	fmt.Fprintf(e.Stdout, "charted initiative %s\n  destination  %s\n", in.ID, in.Destination)
	fmt.Fprintf(e.Stdout, "  write Notes and Not yet specified in .circle/initiatives/%s/map.md\n", in.ID)
	fmt.Fprintf(e.Stdout, "  then: circle decision new --initiative %s --type grill --question \"...\"\n", in.ID)
	return ExitOK
}

func runInitiativeList(e Env, args []string) int {
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	ins, err := initiative.List(repo)
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	if len(ins) == 0 {
		fmt.Fprintln(e.Stdout, "no initiatives — `circle initiative new <id> --destination \"...\"`")
		return ExitOK
	}
	p := NewPalette(e.Stdout)
	for _, in := range ins {
		ds, _ := initiative.Decisions(repo, in.ID)
		open, resolved := 0, 0
		for _, d := range ds {
			switch d.State {
			case initiative.Open:
				open++
			case initiative.Resolved:
				resolved++
			}
		}
		fmt.Fprintf(e.Stdout, "%s%-20s%s %s  %s(%d resolved, %d open)%s\n", p.B, in.ID, p.N, in.Title, p.D, resolved, open, p.N)
	}
	return ExitOK
}

// currentInitiative resolves the initiative a command is about: the flag, the
// initiative of the decision this session is bound to, or the only one there is.
func currentInitiative(repo *contract.Repo, flag string) (string, error) {
	if flag != "" {
		return flag, nil
	}
	if b, ok := session.Lookup(repo.Root, session.ID("")); ok && b.Decision != "" {
		if d, err := initiative.GetDecision(repo, b.Decision); err == nil {
			return d.Initiative, nil
		}
	}
	ins, _ := initiative.List(repo)
	if len(ins) == 1 {
		return ins[0].ID, nil
	}
	return "", errors.New("name the initiative: --initiative <id> (`circle initiative list`)")
}

func runInitiativeShow(e Env, args []string) int {
	f := fs("initiative show", e)
	flag := f.String("initiative", "", "initiative id")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	if f.NArg() > 0 {
		*flag = f.Arg(0)
	}
	id, err := currentInitiative(repo, *flag)
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitPrecond
	}
	in, err := initiative.Load(repo, id)
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitPrecond
	}
	ds, _ := initiative.Decisions(repo, id)
	held := claims.All(repo.Root)

	// Refer by name: a wall of ids is illegible, so every decision prints as its
	// title, with the id riding alongside.
	fmt.Fprintf(e.Stdout, "# %s\n\n## Destination\n\n%s\n\n", in.Title, in.Destination)
	fmt.Fprint(e.Stdout, "## Decisions so far\n\n")
	n := 0
	for _, d := range ds {
		if d.State == initiative.Resolved {
			fmt.Fprintf(e.Stdout, "- %s (`%s`): %s\n", d.Title, d.ID, d.Gist)
			n++
		}
	}
	if n == 0 {
		fmt.Fprintln(e.Stdout, "_none yet_")
	}
	fmt.Fprint(e.Stdout, "\n## Frontier\n\n")
	front := initiative.Frontier(ds)
	for _, d := range front {
		claim := ""
		if c, ok := held[d.ID]; ok && c.Held() {
			claim = " — claimed by " + c.Holder
		}
		fmt.Fprintf(e.Stdout, "- [%s] %s (`%s`)%s\n", d.Type, d.Title, d.ID, claim)
	}
	if len(front) == 0 {
		fmt.Fprintln(e.Stdout, "_empty_")
	}
	blocked := 0
	for _, d := range ds {
		if d.State == initiative.Open && initiative.Blocked(d, initiative.ByID(ds)) {
			blocked++
		}
	}
	if blocked > 0 {
		fmt.Fprintf(e.Stdout, "\n%d more decision(s) blocked behind the frontier.\n", blocked)
	}

	// The human-written sections: notes, fog, and what was ruled out.
	if b, err := os.ReadFile(filepath.Join(initiative.Dir(repo, id), "map.md")); err == nil {
		fmt.Fprintf(e.Stdout, "\n%s\n", strings.TrimRight(string(b), "\n"))
	}

	fmt.Fprint(e.Stdout, "\n## Items\n\n")
	items, _ := item.List(repo)
	n = 0
	for _, it := range items {
		if it.Initiative == id {
			fmt.Fprintf(e.Stdout, "- %s (`%s`) — %s\n", it.Title, it.ID, item.Derive(repo, it.ID).State)
			n++
		}
	}
	if n == 0 {
		fmt.Fprintln(e.Stdout, "_none yet — the map produces items once the way is clear_")
	}
	if len(front) == 0 && blocked == 0 && len(ds) > 0 {
		fmt.Fprintln(e.Stdout, "\nThe frontier is empty. If Not yet specified is empty too, the way is clear:")
		fmt.Fprintf(e.Stdout, "create the items — circle item new <id> --initiative %s --title \"...\"\n", id)
	}
	return ExitOK
}

func runDecisionNew(e Env, args []string) int {
	f := fs("decision new", e)
	flag := f.String("initiative", "", "the initiative this decision belongs to")
	typ := f.String("type", "grill", "research, prototype, grill or task")
	question := f.String("question", "", "REQUIRED: the question this decision resolves")
	title := f.String("title", "", "a short name; defaults to the question")
	blocked := f.String("blocked-by", "", "comma-separated decision ids")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	id, err := currentInitiative(repo, *flag)
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitPrecond
	}
	d := initiative.Decision{Initiative: id, Title: *title, Question: *question,
		Type: initiative.Type(*typ), BlockedBy: splitList(*blocked)}
	d, err = initiative.NewDecision(repo, d)
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	ev := domain.NewEvent(domain.EvDecision)
	ev.Item, ev.Task, ev.Reason = id, d.ID, "new"
	_ = events.Append(repo, ev)
	fmt.Fprintf(e.Stdout, "created %s  [%s] %s\n", d.ID, d.Type, d.Title)
	return ExitOK
}

func runDecisionLink(e Env, args []string) int {
	f := fs("decision link", e)
	blocked := f.String("blocked-by", "", "comma-separated decision ids")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	if f.NArg() == 0 || *blocked == "" {
		fmt.Fprintln(e.Stderr, "usage: circle decision link <id> --blocked-by <id,id>")
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	d, err := initiative.GetDecision(repo, f.Arg(0))
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	if err := initiative.CheckBlockers(repo, d.Initiative, d.ID, splitList(*blocked)); err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	for _, b := range splitList(*blocked) {
		if !contains(d.BlockedBy, b) {
			d.BlockedBy = append(d.BlockedBy, b)
		}
	}
	if err := initiative.SaveDecision(repo, d); err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	fmt.Fprintf(e.Stdout, "%s is blocked by %s\n", d.ID, strings.Join(d.BlockedBy, ", "))
	return ExitOK
}

func runDecisionFrontier(e Env, args []string) int {
	f := fs("decision frontier", e)
	flag := f.String("initiative", "", "only this initiative")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	ins, _ := initiative.List(repo)
	held := claims.All(repo.Root)
	p := NewPalette(e.Stdout)
	shown := 0
	for _, in := range ins {
		if *flag != "" && in.ID != *flag {
			continue
		}
		ds, _ := initiative.Decisions(repo, in.ID)
		for _, d := range initiative.Frontier(ds) {
			if _, ok := held[d.ID]; ok {
				continue
			}
			fmt.Fprintf(e.Stdout, "%s%-18s%s [%s] %s  %s(%s)%s\n", p.B, d.ID, p.N, d.Type, d.Title, p.D, in.ID, p.N)
			shown++
		}
	}
	if shown == 0 {
		fmt.Fprintln(e.Stdout, "no open decisions on any frontier")
	}
	return ExitOK
}

func runDecisionClaim(e Env, args []string) int {
	f := fs("decision claim", e)
	steal := f.Bool("steal", false, "take a decision someone else holds")
	bind := f.Bool("bind", true, "bind this session to the decision")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	if f.NArg() == 0 {
		fmt.Fprintln(e.Stderr, "usage: circle decision claim <id>")
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	d, err := initiative.GetDecision(repo, f.Arg(0))
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	if d.State != initiative.Open {
		fmt.Fprintf(e.Stderr, "circle: %s is %s\n", d.ID, d.State)
		return ExitGate
	}
	ds, _ := initiative.Decisions(repo, d.Initiative)
	if initiative.Blocked(d, initiative.ByID(ds)) {
		fmt.Fprintf(e.Stderr, "circle: %s waits on %s\n", d.ID, strings.Join(d.BlockedBy, ", "))
		return ExitGate
	}
	c := holder(repo)
	c.Task, c.Item = d.ID, d.Initiative
	if err := claims.Take(repo.Root, c, *steal); err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitGate
	}
	if *bind && c.Session != "" {
		_ = session.Bind(repo.Root, session.Binding{Session: c.Session, Decision: d.ID, Worktree: c.Worktree})
	}
	fmt.Fprintf(e.Stdout, "claimed %s [%s] %s\n  question  %s\n", d.ID, d.Type, d.Title, d.Question)
	return ExitOK
}

func runDecisionShow(e Env, args []string) int {
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	if len(args) == 0 {
		fmt.Fprintln(e.Stderr, "usage: circle decision show <id>")
		return ExitError
	}
	d, err := initiative.GetDecision(repo, args[0])
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	fmt.Fprintf(e.Stdout, "%s  [%s] %s  (%s)\n\nQuestion\n  %s\n", d.ID, d.Type, d.Title, d.State, d.Question)
	if len(d.BlockedBy) > 0 {
		fmt.Fprintf(e.Stdout, "\nBlocked by\n  %s\n", strings.Join(d.BlockedBy, ", "))
	}
	if d.Why != "" {
		fmt.Fprintf(e.Stdout, "\nOut of scope\n  %s\n", d.Why)
	}
	if b, err := os.ReadFile(initiative.AnswerPath(repo, d)); err == nil {
		fmt.Fprintf(e.Stdout, "\nAnswer — %s\n\n%s\n", d.Gist, strings.TrimRight(string(b), "\n"))
	}
	return ExitOK
}

func runDecisionResolve(e Env, args []string) int {
	f := fs("decision resolve", e)
	answer := f.String("answer", "", "REQUIRED: a markdown file holding the answer and its reasoning")
	gist := f.String("gist", "", "REQUIRED: the answer in one line, for the map")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	if f.NArg() == 0 || *answer == "" || strings.TrimSpace(*gist) == "" {
		fmt.Fprintln(e.Stderr, `usage: circle decision resolve <id> --answer <file.md> --gist "..."`)
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	d, err := initiative.GetDecision(repo, f.Arg(0))
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	if d.State != initiative.Open {
		fmt.Fprintf(e.Stderr, "circle: %s is already %s\n", d.ID, d.State)
		return ExitGate
	}
	body, err := os.ReadFile(*answer)
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	if err := os.WriteFile(initiative.AnswerPath(repo, d), body, 0o644); err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	d.State, d.Gist, d.Closed = initiative.Resolved, strings.TrimSpace(*gist), time.Now().UTC()
	if err := initiative.SaveDecision(repo, d); err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	_ = claims.Release(repo.Root, d.ID)
	ev := domain.NewEvent(domain.EvDecision)
	ev.Item, ev.Task, ev.Reason = d.Initiative, d.ID, "resolved"
	_ = events.Append(repo, ev)
	fmt.Fprintf(e.Stdout, "resolved %s: %s\n", d.Title, d.Gist)
	fmt.Fprintln(e.Stdout, "Now push back the fog: graduate what this answer made specifiable into new decisions.")
	return ExitOK
}

func runDecisionScopeOut(e Env, args []string) int {
	f := fs("decision scope-out", e)
	why := f.String("why", "", "REQUIRED: why this lies beyond the destination")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	if f.NArg() == 0 || strings.TrimSpace(*why) == "" {
		fmt.Fprintln(e.Stderr, `usage: circle decision scope-out <id> --why "..."`)
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	d, err := initiative.GetDecision(repo, f.Arg(0))
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	d.State, d.Why, d.Closed = initiative.OutOfScope, strings.TrimSpace(*why), time.Now().UTC()
	if err := initiative.SaveDecision(repo, d); err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	_ = claims.Release(repo.Root, d.ID)
	// One line in the map's Out of scope section, so the boundary is visible
	// where people read the map, not only in the decision file.
	mp := filepath.Join(initiative.Dir(repo, d.Initiative), "map.md")
	if b, err := os.ReadFile(mp); err == nil {
		line := fmt.Sprintf("- %s (`%s`) — %s", d.Title, d.ID, d.Why)
		_ = os.WriteFile(mp, []byte(strings.TrimRight(string(b), "\n")+"\n"+line+"\n"), 0o644)
	}
	fmt.Fprintf(e.Stdout, "ruled out of scope: %s\n", d.Title)
	return ExitOK
}

func splitList(s string) []string {
	var out []string
	for _, x := range strings.Split(s, ",") {
		if x = strings.TrimSpace(x); x != "" {
			out = append(out, x)
		}
	}
	return out
}

// initiativeLoad is used where only existence matters.
func initiativeLoad(repo *contract.Repo, id string) (initiative.Initiative, error) {
	return initiative.Load(repo, id)
}
