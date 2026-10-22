package cli

import (
	"errors"
	"fmt"
	"os/user"
	"strings"
	"time"

	"github.com/edwardmontoya/circle/internal/claims"
	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/domain"
	"github.com/edwardmontoya/circle/internal/events"
	"github.com/edwardmontoya/circle/internal/session"
	"github.com/edwardmontoya/circle/internal/task"
)

func init() {
	register(Command{"task create", "Create a task. Rejects a missing goal or verification", runTaskCreate})
	register(Command{"task validate", "Check the task contract. TaskCreated hook target", runTaskValidate})
	register(Command{"task ready", "Claimable tasks, with their goals", runTaskReady})
	register(Command{"task claim", "Take a task. Visible to every worktree", runTaskClaim})
	register(Command{"task release", "Give a claimed task back", runTaskRelease})
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
	id := f.String("id", "", "task id; generated from the item when omitted")
	itemFlag := f.String("item", "", "the item this task belongs to; defaults to this session's")
	parent := f.String("parent", "", "parent task")
	title := f.String("title", "", "one line")
	goal := f.String("goal", "", "REQUIRED: one sentence, what done means for this task alone")
	kind := f.String("verify-kind", "", "REQUIRED: "+domain.VerifyKinds())
	gate := f.String("verify-gate", "", "a gate declared in the quality contract")
	command := f.String("verify-command", "", "the exact command, if narrower than the gate")
	justification := f.String("justification", "", "manual only: why no command can prove this")
	reviewer := f.String("reviewer", "", "manual only: who judges it")
	deps := f.String("depends-on", "", "comma-separated task ids")
	paths := f.String("paths", "", "comma-separated globs this task may write to — its blast radius")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}

	itemID, _ := currentItem(repo, *itemFlag)
	if itemID != task.DefaultItem && !session.Exists(repo, itemID) {
		fmt.Fprintf(e.Stderr, "circle: no item %q — `circle item new %s --title \"...\"`\n", itemID, itemID)
		return ExitPrecond
	}
	if *id == "" && strings.TrimSpace(*title) != "" {
		*id = task.NewID(itemID, *title)
	}
	if *id != "" {
		if existing, err := task.Get(repo, *id); err == nil {
			fmt.Fprintf(e.Stderr, "circle: task %s already exists in item %s\n", *id, task.ItemOf(existing))
			return ExitGate
		}
	}
	stored := itemID
	if stored == task.DefaultItem {
		stored = ""
	}

	t := domain.Task{
		ID: *id, Item: stored, Parent: *parent, Title: *title, State: domain.StateReady,
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

	if errs := append(t.Validate(repo.Contract.Quality), dependencyErrors(repo, t)...); len(errs) > 0 {
		reportTaskErrors(e, t.ID, errs)
		return ExitGate // the TaskCreated hook maps this to "prevent creation"
	}
	if err := task.Save(repo, t); err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	ev := domain.NewEvent(domain.EvTaskCreate)
	ev.Task, ev.Item = t.ID, itemID
	_ = events.Append(repo, ev)

	fmt.Fprintf(e.Stdout, "created %s  %s  %s(item %s)%s\n", t.ID, t.Title, NewPalette(e.Stdout).D, itemID, NewPalette(e.Stdout).N)
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
	f := fs("task validate", e)
	flag := f.String("item", "", "only this item; defaults to this session's")
	all := f.Bool("all", false, "every item")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	var ts []domain.Task
	var err error
	if *all {
		ts, err = task.Load(repo)
	} else {
		id, _ := currentItem(repo, *flag)
		ts, err = task.LoadItem(repo, id)
	}
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	bad := 0
	for _, t := range ts {
		if errs := append(t.Validate(repo.Contract.Quality), dependencyErrors(repo, t)...); len(errs) > 0 {
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
	f := fs("task ready", e)
	itemFlag := f.String("item", "", "only this item's tasks; defaults to this session's")
	all := f.Bool("all", false, "every item's ready tasks")
	asJSON := f.Bool("json", false, "machine-readable")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	if f.NArg() > 0 {
		*itemFlag = f.Arg(0)
	}
	ts, _ := task.Load(repo)
	itemID, _ := currentItem(repo, *itemFlag)
	ready := readyTasks(repo, ts, itemID, *all)
	if *asJSON {
		return writeJSON(e, ready)
	}
	if len(ready) == 0 {
		fmt.Fprintln(e.Stdout, "no claimable tasks")
		return ExitOK
	}
	p := NewPalette(e.Stdout)
	// The goal and verification print alongside the title on purpose: a
	// cold-resuming agent must know what done means without reading the brief.
	for _, t := range ready {
		fmt.Fprintf(e.Stdout, "%s%-14s%s %s", p.B, t.ID, p.N, t.Title)
		if *all {
			fmt.Fprintf(e.Stdout, "  %s(item %s)%s", p.D, task.ItemOf(t), p.N)
		}
		fmt.Fprintln(e.Stdout)
		fmt.Fprintf(e.Stdout, "  %sgoal%s    %s\n", p.D, p.N, t.Goal.Statement)
		fmt.Fprintf(e.Stdout, "  %sverify%s  %s via %s\n", p.D, p.N, t.Verify.Kind, t.Verify.Gate)
	}
	return ExitOK
}

// readyTasks is the ready set: open, unblocked, and unclaimed in the shared
// ledger — so a task another worktree took a second ago is already gone.
func readyTasks(repo *contract.Repo, ts []domain.Task, itemID string, all bool) []domain.Task {
	held := claims.All(repo.Root)
	var out []domain.Task
	for _, t := range task.Ready(ts) {
		if !all && task.ItemOf(t) != itemID {
			continue
		}
		if _, ok := held[t.ID]; ok {
			continue
		}
		out = append(out, t)
	}
	return out
}

// holder identifies who is claiming: the person, and the session or worktree
// they are claiming from, so a second implementer sees exactly where the task
// went.
func holder(repo *contract.Repo) claims.Claim {
	who := gitEmail(repo.Root)
	if who == "" {
		who = "unknown"
		if u, err := user.Current(); err == nil {
			who = u.Username
		}
	}
	return claims.Claim{
		Holder: who, Session: session.ID(""), Worktree: session.WorktreeName(repo.Root),
		At: time.Now().UTC(),
	}
}

func runTaskClaim(e Env, args []string) int {
	f := fs("task claim", e)
	steal := f.Bool("steal", false, "take a task someone else holds")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	if f.NArg() == 0 {
		fmt.Fprintln(e.Stderr, "usage: circle task claim <id> [--steal]")
		return ExitError
	}
	t, err := task.Get(repo, f.Arg(0))
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	if t.State == domain.StateClosed {
		fmt.Fprintf(e.Stderr, "circle: %s is already closed\n", t.ID)
		return ExitGate
	}
	ts, _ := task.Load(repo)
	if t.Blocked(task.ByID(ts)) {
		fmt.Fprintf(e.Stderr, "circle: %s is blocked on %s\n", t.ID, strings.Join(t.DependsOn, ", "))
		return ExitGate
	}
	c := holder(repo)
	c.Task, c.Item = t.ID, task.ItemOf(t)
	// A session works on one item. Claiming a task binds an unbound session to
	// the task's item; a session bound elsewhere must say so first, rather than
	// quietly start work on a second item.
	if c.Session != "" {
		if b, ok := session.Lookup(repo.Root, c.Session); ok && b.Item != "" && b.Item != c.Item {
			fmt.Fprintf(e.Stderr, "circle: this session works on item %s; %s belongs to %s. "+
				"Finish or hand off first, or `circle session bind --item %s`.\n", b.Item, t.ID, c.Item, c.Item)
			return ExitGate
		} else if !ok || b.Item == "" {
			_ = session.Bind(repo.Root, session.Binding{Session: c.Session, Item: c.Item, Worktree: c.Worktree})
		}
	}
	if err := claims.Take(repo.Root, c, *steal); err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		var held claims.HeldError
		var closed claims.ClosedError
		if errors.As(err, &closed) {
			return ExitGate
		}
		if errors.As(err, &held) {
			fmt.Fprintln(e.Stderr, "Pick another from `circle task ready`, or pass --steal if the holder is gone.")
			return ExitGate
		}
		return ExitError
	}
	ev := domain.NewEvent(domain.EvTaskClaim)
	ev.Task, ev.Item, ev.Actor, ev.Session = t.ID, c.Item, c.Holder, c.Session
	_ = events.Append(repo, ev)
	fmt.Fprintf(e.Stdout, "claimed %s by %s\n", t.ID, c.Holder)
	return ExitOK
}

func runTaskRelease(e Env, args []string) int {
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	if len(args) == 0 {
		fmt.Fprintln(e.Stderr, "usage: circle task release <id>")
		return ExitError
	}
	if err := claims.Release(repo.Root, args[0]); err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	fmt.Fprintf(e.Stdout, "released %s\n", args[0])
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
	if err := parse(f, args); err != nil {
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
		since := lastCommitTime(repo, t)
		ev, passed := events.GatePassedSince(evs, t.Verify.Gate, since)
		// A task that declared a command narrower than its gate is proven by
		// that command, run for this task — not by the gate's broader run.
		if t.Verify.Command != "" {
			ev, passed = events.TaskGatePassedSince(evs, t.Verify.Gate, t.ID, since)
			if !passed && ev.Gate == "" {
				fmt.Fprintf(e.Stderr, "circle: %s is verified by its own command; run it with `circle quality run --task %s`\n", t.ID, t.ID)
			}
		}
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
	c := holder(repo)
	c.Task, c.Item = t.ID, task.ItemOf(t)
	_ = claims.Close(repo.Root, c)
	ev := domain.NewEvent(domain.EvTaskClose)
	ev.Task, ev.Item, ev.Gate = t.ID, task.ItemOf(t), t.Verify.Gate
	_ = events.Append(repo, ev)
	fmt.Fprintf(e.Stdout, "closed %s  evidence: %s\n", t.ID, t.ClosedBy)
	fmt.Fprintln(e.Stdout, "  the closure is recorded in .circle/ — it lands with your next commit")
	return ExitOK
}

func runTaskList(e Env, args []string) int {
	f := fs("task list", e)
	unverified := f.Bool("unverified", false, "tasks closed with no passing gate — should be empty")
	if err := parse(f, args); err != nil {
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
	f := fs("task show", e)
	current := f.Bool("current", false, "the task this session holds")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	id := f.Arg(0)
	if *current {
		id = heldBy(repo, holder(repo))
		if id == "" {
			fmt.Fprintln(e.Stderr, "circle: this session holds no task — `circle task ready`, then `circle task claim <id>`")
			return ExitPrecond
		}
	}
	if id == "" {
		fmt.Fprintln(e.Stderr, "usage: circle task show <id> | --current")
		return ExitError
	}
	t, err := task.Get(repo, id)
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	p := NewPalette(e.Stdout)
	fmt.Fprintf(e.Stdout, "%s%s%s  %s  %s(item %s)%s\n\n", p.B, t.ID, p.N, t.Title, p.D, task.ItemOf(t), p.N)
	if len(t.Paths) > 0 {
		fmt.Fprintf(e.Stdout, "%sPATHS%s\n  %s\n\n", p.B, p.N, strings.Join(t.Paths, " "))
	}
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
	if c, ok := claims.All(repo.Root)[t.ID]; ok && c.Held() {
		fmt.Fprintf(e.Stdout, "  held by %s", c.Holder)
		if c.Worktree != "" {
			fmt.Fprintf(e.Stdout, " in %s", c.Worktree)
		}
	}
	if t.ClosedBy != "" {
		fmt.Fprintf(e.Stdout, "  evidence: %s", t.ClosedBy)
	}
	fmt.Fprintln(e.Stdout)
	return ExitOK
}

// heldBy finds the open task a holder has claimed, preferring the session match.
func heldBy(repo *contract.Repo, me claims.Claim) string {
	for _, c := range claims.All(repo.Root) {
		if c.Closed {
			continue
		}
		if me.Session != "" && c.Session == me.Session {
			return c.Task
		}
		if me.Session == "" && c.Holder == me.Holder && c.Worktree == me.Worktree {
			return c.Task
		}
	}
	return ""
}

func init() {
	register(Command{"task update", "Change an open task's paths, gate, goal or dependencies. Voids the approval", runTaskUpdate})
}

// runTaskUpdate changes an open task. The plan hash covers goals, gates, paths
// and dependencies, so any change here makes the approval stale and the human
// approves the changed plan — a task cannot quietly widen its own radius.
func runTaskUpdate(e Env, args []string) int {
	f := fs("task update", e)
	title := f.String("title", "", "new title")
	goal := f.String("goal", "", "new goal")
	kind := f.String("verify-kind", "", "new verification kind")
	gate := f.String("verify-gate", "", "new gate")
	paths := f.String("paths", "", "replace the paths (comma-separated globs)")
	addPaths := f.String("add-paths", "", "add paths (comma-separated globs)")
	deps := f.String("depends-on", "", "replace the dependencies (comma-separated ids)")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	if f.NArg() == 0 {
		fmt.Fprintln(e.Stderr, "usage: circle task update <id> [--goal] [--verify-kind] [--verify-gate] [--paths|--add-paths] [--depends-on] [--title]")
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	t, err := task.Get(repo, f.Arg(0))
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	if t.State == domain.StateClosed {
		fmt.Fprintf(e.Stderr, "circle: %s is closed; a change to closed work is a new task\n", t.ID)
		return ExitGate
	}
	if *title != "" {
		t.Title = *title
	}
	if *goal != "" {
		t.Goal.Statement = *goal
	}
	if *kind != "" {
		t.Verify.Kind = domain.VerifyKind(*kind)
	}
	if *gate != "" {
		t.Verify.Gate = *gate
	}
	if *paths != "" {
		t.Paths = splitList(*paths)
	}
	for _, p := range splitList(*addPaths) {
		if !contains(t.Paths, p) {
			t.Paths = append(t.Paths, p)
		}
	}
	if *deps != "" {
		t.DependsOn = splitList(*deps)
	}
	if errs := append(t.Validate(repo.Contract.Quality), dependencyErrors(repo, t)...); len(errs) > 0 {
		reportTaskErrors(e, t.ID, errs)
		return ExitGate
	}
	if err := task.Save(repo, t); err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	ev := domain.NewEvent(domain.EvTaskCreate)
	ev.Task, ev.Item, ev.Reason = t.ID, task.ItemOf(t), "updated"
	_ = events.Append(repo, ev)
	fmt.Fprintf(e.Stdout, "updated %s — the plan changed, so item %s needs its brief re-approved\n", t.ID, task.ItemOf(t))
	fmt.Fprintf(e.Stdout, "  paths   %s\n  verify  %s via %s\n", strings.Join(t.Paths, " "), t.Verify.Kind, t.Verify.Gate)
	return ExitOK
}

// dependencyErrors rejects a dependency that names no task, or the task itself.
// A made-up id would leave the task blocked forever: Blocked treats a missing
// dependency as open, so the task never reaches the ready set and nothing says
// why.
func dependencyErrors(repo *contract.Repo, t domain.Task) []domain.ValidationError {
	all, _ := task.Load(repo)
	idx := task.ByID(all)
	var errs []domain.ValidationError
	for _, d := range t.DependsOn {
		switch {
		case d == t.ID:
			errs = append(errs, domain.ValidationError{Field: "depends_on", Reason: "a task cannot depend on itself"})
		case idx[d].ID == "":
			errs = append(errs, domain.ValidationError{Field: "depends_on", Reason: "no task " + d,
				Fix: "create the blocker first, or name an existing id (`circle task list`)"})
		}
	}
	return errs
}
