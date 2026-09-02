package cli

import (
	"fmt"

	"github.com/edwardmontoya/circle/internal/serve"
)

func init() {
	register(Command{"serve", "Minimal read-only observation app over SSE", runServe})
}

func runServe(e Env, args []string) int {
	f := fs("serve", e)
	addr := f.String("addr", "localhost:7777", "listen address")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	p := NewPalette(e.Stdout)
	fmt.Fprintf(e.Stdout, "\n  %sCircle%s  %s\n", p.B, p.N, repo.Contract.Project.Name)
	fmt.Fprintf(e.Stdout, "  %sobserving %s%s\n", p.D, repo.Root, p.N)
	fmt.Fprintf(e.Stdout, "\n  http://%s\n\n", *addr)
	fmt.Fprintf(e.Stdout, "  %sRead-only. It suggests; it never dispatches.%s\n\n", p.D, p.N)
	if err := serve.Serve(repo, *addr); err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	return ExitOK
}
