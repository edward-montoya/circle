package cli

import (
	"fmt"
	"os/user"
	"strings"
	"time"

	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/domain"
	"github.com/edwardmontoya/circle/internal/events"
	"github.com/edwardmontoya/circle/internal/task"
)

func init() {
	register(Command{"task create", "Create a task. Rejects a missing goal or verification", runTaskCreate})
	register(Command{"task validate", "Check the task contract. TaskCreated hook target", runTaskValidate})
	register(Command{"task ready", "Claimable tasks, with their goals", runTaskReady})
	register(Command{"task claim", "Take a task", runTaskClaim})
	register(Command{"task close", "Close a task. Requires a passing gate since its last commit", runTaskClose})
	register(Command{"task list", "The graph", runTaskList})
	register(Command{"task show", "Goal, verification, evidence", runTaskShow})
}

func openRepo(e Env) (*contract.Repo, int) {
	repo, err := contract.Open(e.Cwd)
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return nil, ExitPrecond
	}
	return repo, 0
}

func runTaskCreate(e Env, args []string) int {
	f := fs("task create", e)
	id := f.String("id", "", "task id, e.g. task-118.3")
	parent := f.String("parent", "", "parent item or task")
	title := f.String("title", "", "one line")
	goal := f.String("goal", "", "REQUIRED: one sentence, what done means for this task alone")
	kind := f.String("verify-kind", "", "REQUIRED: "+domain.VerifyKinds())
	gate := f.String("verify-gate", "", "a gate declared in the quality contract")
	command := f.String("verify-command", "", "the exact command, if narrower than the gate")
	justification := f.String("justification", "", "manual only: why no command can prove this")
	reviewer := f.String("reviewer", "", "manual only: who judges it")
	deps := f.String("depends-on", "", "comma-separated task ids")
	paths := f.String("paths", "", "comma-separated globs this task may write to — its blast radius")
	if err := f.Parse(args); err != nil {
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}

	t := domain.Task{
		ID: *id, Parent: *parent, Title: *title, State: domain.StateReady,
		Goal: domain.Goal{Statement: *goal},
		Verify: domain.Verification{
			Kind: domain.VerifyKind(*kind), Gate: *gate, Command: *command,
			Justification: *justification, Reviewer: *reviewer,
		},
	}
	for _, d := range strings.Split(*deps, ",") {
		if d = strings.TrimSpace(d); d != "" {
			t.DependsOn = append(t.DependsOn, d)
		}
	}
	// Per-task rather than per-item: a four-task item cannot otherwise express
	// that one task touches src/mw/ and another touches Makefile, and the
	// write-block would have to approve the union of everything.
	for _, p := range strings.Split(*paths, ",") {
		if p = strings.TrimSpace(p); p != "" {
			t.Paths = append(t.Paths, p)
		}
	}

	if errs := t.Validate(repo.Contract.Quality); len(errs) > 0 {
		reportTaskErrors(e, t.ID, errs)
		return ExitGate // the TaskCreated hook maps this to "prevent creation"
	}
	if err := task.Save(repo, t); err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	ev := domain.NewEvent(domain.EvTaskCreate)
	ev.Task, ev.Item = t.ID, t.Parent
	_ = events.Append(repo, ev)

	fmt.Fprintf(e.Stdout, "created %s  %s\n", t.ID, t.Title)
	fmt.Fprintf(e.Stdout, "  goal    %s\n", t.Goal.Statement)
	fmt.Fprintf(e.Stdout, "  verify  %s via %s\n", t.Verify.Kind, t.Verify.Gate)
	return ExitOK
}

func reportTaskErrors(e Env, id string, errs []domain.ValidationError) {
	p := NewPalette(e.Stderr)
	fmt.Fprintf(e.Stderr, "%scircle: task %s rejected — the task contract is not satisfied%s\n",
		p.R, id, p.N)
	for _, er := range errs {
		fmt.Fprintf(e.Stderr, "  %s✗%s %-26s %s\n", p.R, p.N, er.Field, er.Reason)
		if er.Fix != "" {
			fmt.Fprintf(e.Stderr, "     %s└ %s%s\n", p.D, er.Fix, p.N)
		}
	}
	fmt.Fprintf(e.Stderr, "\n%sA task without a goal is a prompt. A task without a verification\n"+
		"closes on the agent's word. Neither is created.%s\n", p.D, p.N)
}

func runTaskValidate(e Env, args []string) int {
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	ts, err := task.Load(repo)
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	bad := 0
	for _, t := range ts {
		if errs := t.Validate(repo.Contract.Quality); len(errs) > 0 {
			reportTaskErrors(e, t.ID, errs)
			bad++
		}
	}
	if u := task.Unverified(ts); len(u) > 0 {
		// Should be impossible. If it happens the hook was bypassed.
		for _, t := range u {
			fmt.Fprintf(e.Stderr, "circle: %s is closed with no passing gate behind it\n", t.ID)
		}
		bad += len(u)
	}
	if bad > 0 {
		return ExitGate
	}
	auto, total := domain.MachineCheckable(ts)
	fmt.Fprintf(e.Stdout, "%d tasks valid · %d of %d machine-checkable\n", len(ts), auto, total)
	return ExitOK
}

func runTaskReady(e Env, args []string) int {
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	ts, _ := task.Load(repo)
	ready := task.Ready(ts)
	if len(ready) == 0 {
		fmt.Fprintln(e.Stdout, "no claimable tasks")
		return ExitOK
	}
	p := NewPalette(e.Stdout)
	// The goal and verification print alongside the title on purpose: a
	// cold-resuming agent must know what done means without reading the brief.
	for _, t := range ready {
		fmt.Fprintf(e.Stdout, "%s%-14s%s %s\n", p.B, t.ID, p.N, t.Title)
		fmt.Fprintf(e.Stdout, "  %sgoal%s    %s\n", p.D, p.N, t.Goal.Statement)
		fmt.Fprintf(e.Stdout, "  %sverify%s  %s via %s\n", p.D, p.N, t.Verify.Kind, t.Verify.Gate)
	}
	return ExitOK
}

func runTaskClaim(e Env, args []string) int {
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	if len(args) == 0 {
		fmt.Fprintln(e.Stderr, "usage: circle task claim <id>")
		return ExitError
	}
	t, err := task.Get(repo, args[0])
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	ts, _ := task.Load(repo)
	if t.Blocked(task.ByID(ts)) {
		fmt.Fprintf(e.Stderr, "circle: %s is blocked on %s\n", t.ID, strings.Join(t.DependsOn, ", "))
		return ExitGate
	}
	who := "unknown"
	if u, err := user.Current(); err == nil {
		who = u.Username
	}
	t.State, t.ClaimedBy = domain.StateClaimed, who
	_ = task.Save(repo, t)
	ev := domain.NewEvent(domain.EvTaskClaim)
	ev.Task, ev.Actor = t.ID, who
	_ = events.Append(repo, ev)
	fmt.Fprintf(e.Stdout, "claimed %s by %s\n", t.ID, who)
	return ExitOK
}

// runTaskClose is where the task contract earns its keep.
//
// A task closes because a named command exited zero after the work, not because
// an agent said so. That moves the 20-point task-closure component of the status
// score from claimed to proven (D-35).
func runTaskClose(e Env, args []string) int {
	f := fs("task close", e)
	verifyOnly := f.Bool("verify-only", false, "check without closing. TaskCompleted hook target")
	if err := f.Parse(args); err != nil {
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	if f.NArg() == 0 {
		fmt.Fprintln(e.Stderr, "usage: circle task close <id>")
		return ExitError
	}
	t, err := task.Get(repo, f.Arg(0))
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}

	if t.Verify.Kind == domain.VerifyManual {
		fmt.Fprintf(e.Stdout, "%s is manual verification — %s must judge it.\n", t.ID, t.Verify.Reviewer)
		fmt.Fprintf(e.Stdout, "justification: %s\n", t.Verify.Justification)
		if verifyOnly != nil && *verifyOnly {
			return ExitOK
		}
	} else {
		evs, _ := events.All(repo)
		since := lastCommitTime(repo, t.ID)
		ev, passed := events.GatePassedSince(evs, t.Verify.Gate, since)
		if !passed {
			p := NewPalette(e.Stderr)
			fmt.Fprintf(e.Stderr, "%scircle: %s cannot close%s\n", p.R, t.ID, p.N)
			fmt.Fprintf(e.Stderr, "  gate %s has no passing run since %s\n",
				t.Verify.Gate, since.Format(time.RFC3339))
			if ev.Gate != "" {
				fmt.Fprintf(e.Stderr, "  last run exit=%d at %s\n", ev.ExitCode, ev.At.Format(time.RFC3339))
			}
			fmt.Fprintf(e.Stderr, "\n  %srun: circle quality run %s%s\n", p.D, t.Verify.Gate, p.N)
			return ExitQuality
		}
		t.ClosedBy = fmt.Sprintf("%s@%s", ev.Gate, ev.At.Format(time.RFC3339))
	}

	if *verifyOnly {
		fmt.Fprintf(e.Stdout, "%s may close: %s\n", t.ID, t.ClosedBy)
		return ExitOK
	}

	t.State, t.ClosedAt = domain.StateClosed, time.Now().UTC().Format(time.RFC3339)
	_ = task.Save(repo, t)
	ev := domain.NewEvent(domain.EvTaskClose)
	ev.Task, ev.Gate = t.ID, t.Verify.Gate
	_ = events.Append(repo, ev)
	fmt.Fprintf(e.Stdout, "closed %s  evidence: %s\n", t.ID, t.ClosedBy)
	return ExitOK
}

func runTaskList(e Env, args []string) int {
	f := fs("task list", e)
	unverified := f.Bool("unverified", false, "tasks closed with no passing gate — should be empty")
	if err := f.Parse(args); err != nil {
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	ts, _ := task.Load(repo)
	if *unverified {
		u := task.Unverified(ts)
		for _, t := range u {
			fmt.Fprintf(e.Stdout, "%-14s %s\n", t.ID, t.Title)
		}
		if len(u) == 0 {
			fmt.Fprintln(e.Stdout, "none — every closed task has a passing gate behind it")
		}
		return ExitOK
	}
	p := NewPalette(e.Stdout)
	idx := task.ByID(ts)
	for _, t := range ts {
		state := string(t.State)
		if t.State == domain.StateReady && t.Blocked(idx) {
			state = "blocked"
		}
		colour := p.D
		switch state {
		case "closed":
			colour = p.G
		case "blocked":
			colour = p.Y
		}
		fmt.Fprintf(e.Stdout, "%-14s %s%-8s%s %-5s %s\n",
			t.ID, colour, state, p.N, t.Verify.Kind, t.Title)
	}
	auto, total := domain.MachineCheckable(ts)
	fmt.Fprintf(e.Stdout, "\n%d tasks · %d of %d machine-checkable\n", len(ts), auto, total)
	return ExitOK
}

func runTaskShow(e Env, args []string) int {
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	if len(args) == 0 {
		fmt.Fprintln(e.Stderr, "usage: circle task show <id>")
		return ExitError
	}
	t, err := task.Get(repo, args[0])
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	p := NewPalette(e.Stdout)
	fmt.Fprintf(e.Stdout, "%s%s%s  %s\n\n", p.B, t.ID, p.N, t.Title)
	fmt.Fprintf(e.Stdout, "%sGOAL%s\n  %s\n\n", p.B, p.N, t.Goal.Statement)
	fmt.Fprintf(e.Stdout, "%sVERIFICATION%s\n", p.B, p.N)
	fmt.Fprintf(e.Stdout, "  kind     %s\n", t.Verify.Kind)
	if t.Verify.Gate != "" {
		fmt.Fprintf(e.Stdout, "  gate     %s\n", t.Verify.Gate)
	}
	if t.Verify.Command != "" {
		fmt.Fprintf(e.Stdout, "  command  %s\n", t.Verify.Command)
	}
	if t.Verify.Kind == domain.VerifyManual {
		fmt.Fprintf(e.Stdout, "  reviewer %s\n  why      %s\n", t.Verify.Reviewer, t.Verify.Justification)
	}
	fmt.Fprintf(e.Stdout, "\n%sSTATE%s\n  %s", p.B, p.N, t.State)
	if t.ClosedBy != "" {
		fmt.Fprintf(e.Stdout, "  evidence: %s", t.ClosedBy)
	}
	fmt.Fprintln(e.Stdout)
	return ExitOK
}
