package cli

import (
	"fmt"

	"github.com/edwardmontoya/circle/internal/knowledge"
)

func init() {
	register(Command{"knowledge verify", "Check registered definitions against the code", runKnowledgeVerify})
}

// runKnowledgeVerify answers the question preflight never asked: do the
// documents agree with the repository?
//
// Existence was already checked. This checks consistency — a definition that
// resolves but contradicts the code is worse than one that is missing, because
// the agent will read it, believe it, and plan against a system that is not
// there.
func runKnowledgeVerify(e Env, args []string) int {
	f := fs("knowledge verify", e)
	failOn := f.Bool("fail-on-drift", false, "exit 2 when a definition contradicts the code")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}

	findings, err := knowledge.Verify(repo)
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	p := NewPalette(e.Stdout)

	if len(repo.Contract.Knowledge.Definitions) == 0 {
		fmt.Fprintf(e.Stdout, "\n%sNo definitions registered.%s Nothing states what should be built,\n",
			p.Y, p.N)
		fmt.Fprintf(e.Stdout, "%sso there is nothing to contradict — and nothing to plan against.%s\n\n", p.D, p.N)
		return ExitOK
	}
	if len(findings) == 0 {
		fmt.Fprintf(e.Stdout, "\n%s✓%s every registered definition agrees with the repository\n\n", p.G, p.N)
		return ExitOK
	}

	fmt.Fprintf(e.Stdout, "\n%sDEFINITIONS CONTRADICTED BY THE CODE%s\n", p.B, p.N)
	fmt.Fprintf(e.Stdout, "%sThese documents are registered as authoritative. They are not.%s\n\n", p.D, p.N)

	for doc, fs := range knowledge.ByDocument(findings) {
		fmt.Fprintf(e.Stdout, "%s%s%s\n", p.B, doc, p.N)
		for _, fd := range fs {
			fmt.Fprintf(e.Stdout, "  %s✗%s %s%-15s%s %s\n",
				p.R, p.N, p.D, fd.Kind, p.N, fd.Claim)
			fmt.Fprintf(e.Stdout, "      %s%s%s\n", p.D, fd.Reality, p.N)
			fmt.Fprintf(e.Stdout, "      %s%s:%d  %s%s\n", p.D, doc, fd.Line, fd.Excerpt, p.N)
		}
		fmt.Fprintln(e.Stdout)
	}

	fmt.Fprintf(e.Stdout, "%s%d contradiction(s).%s A stale definition is worse than a missing one:\n",
		p.Y, len(findings), p.N)
	fmt.Fprintf(e.Stdout, "%san empty registry is honest, a wrong one is a lie with a checkmark.%s\n\n", p.D, p.N)
	fmt.Fprintf(e.Stdout, "%sFix the document, or drop it:  circle knowledge remove <path>%s\n\n", p.D, p.N)

	if *failOn {
		return ExitGate
	}
	return ExitOK
}
