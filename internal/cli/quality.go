package cli

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"os/exec"
	"strings"
	"time"

	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/domain"
	"github.com/edwardmontoya/circle/internal/events"
	"github.com/edwardmontoya/circle/internal/task"
)

func init() {
	register(Command{"quality list", "List the declared gates", runQualityList})
	register(Command{"quality run", "Execute gates and record the results", runQualityRun})
	register(Command{"quality add", "Declare a gate in the quality contract", runQualityAdd})
}

func runQualityList(e Env, args []string) int {
	repo, err := contract.Open(e.Cwd)
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitPrecond
	}
	gates := repo.Contract.Quality.Gates()
	if len(gates) == 0 {
		fmt.Fprintln(e.Stdout, "no gates declared")
		return ExitOK
	}
	if b := repo.Contract.Quality.Bootstrap; b != "" {
		fmt.Fprintf(e.Stdout, "%-12s %s\n", "bootstrap", b)
	}
	for _, g := range gates {
		fmt.Fprintf(e.Stdout, "%-12s %s\n", g.Name, g.Command)
	}
	return ExitOK
}

func runQualityRun(e Env, args []string) int {
	f := fs("quality run", e)
	all := f.Bool("all", false, "run every gate")
	noRecord := f.Bool("no-record", false, "do not emit result events")
	failFast := f.Bool("fail-fast", false, "stop at the first failure")
	taskID := f.String("task", "", "attribute the run to a task, for its red → green history")
	noBootstrap := f.Bool("no-bootstrap", false, "skip quality.bootstrap before the gates")
	expect := f.String("expect", "", "red or green: exit 0 only when every gate matches. For test-first work")
	if err := parse(f, args); err != nil {
		return ExitError
	}

	if *expect != "" && *expect != "red" && *expect != "green" {
		fmt.Fprintln(e.Stderr, "circle: --expect is red or green")
		return ExitError
	}
	repo, err := contract.Open(e.Cwd)
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitPrecond
	}

	wanted := f.Args()
	gates := repo.Contract.Quality.Gates()
	// A task with a verification command narrower than its gate runs that
	// command, recorded under the gate and attributed to the task.
	if *taskID != "" {
		if t, err := task.Get(repo, *taskID); err == nil && t.Verify.Command != "" {
			if len(wanted) == 0 || (len(wanted) == 1 && (wanted[0] == t.Verify.Gate || "test:"+wanted[0] == t.Verify.Gate)) {
				gates, wanted = []domain.GateSpec{{Name: t.Verify.Gate, Command: t.Verify.Command}}, nil
			}
		}
	}
	if len(wanted) > 0 && !*all {
		var sel []domain.GateSpec
		for _, name := range wanted {
			g, ok := repo.Contract.Quality.Gate(name)
			if !ok {
				// "unit" for "test:unit": the key in [quality.test] is the name
				// people type.
				g, ok = repo.Contract.Quality.Gate("test:" + name)
			}
			if !ok {
				fmt.Fprintf(e.Stderr, "circle: no gate %q. declared: %s\n",
					name, repo.Contract.Quality.GateNames())
				return ExitError
			}
			sel = append(sel, g)
		}
		gates = sel
	}
	if len(gates) == 0 {
		fmt.Fprintln(e.Stderr, "circle: no gates declared")
		return ExitError
	}

	p := NewPalette(e.Stdout)

	// Bootstrap first. A gate that runs inside a container proves the image it
	// runs in, and an image built before the last edit proves old code: the
	// first gate an agent ran after writing new code came back red for exactly
	// that reason. The bootstrap is what makes the gates runnable on the
	// current tree, so it runs before them unless told otherwise.
	if b := strings.TrimSpace(repo.Contract.Quality.Bootstrap); b != "" && !*noBootstrap {
		cmd := exec.Command("sh", "-c", b)
		cmd.Dir = repo.Root
		// Quiet unless it fails: a build log streamed before every gate buries
		// the one thing the run is for, which gate passed and which did not.
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		cmd.Env = gateEnv(repo)
		if err := cmd.Run(); err != nil {
			lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
			if len(lines) > 40 {
				lines = lines[len(lines)-40:]
			}
			fmt.Fprintln(e.Stderr, strings.Join(lines, "\n"))
			fmt.Fprintf(e.Stdout, "%s✗%s bootstrap failed: %s\n", p.R, p.N, b)
			fmt.Fprintln(e.Stdout, "  the gates cannot prove the current tree until it succeeds")
			return ExitQuality
		}
		fmt.Fprintf(e.Stdout, "%s·%s bootstrap   %s\n", p.D, p.N, b)
	}

	if local := localEnv(repo); len(local) > 0 {
		keys := make([]string, 0, len(local))
		for k := range local {
			if os.Getenv(k) == "" {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		if len(keys) > 0 {
			fmt.Fprintf(e.Stdout, "%s·%s local       %s %s(circle local list)%s\n", p.D, p.N, strings.Join(keys, " "), p.D, p.N)
		}
	}

	failed, ran := 0, 0
	for _, g := range gates {
		ran++
		start := time.Now()
		cmd := exec.Command("sh", "-c", g.Command)
		cmd.Dir = repo.Root
		cmd.Stdout, cmd.Stderr = e.Stdout, e.Stderr
		cmd.Env = gateEnv(repo)
		runErr := cmd.Run()
		dur := time.Since(start)

		code := 0
		if runErr != nil {
			code = 1
			if ee, ok := runErr.(*exec.ExitError); ok {
				code = ee.ExitCode()
			}
			failed++
		}

		if !*noRecord {
			// Results are events. This is the entire reason the status score is
			// a measurement rather than an assertion.
			ev := domain.NewEvent(domain.EvGateRun)
			ev.Gate, ev.Command = g.Name, g.Command
			ev.ExitCode, ev.DurMS = code, dur.Milliseconds()
			ev.Task = *taskID
			if *expect != "" {
				ev.Reason = "expect=" + *expect
			}
			_ = events.Append(repo, ev)
		}

		mark, colour := "✓", p.G
		if code != 0 {
			mark, colour = "✗", p.R
		}
		fmt.Fprintf(e.Stdout, "%s%s%s %-12s exit=%d  %s%s%s\n",
			colour, mark, p.N, g.Name, code, p.D, dur.Round(time.Millisecond), p.N)

		if code != 0 && *failFast {
			break
		}
	}

	// Test-first: a new test must fail before the code exists. A red that came
	// out green means the test is not testing the new behaviour.
	if *expect == "red" {
		if failed == ran {
			fmt.Fprintf(e.Stdout, "\n%sred, as expected — now write the code that turns it green%s\n", p.Y, p.N)
			return ExitOK
		}
		fmt.Fprintf(e.Stdout, "\n%sexpected red, got green — the test does not exercise the new behaviour yet%s\n", p.R, p.N)
		return ExitQuality
	}
	if failed > 0 {
		fmt.Fprintf(e.Stdout, "\n%s%d of %d gates failed%s\n", p.R, failed, len(gates), p.N)
		return ExitQuality
	}
	fmt.Fprintf(e.Stdout, "\n%s%d/%d green%s\n", p.G, len(gates), len(gates), p.N)
	return ExitOK
}

// bootstrapHint is used by several commands when a gate binary is missing.
func bootstrapHint(q domain.Quality) string {
	if q.Bootstrap == "" {
		return "declare quality.bootstrap"
	}
	return "run: " + strings.TrimSpace(q.Bootstrap)
}

// runQualityAdd declares a gate. It is how a retro turns a mistake a tool could
// have caught into a check that catches it next time.
//
// The name decides where it goes: format, lint and typecheck are their own
// keys; test:<kind> goes under [quality.test]; anything else under
// [quality.extra].
func runQualityAdd(e Env, args []string) int {
	f := fs("quality add", e)
	if err := parse(f, args); err != nil {
		return ExitError
	}
	if f.NArg() != 2 {
		fmt.Fprintln(e.Stderr, `usage: circle quality add <name> "<command>"   e.g. test:integration, lint, check-imports`)
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	name, command := f.Arg(0), f.Arg(1)
	section, key := "quality", name
	switch {
	case name == "format" || name == "lint" || name == "typecheck":
	case strings.HasPrefix(name, "test:"):
		section, key = "quality.test", strings.TrimPrefix(name, "test:")
	default:
		section = "quality.extra"
	}
	if key == "" || strings.TrimSpace(command) == "" {
		fmt.Fprintln(e.Stderr, "circle: a gate needs a name and a command")
		return ExitError
	}
	if err := setTOMLKey(repo.StateDir("project.toml"), section, key, fmt.Sprintf("%q", command)); err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	if _, err := contract.Load(repo.Root); err != nil {
		fmt.Fprintf(e.Stderr, "circle: the contract no longer parses after the edit: %v\n", err)
		return ExitError
	}
	fmt.Fprintf(e.Stdout, "declared gate %s → %s  (under [%s])\n", name, command, section)
	fmt.Fprintf(e.Stdout, "  run it: circle quality run %s\n", name)
	return ExitOK
}
