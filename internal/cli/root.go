package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// Env carries everything a command needs, so commands stay testable without
// touching the real process.
type Env struct {
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader
	Cwd    string
}

func OSEnv() Env {
	cwd, _ := os.Getwd()
	return Env{os.Stdout, os.Stderr, os.Stdin, cwd}
}

type Command struct {
	Name    string
	Summary string
	Run     func(e Env, args []string) int
}

// Deliberately no cobra. The binary ships inside a plugin's bin/ and runs on
// strangers' machines; a zero-dependency command tree is one less thing that can
// fail to resolve. TOML is the single external dependency.
var commands []Command

func register(c Command) { commands = append(commands, c) }

func Main() int {
	e := OSEnv()
	args := os.Args[1:]
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		usage(e.Stdout)
		return ExitOK
	}
	if args[0] == "--version" || args[0] == "-V" {
		fmt.Fprintln(e.Stdout, "circle 0.1.0-dev")
		return ExitOK
	}
	name := args[0]
	for _, c := range commands {
		if c.Name == name {
			return c.Run(e, args[1:])
		}
	}
	// Two-word commands: `circle task close`, `circle timeline show`.
	if len(args) >= 2 {
		joined := args[0] + " " + args[1]
		for _, c := range commands {
			if c.Name == joined {
				return c.Run(e, args[2:])
			}
		}
	}
	fmt.Fprintf(e.Stderr, "circle: unknown command %q\n\n", strings.Join(args, " "))
	usage(e.Stderr)
	return ExitError
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "circle — encode a project's execution, quality and knowledge contracts")
	fmt.Fprintln(w)
	var last string
	for _, c := range commands {
		group := strings.Fields(c.Name)[0]
		if group != last {
			fmt.Fprintln(w)
			last = group
		}
		fmt.Fprintf(w, "  %-22s %s\n", c.Name, c.Summary)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Exit codes: 0 ok · 2 gate failed · 3 precondition · 4 external · 5 quality · 7 drift")
}

// fs builds a flag set that prints to the command's own stderr.
func fs(name string, e Env) *flag.FlagSet {
	f := flag.NewFlagSet(name, flag.ContinueOnError)
	f.SetOutput(e.Stderr)
	return f
}

// parse accepts flags in any position.
//
// Go's flag package stops parsing at the first non-flag argument, so
// `knowledge add docs/ --as definitions` would silently ignore --as — and that
// is the order the tool's own hints and documentation use. Rather than bend the
// documented syntax to the parser, hoist the flags and parse the rest.
//
// A flag that takes a value consumes the token after it, which is why this
// inspects the FlagSet rather than guessing from the string alone.
func parse(f *flag.FlagSet, args []string) error {
	takesValue := map[string]bool{}
	f.VisitAll(func(fl *flag.Flag) {
		// A bool flag is the only kind that may appear without a value.
		if bf, ok := fl.Value.(interface{ IsBoolFlag() bool }); !ok || !bf.IsBoolFlag() {
			takesValue[fl.Name] = true
		}
	})

	var flags, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			positional = append(positional, a)
			continue
		}
		flags = append(flags, a)
		name := strings.TrimLeft(a, "-")
		if strings.Contains(name, "=") {
			continue // --flag=value carries its own value
		}
		if takesValue[name] && i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return f.Parse(append(flags, positional...))
}
