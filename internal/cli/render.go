package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/edwardmontoya/circle/internal/contract"
)

// Palette is resolved once per process. Honours NO_COLOR and a non-TTY stdout,
// because most invocations are from a hook and nobody wants escape codes in a
// deny reason.
type Palette struct{ G, R, Y, D, B, N string }

func NewPalette(w io.Writer) Palette {
	f, ok := w.(*os.File)
	isTTY := ok && func() bool {
		st, err := f.Stat()
		return err == nil && st.Mode()&os.ModeCharDevice != 0
	}()
	if !isTTY || os.Getenv("NO_COLOR") != "" {
		return Palette{}
	}
	return Palette{
		G: "\033[32m", R: "\033[31m", Y: "\033[33m",
		D: "\033[2m", B: "\033[1m", N: "\033[0m",
	}
}

func (p Palette) mark(s contract.Severity) string {
	switch s {
	case contract.OK:
		return p.G + "✓" + p.N
	case contract.Warn:
		return p.Y + "⚠" + p.N
	default:
		return p.R + "✗" + p.N
	}
}

// RenderExplain is the full human report — A-1 screen 02.
func RenderExplain(w io.Writer, root string, res contract.Result, p Palette) {
	fmt.Fprintf(w, "\n%sPREFLIGHT%s  %s\n\n", p.B, p.N, root)
	section := ""
	for _, c := range res.Checks {
		if c.Section != section {
			fmt.Fprintf(w, "%s%s%s\n", p.B, strings.ToUpper(c.Section), p.N)
			section = c.Section
		}
		fmt.Fprintf(w, "  %s %-13s %s\n", p.mark(c.Severity), c.Label, c.Detail)
		if c.Fix != "" {
			fmt.Fprintf(w, "     %s└ %s%s\n", p.D, c.Fix, p.N)
		}
	}
	fails, warns := res.Count(contract.Fail), res.Count(contract.Warn)
	fmt.Fprintln(w)
	if res.Incubating {
		// Distinct from a pass on purpose. Reporting "PASSED" on an empty
		// repository would claim a stranger could run and test it, which is the
		// one thing preflight exists to be honest about.
		fmt.Fprintf(w, "%sINCUBATING%s  %snothing to run and nothing to prove yet%s\n\n",
			p.Y, p.N, p.D, p.N)
		fmt.Fprintf(w, "%sThis is expected in a new project, and writes are not blocked.%s\n", p.D, p.N)
		fmt.Fprintf(w, "%sCome back when the project has:%s\n", p.D, p.N)
		fmt.Fprintf(w, "%s  · something that starts   → set execution.compose and up/down%s\n", p.D, p.N)
		fmt.Fprintf(w, "%s  · a first test            → set a gate under [quality]%s\n", p.D, p.N)
		fmt.Fprintf(w, "%sThen `circle init --force` re-detects both.%s\n\n", p.D, p.N)
		return
	}
	if fails == 0 {
		fmt.Fprintf(w, "%sPREFLIGHT PASSED%s  %s%d advisory%s\n\n", p.G, p.N, p.D, warns, p.N)
		return
	}
	fmt.Fprintf(w, "%sPREFLIGHT FAILED%s  %s%d blocking · %d advisory%s\n",
		p.R, p.N, p.D, fails, warns, p.N)
	fmt.Fprintf(w, "%sWrites are blocked until these pass.%s\n\n", p.D, p.N)
}

// RenderContext is what gets injected into a skill body.
//
// It must stay small: skill content is re-attached after compaction under a
// 5k-per-skill budget inside a 25k total, so a verbose block silently costs the
// session other skills.
func RenderContext(w io.Writer, name string, res contract.Result, p Palette) {
	fmt.Fprintf(w, "CONTRACTS  %s\n", name)
	for _, c := range res.Checks {
		// Collapse the per-path knowledge noise, keep the ratio line.
		if c.Severity == contract.OK && c.Section == "knowledge" && c.Label != "paths" {
			continue
		}
		fmt.Fprintf(w, "  %s %-9s %-12s %s\n", p.mark(c.Severity), c.Section, c.Label, c.Detail)
	}
	switch {
	case res.Incubating:
		fmt.Fprintf(w, "\nINCUBATING — nothing to run or prove yet; writes are not blocked\n")
	case res.Blocked():
		fmt.Fprintf(w, "\nPREFLIGHT FAILED — run circle preflight --explain\n")
	}
}

// readFileTrim reads a small state file, trimming trailing whitespace.
func readFileTrim(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}
