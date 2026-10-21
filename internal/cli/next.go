package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/edwardmontoya/circle/internal/claims"
	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/domain"
	"github.com/edwardmontoya/circle/internal/events"
	"github.com/edwardmontoya/circle/internal/initiative"
	"github.com/edwardmontoya/circle/internal/item"
	"github.com/edwardmontoya/circle/internal/session"
	"github.com/edwardmontoya/circle/internal/task"
)

func init() {
	register(Command{"next", "Where this session stands, the next skill, and how to cross the phase boundary", runNext})
	register(Command{"event list", "The event log, filtered", runEventList})
}

// Suggestion is what `circle next` computes. Every field comes from recorded
// state; nothing is guessed from the conversation.
type Suggestion struct {
	Item     string `json:"item,omitempty"`
	Decision string `json:"decision,omitempty"`
	State    string `json:"state"`
	Reason   string `json:"reason"`
	Skill    string `json:"skill"`
	Command  string `json:"command,omitempty"`
	Boundary string `json:"boundary"`
	Why      string `json:"why"`
}

// boundaries is the phase-boundary move per state: continue when the next phase
// needs this one verbatim, clear when the context is disposable, a fresh context
// when the next phase should not inherit this one's reasoning.
var boundaries = map[item.State][3]string{
	item.Shaping:   {"/circle:shape", "continue", "nothing is settled yet; shape it"},
	item.Shaped:    {"/circle:spec", "continue", "the spec wants the shaping conversation verbatim — do not clear or compact"},
	item.Specified: {"/circle:tasks", "continue", "the tasks build on the same thinking; clear only after they exist"},
	item.Planned:   {"/circle:brief", "continue", "a human reads the brief next; the plan is on disk now"},
	item.Approved:  {"/circle:implement", "clear", "the task, spec and code carry everything; this context is disposable"},
	item.Building:  {"/circle:implement", "clear", "each task is self-contained; clear between tasks"},
	item.Reviewing: {"/circle:review", "fresh context", "the reviewer should read the diff, not inherit the builder's reasoning"},
	item.Done:      {"/circle:retro", "continue", "retro wants this session as its primary source; hand off only if the work travels"},
}

func computeNext(repo *contract.Repo) Suggestion {
	res := contract.Validate(repo)
	if res.Blocked() && !res.Incubating {
		return Suggestion{State: "blocked", Reason: "the contracts are invalid", Skill: "",
			Command: "circle preflight --explain", Boundary: "continue", Why: "nothing can be written until preflight passes"}
	}

	if b, ok := session.Lookup(repo.Root, session.ID("")); ok && b.Decision != "" && b.Item == "" {
		if d, err := initiative.GetDecision(repo, b.Decision); err == nil {
			if d.State == initiative.Open {
				return Suggestion{Decision: d.ID, State: "deciding", Reason: fmt.Sprintf("[%s] %s", d.Type, d.Title),
					Skill: "/circle:chart", Command: "circle decision show " + d.ID, Boundary: "continue",
					Why: "resolve this one decision, then hand off; one decision per session"}
			}
			return Suggestion{Decision: d.ID, State: "decided", Reason: d.Title + " is " + string(d.State),
				Skill: "/circle:handoff", Command: "circle initiative show " + d.Initiative, Boundary: "handoff",
				Why: "the decision is recorded; the next one belongs to a fresh session"}
		}
	}

	id, src := currentItem(repo, "")
	if id == task.DefaultItem && src == session.FromDefault {
		ts, _ := task.LoadItem(repo, task.DefaultItem)
		if len(ts) == 0 {
			if items, _ := item.List(repo); len(items) > 0 {
				var open []string
				for _, it := range items {
					if item.Derive(repo, it.ID).State != item.Done {
						open = append(open, it.ID)
					}
				}
				if len(open) > 0 {
					return Suggestion{State: "unbound", Reason: "open items: " + strings.Join(open, ", "),
						Skill: "/circle:start", Command: "circle session bind --item " + open[0], Boundary: "continue",
						Why: "a session works on exactly one item; pick one, or start a worktree per item"}
				}
			}
			return Suggestion{State: "idle", Reason: "no item yet", Skill: "/circle:shape",
				Command: `circle item new <id> --title "..." --bind`, Boundary: "continue",
				Why: "new work starts by shaping; reach for /circle:chart if it is too big to decide in one session"}
		}
	}

	st := item.Derive(repo, id)
	b := boundaries[st.State]
	s := Suggestion{Item: id, State: string(st.State), Reason: st.Reason, Skill: b[0], Boundary: b[1], Why: b[2]}
	switch st.State {
	case item.Shaping:
		s.Command = "circle item show " + id
	case item.Specified:
		s.Command = "circle spec check"
	case item.Planned:
		s.Command = "circle brief generate && circle brief approve"
	case item.Approved, item.Building:
		ts, _ := task.Load(repo)
		if mine := heldBy(repo, holder(repo)); mine != "" {
			if t, err := task.Get(repo, mine); err == nil && task.ItemOf(t) == id && t.State != domain.StateClosed {
				s.Command = "circle task show --current"
				s.Boundary = "continue"
				s.Why = fmt.Sprintf("you hold %s — finish it: red → green at its gate, commit, then `circle task close %s`", mine, mine)
				break
			}
		}
		if ready := readyTasks(repo, ts, id, false); len(ready) > 0 {
			s.Command = "circle task claim " + ready[0].ID
		} else {
			held := 0
			for _, c := range claims.All(repo.Root) {
				if c.Item == id && c.Held() {
					held++
				}
			}
			s.Command = "circle task ready"
			s.Why = fmt.Sprintf("no task is ready: %d held elsewhere, the rest blocked — wait or help review", held)
			s.Boundary = "continue"
		}
	case item.Reviewing:
		s.Command = "circle review drift"
	case item.Done:
		s.Command = "circle event list --types gate.deny,gate.run,skill.run"
	}
	return s
}

func runNext(e Env, args []string) int {
	f := fs("next", e)
	asJSON := f.Bool("json", false, "machine-readable")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		fmt.Fprintln(e.Stdout, "not a Circle repository — run `circle init`")
		return code
	}
	s := computeNext(repo)
	if *asJSON {
		return writeJSON(e, s)
	}
	p := NewPalette(e.Stdout)
	where := s.Item
	if s.Decision != "" {
		where = "decision " + s.Decision
	}
	if where == "" {
		where = "this session"
	}
	fmt.Fprintf(e.Stdout, "%s%s%s is %s%s%s — %s\n", p.B, where, p.N, p.B, s.State, p.N, s.Reason)
	if s.Skill != "" {
		fmt.Fprintf(e.Stdout, "  next      %s%s%s\n", p.G, s.Skill, p.N)
	}
	if s.Command != "" {
		fmt.Fprintf(e.Stdout, "  command   %s\n", s.Command)
	}
	fmt.Fprintf(e.Stdout, "  boundary  %s — %s\n", s.Boundary, s.Why)
	return ExitOK
}

func runEventList(e Env, args []string) int {
	f := fs("event list", e)
	flag := f.String("item", "", "only this item; defaults to this session's")
	all := f.Bool("all", false, "every item")
	types := f.String("types", "", "comma-separated event types")
	since := f.Duration("since", 0, "only events newer than this, e.g. 24h")
	limit := f.Int("limit", 200, "at most this many, newest last")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	id, _ := currentItem(repo, *flag)
	want := map[string]bool{}
	for _, t := range splitList(*types) {
		want[t] = true
	}
	evs, _ := events.All(repo)
	var out []domain.Event
	for _, ev := range evs {
		if !*all && ev.Item != "" && ev.Item != id {
			continue
		}
		if len(want) > 0 && !want[string(ev.Type)] {
			continue
		}
		if *since > 0 && ev.At.Before(time.Now().Add(-*since)) {
			continue
		}
		out = append(out, ev)
	}
	if *limit < 0 {
		fmt.Fprintln(e.Stderr, "circle: --limit must be zero or more")
		return ExitError
	}
	if len(out) > *limit {
		out = out[len(out)-*limit:]
	}
	for _, ev := range out {
		detail := ev.Reason
		if ev.Gate != "" {
			detail = fmt.Sprintf("%s exit=%d %s", ev.Gate, ev.ExitCode, ev.Reason)
		}
		if ev.Path != "" {
			detail += " " + ev.Path
		}
		fmt.Fprintf(e.Stdout, "%s  %-18s %-14s %-12s %s\n", ev.At.Format("2006-01-02 15:04:05"), ev.Type, ev.Item, ev.Task, strings.TrimSpace(detail))
	}
	if len(out) == 0 {
		fmt.Fprintln(e.Stdout, "no matching events")
	}
	return ExitOK
}
