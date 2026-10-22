package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/domain"
	"github.com/edwardmontoya/circle/internal/events"
	"github.com/edwardmontoya/circle/internal/item"
)

func init() {
	register(Command{"preflight", "Validate all three contracts. The gate.", runPreflight})
	register(Command{"project validate", "Validate the contract only", runPreflight})
}

func runPreflight(e Env, args []string) int {
	f := fs("preflight", e)
	explain := f.Bool("explain", false, "full report with fixes")
	ctx := f.Bool("context", false, "compact block for skill injection")
	advisory := f.Bool("advisory", false, "report a failure but exit 0: for skills that only plan and write documents")
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
	if res.Blocked() && !res.Incubating {
		ev.ExitCode = ExitGate
	}
	_ = events.Append(repo, ev)

	switch {
	case *quiet:
	case *ctx:
		RenderContext(e.Stdout, filepath.Base(repo.Root), res, NewPalette(e.Stdout))
		renderSessionContext(e, repo)
	case *explain:
		RenderExplain(e.Stdout, repo.Root, res, NewPalette(e.Stdout))
	default:
		RenderExplain(e.Stdout, repo.Root, res, NewPalette(e.Stdout))
	}

	if res.Blocked() && !res.Incubating {
		// Planning does not need a runnable stack: shaping, charting and retro
		// write documents, which the gate allows while preflight fails. Aborting
		// them would stop a team from planning the next item while the current
		// one is still building the stack the contract declares.
		if *advisory {
			fmt.Fprintln(e.Stdout, "\nPLANNING ONLY — documents and .circle/ are writable; code is not until preflight passes")
			return ExitOK
		}
		if *ctx {
			return refuseSkill(e, "the contracts are invalid — `circle preflight --explain` shows what to fix")
		}
		return ExitGate
	}
	return ExitOK
}

// renderSessionContext adds what a skill needs beyond the contracts: which item
// this session works on and where it stands, and which practice fills each
// lifecycle slot.
func renderSessionContext(e Env, repo *contract.Repo) {
	id, src := currentItem(repo, "")
	st := item.Derive(repo, id)
	fmt.Fprintf(e.Stdout, "\nITEM       %s — %s (%s; resolved from %s)\n", id, st.State, st.Reason, src)
	var slots []string
	for _, s := range repo.Contract.Lifecycle.Slots() {
		slots = append(slots, s[0]+"="+s[1])
	}
	fmt.Fprintf(e.Stdout, "LIFECYCLE  %s\n", strings.Join(slots, "  "))
	// The gate commands are listed above, and an agent asked to "run every
	// gate" ran them straight from that list: green, and unrecorded. A run
	// Circle did not see proves nothing to task close, review or status.
	if len(repo.Contract.Quality.Gates()) > 0 {
		fmt.Fprintln(e.Stdout, "GATES      run them with `circle quality run [gate]` — bootstrap first, each result recorded.")
		fmt.Fprintln(e.Stdout, "           A gate command run any other way leaves no record and counts for nothing.")
	}
}
