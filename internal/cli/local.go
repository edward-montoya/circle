package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/shared"
)

func init() {
	register(Command{"local set", "Set a machine-local variable for gates and the bootstrap. Never committed", runLocalSet})
	register(Command{"local list", "The machine-local variables gates run with", runLocalList})
	register(Command{"local export", "Print them as export lines: eval \"$(circle local export)\"", runLocalExport})
}

// Machine-local environment is for what is true of this machine and no other:
// a proxy CA directory, a registry mirror, a credentials path. Committing it
// would break every other checkout; leaving it out means every gate run needs
// it typed again. It lives in the shared git directory, so every worktree of
// the repository gets it, and .circle/local.env can override it per checkout.
func localFiles(repo *contract.Repo) []string {
	return []string{
		filepath.Join(shared.Dir(repo.Root), "local.env"),
		repo.StateDir("local.env"),
	}
}

func readEnvFile(path string, into map[string]string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		if k, v, ok := strings.Cut(line, "="); ok {
			into[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
}

// localEnv is the machine-local variables, later files winning.
func localEnv(repo *contract.Repo) map[string]string {
	m := map[string]string{}
	for _, p := range localFiles(repo) {
		readEnvFile(p, m)
	}
	return m
}

// gateEnv is the process environment plus the machine-local variables. The
// process wins: a variable set on the command line is the more deliberate one.
func gateEnv(repo *contract.Repo) []string {
	env := os.Environ()
	have := map[string]bool{}
	for _, kv := range env {
		k, _, _ := strings.Cut(kv, "=")
		have[k] = true
	}
	local := localEnv(repo)
	keys := make([]string, 0, len(local))
	for k := range local {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !have[k] {
			env = append(env, k+"="+local[k])
		}
	}
	return env
}

func runLocalSet(e Env, args []string) int {
	f := fs("local set", e)
	if err := parse(f, args); err != nil {
		return ExitError
	}
	if f.NArg() != 2 {
		fmt.Fprintln(e.Stderr, "usage: circle local set <KEY> <value>   (an empty value removes it)")
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	path := localFiles(repo)[0]
	m := map[string]string{}
	readEnvFile(path, m)
	if f.Arg(1) == "" {
		delete(m, f.Arg(0))
	} else {
		m[f.Arg(0)] = f.Arg(1)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("# Machine-local variables for circle gates. Not committed.\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "%s=%s\n", k, m[k])
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	fmt.Fprintf(e.Stdout, "set %s for every gate run in every worktree of this repository\n", f.Arg(0))
	return ExitOK
}

func runLocalList(e Env, args []string) int {
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	m := localEnv(repo)
	if len(m) == 0 {
		fmt.Fprintln(e.Stdout, "no machine-local variables — `circle local set <KEY> <value>`")
		return ExitOK
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(e.Stdout, "%s=%s\n", k, m[k])
	}
	return ExitOK
}

func runLocalExport(e Env, args []string) int {
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	m := localEnv(repo)
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(e.Stdout, "export %s=%q\n", k, m[k])
	}
	return ExitOK
}
