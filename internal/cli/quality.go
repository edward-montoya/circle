package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/domain"
	"github.com/edwardmontoya/circle/internal/events"
)

func init() {
	register(Command{"quality list", "List the declared gates", runQualityList})
	register(Command{"quality run", "Execute gates and record the results", runQualityRun})
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
	if err := f.Parse(args); err != nil {
		return ExitError
	}

	repo, err := contract.Open(e.Cwd)
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitPrecond
	}

	wanted := f.Args()
	gates := repo.Contract.Quality.Gates()
	if len(wanted) > 0 && !*all {
		var sel []domain.GateSpec
		for _, name := range wanted {
			g, ok := repo.Contract.Quality.Gate(name)
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
	failed := 0
	for _, g := range gates {
		start := time.Now()
		cmd := exec.Command("sh", "-c", g.Command)
		cmd.Dir = repo.Root
		cmd.Stdout, cmd.Stderr = e.Stdout, e.Stderr
		cmd.Env = os.Environ()
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
