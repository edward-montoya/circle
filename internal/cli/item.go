package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/edwardmontoya/circle/internal/claims"
	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/domain"
	"github.com/edwardmontoya/circle/internal/item"
	"github.com/edwardmontoya/circle/internal/session"
	"github.com/edwardmontoya/circle/internal/task"
)

func init() {
	register(Command{"item new", "Create a work item: a feature, fix or refactor", runItemNew})
	register(Command{"item list", "Every item and its derived state", runItemList})
	register(Command{"item show", "One item: state, evidence, tasks", runItemShow})
	register(Command{"item require", "Exit 2 unless the item is in a given state. Skill gate", runItemRequire})
}

// currentItem resolves the item for a command run by the agent or a human.
// An explicit --item wins; otherwise the session decides (see session.Resolve).
func currentItem(repo *contract.Repo, flag string) (string, session.Source) {
	return session.Resolve(repo, flag, session.ID(""))
}

func runItemNew(e Env, args []string) int {
	f := fs("item new", e)
	title := f.String("title", "", "one line")
	kind := f.String("kind", "feature", "feature, fix or refactor")
	initiative := f.String("initiative", "", "the initiative this item belongs to")
	bind := f.Bool("bind", false, "bind this session to the new item")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	if f.NArg() == 0 || strings.TrimSpace(*title) == "" {
		fmt.Fprintln(e.Stderr, `usage: circle item new <id> --title "..." [--kind feature|fix|refactor] [--initiative <id>] [--bind]`)
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	if *initiative != "" {
		if _, err := initiativeLoad(repo, *initiative); err != nil {
			fmt.Fprintf(e.Stderr, "circle: %v\n", err)
			return ExitPrecond
		}
	}
	it := item.Item{
		ID: f.Arg(0), Title: *title, Kind: *kind, Initiative: *initiative,
		Author: gitEmail(repo.Root),
	}
	if err := item.New(repo, it); err != nil {
		if errors.Is(err, item.ErrExists) {
			fmt.Fprintf(e.Stderr, "circle: item %s already exists — `circle item show %s`\n", it.ID, it.ID)
			return ExitPrecond
		}
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	fmt.Fprintf(e.Stdout, "created item %s  %s\n", it.ID, it.Title)
	if *bind {
		return bindSession(e, repo, it.ID, "")
	}
	fmt.Fprintf(e.Stdout, "  bind a session to it: circle session bind --item %s\n", it.ID)
	fmt.Fprintf(e.Stdout, "  or work in its own worktree: claude --worktree %s\n", it.ID)
	return ExitOK
}

type itemView struct {
	item.Item
	State  item.State `json:"state"`
	Reason string     `json:"reason"`
	Tasks  int        `json:"tasks"`
	Closed int        `json:"closed"`
	Held   int        `json:"held"`
}

func viewOf(repo *contract.Repo, it item.Item) itemView {
	s := item.Derive(repo, it.ID)
	return itemView{it, s.State, s.Reason, len(s.Tasks), s.Closed, s.Held}
}

func runItemList(e Env, args []string) int {
	f := fs("item list", e)
	open := f.Bool("open", false, "only items not yet done")
	asJSON := f.Bool("json", false, "machine-readable")
	initiative := f.String("initiative", "", "only items of this initiative")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	items, err := item.List(repo)
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	var views []itemView
	for _, it := range items {
		if *initiative != "" && it.Initiative != *initiative {
			continue
		}
		v := viewOf(repo, it)
		if *open && v.State == item.Done {
			continue
		}
		views = append(views, v)
	}
	if *asJSON {
		return writeJSON(e, views)
	}
	if len(views) == 0 {
		fmt.Fprintln(e.Stdout, "no items — `circle item new <id> --title \"...\"`")
		return ExitOK
	}
	cur, _ := currentItem(repo, "")
	p := NewPalette(e.Stdout)
	for _, v := range views {
		mark := " "
		if v.ID == cur {
			mark = p.G + "▸" + p.N
		}
		fmt.Fprintf(e.Stdout, "%s %s%-20s%s %-10s %s", mark, p.B, v.ID, p.N, v.State, v.Title)
		if v.Tasks > 0 {
			fmt.Fprintf(e.Stdout, "  %s%d/%d closed%s", p.D, v.Closed, v.Tasks, p.N)
		}
		fmt.Fprintln(e.Stdout)
	}
	return ExitOK
}

func runItemShow(e Env, args []string) int {
	f := fs("item show", e)
	flag := f.String("item", "", "item id; defaults to this session's")
	asJSON := f.Bool("json", false, "machine-readable")
	var with multiFlag
	f.Var(&with, "with", "inline an artifact: shape, spec, brief, review, handoff, repro; repeatable")
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
	id, src := currentItem(repo, *flag)
	defer func() {
		if *asJSON {
			return // inlined markdown would corrupt the JSON
		}
		for _, w := range with {
			b, err := os.ReadFile(filepath.Join(item.Dir(repo, id), w+".md"))
			if err != nil {
				fmt.Fprintf(e.Stdout, "\n===== %s.md — not written yet =====\n", w)
				continue
			}
			fmt.Fprintf(e.Stdout, "\n===== %s.md =====\n%s\n", w, strings.TrimRight(string(b), "\n"))
		}
	}()
	it, err := item.Load(repo, id)
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitPrecond
	}
	v := viewOf(repo, it)
	if *asJSON {
		return writeJSON(e, v)
	}
	p := NewPalette(e.Stdout)
	fmt.Fprintf(e.Stdout, "%s%s%s  %s  %s(%s, resolved from %s)%s\n", p.B, v.ID, p.N, v.Title, p.D, v.Kind, src, p.N)
	if v.Initiative != "" {
		fmt.Fprintf(e.Stdout, "  initiative  %s\n", v.Initiative)
	}
	fmt.Fprintf(e.Stdout, "  state       %s%s%s  %s%s%s\n", p.B, v.State, p.N, p.D, v.Reason, p.N)
	ts, _ := task.LoadItem(repo, id)
	held := claims.All(repo.Root)
	for _, t := range ts {
		state, by := string(t.State), ""
		if c, ok := held[t.ID]; ok && t.State != domain.StateClosed {
			state, by = "held", "  — "+c.Holder+worktreeSuffix(c.Worktree)
		}
		fmt.Fprintf(e.Stdout, "    %-18s %-8s %s%s\n", t.ID, state, t.Title, by)
	}
	return ExitOK
}

// runItemRequire is the gate every orchestrator skill injects first. A skill
// declares the state it needs; when the item is elsewhere this exits 2, and an
// injected command exiting non-zero aborts the skill before the model sees it.
// That is what lets a team's own skill get the same enforcement as a built-in.
func runItemRequire(e Env, args []string) int {
	f := fs("item require", e)
	flag := f.String("item", "", "item id; defaults to this session's")
	var states multiFlag
	f.Var(&states, "state", "required state; repeat to accept any of several")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	if len(states) == 0 {
		fmt.Fprintln(e.Stderr, "usage: circle item require --state <state> [--state <state>]")
		return ExitError
	}
	for _, s := range states {
		if !item.ValidState(s) {
			fmt.Fprintf(e.Stderr, "circle: unknown state %q — one of %s\n", s, stateNames())
			return ExitError
		}
	}
	id, src := currentItem(repo, *flag)
	if id != task.DefaultItem && !session.Exists(repo, id) {
		fmt.Fprintf(e.Stderr, "circle: no item %q (resolved from %s) — `circle item new %s`\n", id, src, id)
		return ExitGate
	}
	got := item.Derive(repo, id)
	for _, s := range states {
		if string(got.State) == s {
			return ExitOK
		}
	}
	fmt.Fprintf(e.Stderr, "circle: item %s is %s (%s); this step needs %s.\n",
		id, got.State, got.Reason, strings.Join(states, " or "))
	fmt.Fprintln(e.Stderr, "Run `circle next` for the step that fits where it stands.")
	return ExitGate
}

func stateNames() string {
	var out []string
	for _, s := range item.States {
		out = append(out, string(s))
	}
	return strings.Join(out, ", ")
}

func worktreeSuffix(w string) string {
	if w == "" {
		return ""
	}
	return " in worktree " + w
}

// multiFlag collects a repeatable string flag.
type multiFlag []string

func (m *multiFlag) String() string     { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error { *m = append(*m, v); return nil }

func writeJSON(e Env, v any) int {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	fmt.Fprintln(e.Stdout, string(b))
	return ExitOK
}

func init() {
	register(Command{"spec check", "Validate the item's spec.md: sections, and every seam on a declared gate", runSpecCheck})
}

func runSpecCheck(e Env, args []string) int {
	f := fs("spec check", e)
	flag := f.String("item", "", "item id; defaults to this session's")
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
	id, _ := currentItem(repo, *flag)
	path := filepath.Join(item.Dir(repo, id), "spec.md")
	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: no spec for item %s — write .circle/items/%s/spec.md\n", id, id)
		return ExitPrecond
	}
	gates := map[string]bool{}
	for _, g := range repo.Contract.Quality.Gates() {
		gates[g.Name] = true
	}
	seams, problems := item.CheckSpec(string(b), gates)
	p := NewPalette(e.Stdout)
	if len(problems) > 0 {
		fmt.Fprintf(e.Stderr, "%scircle: the spec for %s cannot advance%s\n", p.R, id, p.N)
		for _, pr := range problems {
			fmt.Fprintf(e.Stderr, "  %s✗%s spec.md:%d  %s\n", p.R, p.N, pr.Line, pr.Reason)
		}
		fmt.Fprintf(e.Stderr, "\n%sdeclared gates: %s (or \"manual\" with a reason)%s\n", p.D, repo.Contract.Quality.GateNames(), p.N)
		return ExitGate
	}
	fmt.Fprintf(e.Stdout, "%s✓%s spec for %s is complete · %d seam(s), each on a declared gate\n", p.G, p.N, id, len(seams))
	return ExitOK
}
