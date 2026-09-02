package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/domain"
	"github.com/edwardmontoya/circle/internal/events"
	"github.com/edwardmontoya/circle/internal/worktree"
)

func init() {
	register(Command{"worktree provision", "WorktreeCreate hook target: Compose namespace + ports", runWorktreeProvision})
	register(Command{"worktree release", "WorktreeRemove hook target: compose down -v, free the ports", runWorktreeRelease})
	register(Command{"worktree list", "Live port leases across every worktree of this repo", runWorktreeList})
	register(Command{"run", "Print the exact commands to bring the stack up", runRun})
}

// worktreePayload is the WorktreeCreate/WorktreeRemove hook input.
//
// Path is where Claude Code put the worktree. As everywhere else, we trust the
// payload over ${CLAUDE_PROJECT_DIR}, which still points at the main checkout.
type worktreePayload struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Cwd  string `json:"cwd"`
}

func readWorktreePayload(e Env) worktreePayload {
	var p worktreePayload
	b, err := io.ReadAll(e.Stdin)
	if err == nil && len(b) > 0 {
		_ = json.Unmarshal(b, &p)
	}
	return p
}

// defaultPortRange is used when the contract does not reserve one. Well above
// the ephemeral range on Linux and macOS, so it will not fight the kernel.
var defaultPortRange = [2]int{42000, 42999}

func runWorktreeProvision(e Env, args []string) int {
	f := fs("worktree provision", e)
	item := f.String("item", "", "work item id; defaults to the worktree name")
	dir := f.String("dir", "", "worktree directory; defaults to the hook payload or cwd")
	hook := f.Bool("hook", false, "read a WorktreeCreate payload from stdin")
	dry := f.Bool("dry-run", false, "print the plan, write nothing")
	if err := parse(f, args); err != nil {
		return ExitError
	}

	target, name := *dir, *item
	if *hook {
		p := readWorktreePayload(e)
		if p.Path != "" {
			target = p.Path
		}
		if name == "" {
			name = p.Name
		}
	}
	if target == "" {
		target = e.Cwd
	}
	if name == "" {
		name = filepath.Base(target)
	}

	repo, err := contract.Open(target)
	if err != nil {
		// Not a Circle repo, or not scaffolded. A WorktreeCreate hook that fails
		// aborts worktree creation entirely, so this must not be fatal.
		if *hook {
			return ExitOK
		}
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitPrecond
	}
	ex := repo.Contract.Execution
	if ex.Compose == "" {
		if *hook {
			return ExitOK
		}
		fmt.Fprintln(e.Stderr, "circle: no compose file in the execution contract")
		return ExitPrecond
	}

	if n, err := worktree.Prune(target); err == nil && n > 0 {
		fmt.Fprintf(e.Stderr, "circle: released %d lease(s) from worktrees that no longer exist\n", n)
	}

	lo, hi := ex.Worktree.PortRange[0], ex.Worktree.PortRange[1]
	if lo == 0 || hi <= lo {
		lo, hi = defaultPortRange[0], defaultPortRange[1]
	}

	plan, err := worktree.BuildPlan(target, filepath.Join(repo.Root, ex.Compose), name, lo, hi)
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitExternal
	}

	p := NewPalette(e.Stdout)
	fmt.Fprintf(e.Stdout, "\n%sWORKTREE%s  %s\n", p.B, p.N, plan.Item)
	fmt.Fprintf(e.Stdout, "  %sproject%s  %s\n", p.D, p.N, plan.Project)
	for _, r := range plan.Ports {
		fmt.Fprintf(e.Stdout, "  %s✓%s %-10s %d → %s%d%s  %s(container %d)%s\n",
			p.G, p.N, r.Service, r.From, p.B, r.To, p.N, p.D, r.Container, p.N)
	}
	if len(plan.Ports) == 0 {
		fmt.Fprintf(e.Stdout, "  %sno published host ports — nothing to remap%s\n", p.D, p.N)
	}
	for svc, entries := range plan.Skipped {
		for _, en := range entries {
			// Loud on purpose: a stack reported as isolated while one port still
			// collides is worse than an honest refusal.
			fmt.Fprintf(e.Stdout, "  %s⚠%s %-10s could not parse %q — this port is NOT isolated\n",
				p.Y, p.N, svc, en)
		}
	}

	if *dry {
		fmt.Fprintf(e.Stdout, "\n%sdry-run: nothing written%s\n\n", p.D, p.N)
		return ExitOK
	}
	if err := worktree.Apply(plan); err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	ev := domain.NewEvent(domain.EvWorktreeUp)
	ev.Item, ev.Reason = plan.Item, plan.Project
	_ = events.Append(repo, ev)

	fmt.Fprintf(e.Stdout, "\n  %swrote %s and .circle/%s%s\n\n",
		p.D, worktree.OverrideFile, worktree.MapFile, p.N)
	return ExitOK
}

func runWorktreeRelease(e Env, args []string) int {
	f := fs("worktree release", e)
	item := f.String("item", "", "work item id")
	dir := f.String("dir", "", "worktree directory")
	hook := f.Bool("hook", false, "read a WorktreeRemove payload from stdin")
	keepVolumes := f.Bool("keep-volumes", false, "skip compose down -v")
	if err := parse(f, args); err != nil {
		return ExitError
	}

	target, name := *dir, *item
	if *hook {
		p := readWorktreePayload(e)
		if p.Path != "" {
			target = p.Path
		}
		if name == "" {
			name = p.Name
		}
	}
	if target == "" {
		target = e.Cwd
	}
	if name == "" {
		name = filepath.Base(target)
	}

	p := NewPalette(e.Stdout)
	repo, err := contract.Open(target)
	if err == nil && !*keepVolumes {
		compose := filepath.Join(repo.Root, repo.Contract.Execution.Compose)
		if _, statErr := os.Stat(compose); statErr == nil {
			// Down before the directory disappears. Volumes are namespaced by
			// project, so -v removes only this worktree's data — orphaned
			// volumes are what the Phase 3 exit criterion forbids.
			if err := worktree.Down(target, worktree.ProjectName(name), compose); err != nil {
				fmt.Fprintf(e.Stderr, "circle: %v\n", err)
			} else {
				fmt.Fprintf(e.Stdout, "  %s✓%s stack down, volumes removed\n", p.G, p.N)
			}
		}
	}

	lease, err := worktree.Release(target, name)
	if err != nil {
		// A release that cannot find the ledger must not block worktree removal.
		if *hook {
			return ExitOK
		}
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	if repo != nil {
		ev := domain.NewEvent(domain.EvWorktreeDown)
		ev.Item = name
		_ = events.Append(repo, ev)
	}
	fmt.Fprintf(e.Stdout, "  %s✓%s released %d port(s) held by %s\n", p.G, p.N, len(lease.Ports), name)
	return ExitOK
}

func runWorktreeList(e Env, args []string) int {
	leases, err := worktree.List(e.Cwd)
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitExternal
	}
	if len(leases) == 0 {
		fmt.Fprintln(e.Stdout, "no worktrees provisioned")
		return ExitOK
	}
	p := NewPalette(e.Stdout)
	for _, l := range leases {
		fmt.Fprintf(e.Stdout, "%s%-18s%s %s\n", p.B, l.Item, p.N, l.Project)
		for k, port := range l.Ports {
			fmt.Fprintf(e.Stdout, "  %s%-22s%s %d\n", p.D, k, p.N, port)
		}
	}
	return ExitOK
}

// runRun emits the commands Claude should execute. It never runs them: one
// execution model, and the contract records rather than orchestrates (D-11).
func runRun(e Env, args []string) int {
	f := fs("run", e)
	printOnly := f.Bool("print", true, "emit the commands rather than running them")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	ex := repo.Contract.Execution
	p := NewPalette(e.Stdout)

	if !*printOnly {
		// Refusing is the whole point. A second way to start the app is a second
		// thing that goes stale.
		fmt.Fprintln(e.Stderr, "circle: run only prints. Claude executes; the contract records (D-11).")
		return ExitError
	}

	fmt.Fprintf(e.Stdout, "\n%s# %s — from the execution contract%s\n\n", p.D, repo.Contract.Project.Name, p.N)
	if ex.Bootstrap != "" {
		fmt.Fprintf(e.Stdout, "%s\n", ex.Bootstrap)
	}
	if ex.Up != "" {
		fmt.Fprintf(e.Stdout, "%s\n", ex.Up)
	}

	mapPath := repo.StateDir(worktree.MapFile)
	if b, err := os.ReadFile(mapPath); err == nil {
		fmt.Fprintf(e.Stdout, "\n%s# live ports in this worktree%s\n", p.D, p.N)
		fmt.Fprintf(e.Stdout, "%s%s%s", p.D, string(b), p.N)
	} else if ex.URL != "" {
		fmt.Fprintf(e.Stdout, "\n%s# %s%s\n", p.D, ex.URL, p.N)
		if ex.PortsFixed {
			fmt.Fprintf(e.Stdout, "%s# ports are fixed in compose: two checkouts cannot run at once\n"+
				"# unless this worktree was provisioned%s\n", p.Y, p.N)
		}
	}
	if ex.Down != "" {
		fmt.Fprintf(e.Stdout, "\n%s# stop%s\n%s\n", p.D, p.N, ex.Down)
	}
	fmt.Fprintln(e.Stdout)
	return ExitOK
}
