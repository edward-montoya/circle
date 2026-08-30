package cli

import (
	"fmt"
	"strings"

	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/domain"
	"github.com/edwardmontoya/circle/internal/events"
	"github.com/edwardmontoya/circle/internal/task"
	"github.com/edwardmontoya/circle/internal/timeline"
)

func init() {
	register(Command{"status", "The terminal dashboard. One score, every point traced", runStatus})
	register(Command{"score explain", "Every signal behind the score", runScoreExplain})
	register(Command{"event record", "Hook target: append an event from stdin", runEventRecord})
}

// computeStatus gathers the evidence. Nothing here is inferred: every number
// comes from a recorded event, the task graph or git.
func computeStatus(repo *contract.Repo) (domain.Status, []domain.Task, domain.Timeline) {
	evs, _ := events.All(repo)
	gates := repo.Contract.Quality.Gates()
	pass, total := events.GateSummary(evs, gates)

	ts, _ := task.Load(repo)
	closed := 0
	for _, t := range ts {
		if t.State == domain.StateClosed {
			closed++
		}
	}

	tl, _ := timeline.Load(repo, "default")

	// Goal criteria in v0.1 are the tasks themselves: each carries a goal and a
	// verification, so "verified" means a gate proved it (D-35).
	goalMet := 0
	for _, t := range ts {
		if t.State == domain.StateClosed && t.ClosedBy != "" {
			goalMet++
		}
	}

	res := contract.Validate(repo)
	return domain.ComputeStatus(pass, total, goalMet, len(ts), closed, len(ts), 0, res.Forced), ts, tl
}

func runStatus(e Env, args []string) int {
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	s, ts, tl := computeStatus(repo)
	p := NewPalette(e.Stdout)

	fmt.Fprintf(e.Stdout, "\n%s%s%s  %s\n", p.B, repo.Contract.Project.Name, p.N,
		strings.TrimPrefix(tl.Branch, "refs/heads/"))

	det := s.DeterministicPct()
	fmt.Fprintf(e.Stdout, "\n  %sSTATUS%s  %s%d%s%s/100%s   %s%d%% deterministic%s",
		p.B, p.N, p.B, s.Total(), p.N, p.D, p.N, p.G, det, p.N)
	if s.Forced {
		fmt.Fprintf(e.Stdout, "  %s(capped: preflight forced)%s", p.Y, p.N)
	}
	fmt.Fprint(e.Stdout, "\n\n")

	labels := s.Labels()
	for i, c := range s.Components() {
		filled := 0
		if c.Weight > 0 {
			filled = c.Achieved * 24 / c.Weight
		}
		colour := p.G
		switch {
		case c.Achieved == 0:
			colour = p.R
		case c.Achieved < c.Weight:
			colour = p.Y
		}
		fmt.Fprintf(e.Stdout, "    %s%s%s%s%s  %-14s %s%2d/%d%s  %s%s%s\n",
			colour, strings.Repeat("█", filled), p.N,
			p.D, strings.Repeat("░", 24-filled),
			labels[i], p.B, c.Achieved, c.Weight, p.N, p.D, c.Detail, p.N)
	}

	res := contract.Validate(repo)
	fmt.Fprintf(e.Stdout, "\n  %sCONTRACTS%s   ", p.B, p.N)
	if res.Blocked() {
		fmt.Fprintf(e.Stdout, "%s%d blocking%s\n", p.R, res.Count(contract.Fail), p.N)
	} else {
		fmt.Fprintf(e.Stdout, "%svalid%s  %s%d advisory%s\n", p.G, p.N, p.D, res.Count(contract.Warn), p.N)
	}

	live, rew, ph, unc := tl.Counts()
	fmt.Fprintf(e.Stdout, "  %sTIMELINE%s    %d live · %d rewritten · %d uncommitted",
		p.B, p.N, live, rew, unc)
	if ph > 0 {
		fmt.Fprintf(e.Stdout, "  %s· %d phantom%s", p.R, ph, p.N)
	}
	fmt.Fprintln(e.Stdout)

	if u := task.Unverified(ts); len(u) > 0 {
		fmt.Fprintf(e.Stdout, "  %sINTEGRITY%s   %s%d task(s) closed with no passing gate%s\n",
			p.B, p.N, p.R, len(u), p.N)
	}

	ready := task.Ready(ts)
	if len(ready) > 0 {
		fmt.Fprintf(e.Stdout, "\n  %sNEXT%s\n", p.B, p.N)
		for _, t := range ready {
			fmt.Fprintf(e.Stdout, "    %scircle task claim %s%s   %s%s%s\n",
				p.B, t.ID, p.N, p.D, t.Title, p.N)
		}
	}
	fmt.Fprintln(e.Stdout)
	return ExitOK
}

func runScoreExplain(e Env, args []string) int {
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	s, ts, _ := computeStatus(repo)
	evs, _ := events.All(repo)
	p := NewPalette(e.Stdout)

	fmt.Fprintf(e.Stdout, "\n%sSCORE %d/100%s  %d%% deterministic\n\n", p.B, s.Total(), p.N, s.DeterministicPct())
	fmt.Fprintf(e.Stdout, "%sQuality gates — %d/%d%s\n", p.B, s.Gates.Achieved, s.Gates.Weight, p.N)
	latest := map[string]domain.Event{}
	for _, ev := range evs {
		if ev.Type == domain.EvGateRun {
			latest[ev.Gate] = ev
		}
	}
	for _, g := range repo.Contract.Quality.Gates() {
		if ev, ok := latest[g.Name]; ok {
			mark := p.G + "✓" + p.N
			if ev.ExitCode != 0 {
				mark = p.R + "✗" + p.N
			}
			fmt.Fprintf(e.Stdout, "  %s %-12s exit=%d  %s%s%s\n",
				mark, g.Name, ev.ExitCode, p.D, ev.At.Format("15:04:05"), p.N)
		} else {
			fmt.Fprintf(e.Stdout, "  %s?%s %-12s never run\n", p.Y, p.N, g.Name)
		}
	}

	fmt.Fprintf(e.Stdout, "\n%sTask closure — %d/%d%s\n", p.B, s.Tasks.Achieved, s.Tasks.Weight, p.N)
	for _, t := range ts {
		ev := "—"
		if t.ClosedBy != "" {
			ev = t.ClosedBy
		}
		fmt.Fprintf(e.Stdout, "  %-14s %-8s %s%s%s\n", t.ID, t.State, p.D, ev, p.N)
	}
	fmt.Fprintf(e.Stdout, "\n%sNo model contributed to this number.%s\n\n", p.D, p.N)
	return ExitOK
}

func runEventRecord(e Env, args []string) int {
	repo, _ := openRepo(e)
	if repo == nil {
		// A hook must never fail the thing it observes.
		return ExitOK
	}
	f := fs("event record", e)
	typ := f.String("type", string(domain.EvToolUse), "event type")
	item := f.String("item", "", "item id")
	taskID := f.String("task", "", "task id")
	reason := f.String("reason", "", "free-form context")
	if err := f.Parse(args); err != nil {
		return ExitOK
	}
	ev := domain.NewEvent(domain.EventType(*typ))
	ev.Item, ev.Task, ev.Reason = *item, *taskID, *reason
	_ = events.Append(repo, ev)
	return ExitOK
}
