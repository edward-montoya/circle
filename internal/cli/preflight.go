package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/domain"
	"github.com/edwardmontoya/circle/internal/events"
)

func init() {
	register(Command{"preflight", "Validate all three contracts. The gate.", runPreflight})
	register(Command{"project validate", "Validate the contract only", runPreflight})
}

func runPreflight(e Env, args []string) int {
	f := fs("preflight", e)
	explain := f.Bool("explain", false, "full report with fixes")
	ctx := f.Bool("context", false, "compact block for skill injection")
	quiet := f.Bool("quiet", false, "exit code only")
	force := f.Bool("force", false, "bypass; requires --reason, recorded, caps the score")
	reason := f.String("reason", "", "why the bypass is justified")
	if err := parse(f, args); err != nil {
		return ExitError
	}

	repo, err := contract.Open(e.Cwd)
	if err != nil {
		if !*quiet {
			fmt.Fprintf(e.Stderr, "circle: %v — run `circle init`\n", err)
		}
		return ExitPrecond
	}

	if *force {
		// A bypass with no consequence is paperwork. This one is recorded, caps
		// the status score, and blocks PR creation until cleared.
		if strings.TrimSpace(*reason) == "" {
			fmt.Fprintln(e.Stderr, "circle: --force requires --reason")
			return ExitError
		}
		_ = os.MkdirAll(repo.RuntimeDir(), 0o755)
		_ = os.WriteFile(repo.RuntimeDir("forced"), []byte(*reason), 0o644)
		ev := domain.NewEvent(domain.EvPreflightForce)
		ev.Reason = *reason
		_ = events.Append(repo, ev)
		fmt.Fprintf(e.Stdout, "preflight bypassed and recorded: %s\n", *reason)
		fmt.Fprintln(e.Stdout, "status is capped at 60 and PR creation is blocked.")
		return ExitOK
	}

	res := contract.Validate(repo)

	ev := domain.NewEvent(domain.EvPreflight)
	if res.Blocked() {
		ev.ExitCode = ExitGate
	}
	_ = events.Append(repo, ev)

	switch {
	case *quiet:
	case *ctx:
		RenderContext(e.Stdout, filepath.Base(repo.Root), res, NewPalette(e.Stdout))
	case *explain:
		RenderExplain(e.Stdout, repo.Root, res, NewPalette(e.Stdout))
	default:
		RenderExplain(e.Stdout, repo.Root, res, NewPalette(e.Stdout))
	}

	if res.Blocked() {
		return ExitGate
	}
	return ExitOK
}
