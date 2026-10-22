package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/domain"
)

func init() {
	register(Command{"knowledge add", "Register a path under docs, definitions, validations or standards", runKnowledgeAdd})
	register(Command{"knowledge list", "The registry, and what each path resolves to", runKnowledgeList})
	register(Command{"knowledge remove", "Drop a registered path", runKnowledgeRemove})
}

var knowledgeClasses = []string{"docs", "definitions", "validations", "standards"}

func runKnowledgeAdd(e Env, args []string) int {
	f := fs("knowledge add", e)
	class := f.String("as", "", "docs | definitions | validations | standards")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	if f.NArg() == 0 || *class == "" {
		fmt.Fprintln(e.Stderr, "usage: circle knowledge add <path> --as docs|definitions|validations|standards")
		return ExitError
	}
	valid := false
	for _, c := range knowledgeClasses {
		if *class == c {
			valid = true
		}
	}
	if !valid {
		fmt.Fprintf(e.Stderr, "circle: unknown class %q — one of: %s\n",
			*class, strings.Join(knowledgeClasses, ", "))
		return ExitError
	}

	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	path := f.Arg(0)

	// Refuse a path that resolves to nothing. Registering it would only move the
	// failure from here to preflight, which is a worse place to discover it.
	if matches, _ := filepath.Glob(repo.Path(path)); len(matches) == 0 {
		if _, err := os.Stat(repo.Path(path)); err != nil {
			fmt.Fprintf(e.Stderr, "circle: %s matches nothing in this repository\n", path)
			fmt.Fprintln(e.Stderr, "A contract that asserts a file which does not exist is the thing preflight blocks on.")
			return ExitError
		}
	}

	c := repo.Contract
	target := &c.Knowledge.Docs
	switch *class {
	case "definitions":
		target = &c.Knowledge.Definitions
	case "validations":
		target = &c.Knowledge.Validations
	case "standards":
		target = &c.Knowledge.Standards
	}
	for _, existing := range *target {
		if existing == path {
			fmt.Fprintf(e.Stdout, "%s is already registered under %s\n", path, *class)
			return ExitOK
		}
	}
	*target = append(*target, path)

	if err := writeContract(repo, c); err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	matches, _ := filepath.Glob(repo.Path(path))
	p := NewPalette(e.Stdout)
	fmt.Fprintf(e.Stdout, "%s✓%s registered %s under %s%s%s → %d match(es)\n",
		p.G, p.N, path, p.B, *class, p.N, len(matches))
	return ExitOK
}

func runKnowledgeRemove(e Env, args []string) int {
	f := fs("knowledge remove", e)
	if err := parse(f, args); err != nil {
		return ExitError
	}
	if f.NArg() == 0 {
		fmt.Fprintln(e.Stderr, "usage: circle knowledge remove <path>")
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	path := f.Arg(0)
	c := repo.Contract
	removed := false
	for _, target := range []*[]string{
		&c.Knowledge.Docs, &c.Knowledge.Definitions, &c.Knowledge.Validations,
	} {
		var keep []string
		for _, p := range *target {
			if p == path {
				removed = true
				continue
			}
			keep = append(keep, p)
		}
		*target = keep
	}
	if !removed {
		fmt.Fprintf(e.Stderr, "circle: %s is not registered\n", path)
		return ExitError
	}
	if err := writeContract(repo, c); err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	fmt.Fprintf(e.Stdout, "removed %s\n", path)
	return ExitOK
}

func runKnowledgeList(e Env, args []string) int {
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	f := fs("knowledge list", e)
	unresolved := f.Bool("unresolved", false, "only paths that match nothing")
	var only multiFlag
	f.Var(&only, "as", "only this class; repeatable")
	content := f.Bool("content", false, "print each file's contents. For skill injection")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	if *content {
		return printKnowledge(e, repo, only)
	}
	p := NewPalette(e.Stdout)
	empty := true
	for _, cls := range repo.Contract.Knowledge.Classes() {
		if len(only) > 0 && !contains(only, cls.Name) {
			continue
		}
		for _, path := range cls.Paths {
			matches, _ := filepath.Glob(repo.Path(path))
			if len(matches) == 0 {
				if _, err := os.Stat(repo.Path(path)); err == nil {
					matches = []string{path}
				}
			}
			if *unresolved && len(matches) > 0 {
				continue
			}
			empty = false
			mark, colour := "✓", p.G
			if len(matches) == 0 {
				mark, colour = "✗", p.R
			}
			fmt.Fprintf(e.Stdout, "%s%s%s %-12s %-34s %d\n",
				colour, mark, p.N, cls.Name, path, len(matches))
		}
	}
	if empty {
		if *unresolved {
			fmt.Fprintf(e.Stdout, "%s✓%s every registered path resolves\n", p.G, p.N)
		} else {
			fmt.Fprintln(e.Stdout, "nothing registered")
		}
	}
	return ExitOK
}

// writeContract rewrites only the three knowledge arrays, in place.
//
// Deliberately NOT a re-encode of the whole struct. project.toml is written to
// be read and hand-edited — `circle init` puts explanatory comments in it, and
// users add their own — and round-tripping it through the encoder would delete
// every one of them, plus print an empty string for every unset optional field.
// A tool that silently strips your comments is a tool you stop trusting with
// your files.
func writeContract(r *contract.Repo, c domain.Contract) error {
	path := r.StateDir("project.toml")
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(b), "\n")

	want := map[string][]string{
		"docs":        c.Knowledge.Docs,
		"definitions": c.Knowledge.Definitions,
		"validations": c.Knowledge.Validations,
		"standards":   c.Knowledge.Standards,
	}
	seen := map[string]bool{}
	inKnowledge := false
	insertAt := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			if inKnowledge && insertAt < 0 {
				insertAt = i // the section ended; anything missing goes here
			}
			inKnowledge = trimmed == "[knowledge]"
			continue
		}
		if !inKnowledge {
			continue
		}
		for key, vals := range want {
			if seen[key] {
				continue
			}
			if strings.HasPrefix(trimmed, key+" ") || strings.HasPrefix(trimmed, key+"=") {
				// Preserve the author's alignment: keep whatever ran before the '='.
				prefix := line[:strings.Index(line, "=")+1]
				lines[i] = prefix + " " + tomlArray(vals)
				seen[key] = true
			}
		}
	}

	// A class that was absent from the file entirely still has to land somewhere.
	var missing []string
	for _, key := range knowledgeClasses {
		if !seen[key] && len(want[key]) > 0 {
			missing = append(missing, fmt.Sprintf("%-11s = %s", key, tomlArray(want[key])))
		}
	}
	if len(missing) > 0 {
		switch {
		case insertAt >= 0:
			// Before the blank lines that separate the section from the next,
			// so the file keeps its shape.
			for insertAt > 0 && strings.TrimSpace(lines[insertAt-1]) == "" {
				insertAt--
			}
			lines = append(lines[:insertAt], append(missing, lines[insertAt:]...)...)
		case inKnowledge:
			lines = append(lines, missing...)
		default:
			lines = append(lines, append([]string{"", "[knowledge]"}, missing...)...)
		}
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

func tomlArray(vals []string) string {
	if len(vals) == 0 {
		return "[]"
	}
	quoted := make([]string, len(vals))
	for i, v := range vals {
		quoted[i] = fmt.Sprintf("%q", v)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// printKnowledge prints the registered files themselves, so a skill can inject
// the standards a reviewer applies rather than a list of where they are.
func printKnowledge(e Env, repo *contract.Repo, only []string) int {
	printed := 0
	for _, cls := range repo.Contract.Knowledge.Classes() {
		if len(only) > 0 && !contains(only, cls.Name) {
			continue
		}
		for _, pat := range cls.Paths {
			for _, m := range expand(repo, pat) {
				b, err := os.ReadFile(m)
				if err != nil {
					continue
				}
				rel, _ := filepath.Rel(repo.Root, m)
				fmt.Fprintf(e.Stdout, "===== %s (%s) =====\n%s\n", rel, cls.Name, strings.TrimRight(string(b), "\n"))
				printed++
			}
		}
	}
	if printed == 0 {
		fmt.Fprintf(e.Stdout, "(nothing registered under %s", strings.Join(only, ", "))
		if contains(only, "standards") {
			fmt.Fprint(e.Stdout, " — review against the smell baseline and the DOMAIN.md files, and say so")
		}
		fmt.Fprintln(e.Stdout, ")")
	}
	return ExitOK
}

// expand resolves a registered path to files: a glob, a file, or every
// markdown file under a directory.
func expand(repo *contract.Repo, pat string) []string {
	matches, _ := filepath.Glob(repo.Path(pat))
	var out []string
	for _, m := range matches {
		st, err := os.Stat(m)
		if err != nil {
			continue
		}
		if !st.IsDir() {
			out = append(out, m)
			continue
		}
		_ = filepath.WalkDir(m, func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(p, ".md") {
				out = append(out, p)
			}
			return nil
		})
	}
	return out
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
