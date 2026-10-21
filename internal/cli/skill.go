package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/domain"
	"github.com/edwardmontoya/circle/internal/item"
	"github.com/edwardmontoya/circle/internal/skill"
)

func init() {
	register(Command{"skill validate", "Check skills against the Circle skill contract", runSkillValidate})
	register(Command{"skill new", "Scaffold a skill that conforms to the contract", runSkillNew})
}

// skillOptions is the contract a skill is checked against: the commands this
// binary really has, the lifecycle states, the practice slots.
func skillOptions() skill.Options {
	o := skill.Options{Commands: map[string]bool{}, States: map[string]bool{}, Slots: map[string]bool{}}
	for _, c := range commands {
		o.Commands[c.Name] = true
	}
	for _, s := range item.States {
		o.States[string(s)] = true
	}
	for _, s := range (domain.Lifecycle{}).Slots() {
		o.Slots[s[0]] = true
	}
	return o
}

// defaultSkillRoots are where a repository keeps skills: its own .claude/skills,
// and a plugin's skills directory when the repository is a plugin.
func defaultSkillRoots(root string) []string {
	var out []string
	for _, d := range []string{".claude/skills", "plugin/skills", "skills"} {
		if st, err := os.Stat(filepath.Join(root, d)); err == nil && st.IsDir() {
			out = append(out, filepath.Join(root, d))
		}
	}
	return out
}

func runSkillValidate(e Env, args []string) int {
	f := fs("skill validate", e)
	if err := parse(f, args); err != nil {
		return ExitError
	}
	roots := f.Args()
	if len(roots) == 0 {
		root := e.Cwd
		if r, err := contract.FindRoot(e.Cwd); err == nil {
			root = r
		}
		roots = defaultSkillRoots(root)
	}
	paths := skill.Find(roots...)
	if len(paths) == 0 {
		fmt.Fprintln(e.Stderr, "circle: no SKILL.md found — pass a skills directory or a SKILL.md")
		return ExitPrecond
	}
	o := skillOptions()
	p := NewPalette(e.Stdout)
	failed := 0
	for _, path := range paths {
		s, err := skill.Parse(path)
		if err != nil {
			fmt.Fprintf(e.Stdout, "%s✗%s %v\n", p.R, p.N, err)
			failed++
			continue
		}
		problems := skill.Validate(s, o)
		hard := 0
		for _, pr := range problems {
			if !pr.Warn {
				hard++
			}
		}
		if hard > 0 {
			failed++
		}
		mark, colour := "✓", p.G
		if hard > 0 {
			mark, colour = "✗", p.R
		}
		fmt.Fprintf(e.Stdout, "%s%s%s %-14s %s%s%s\n", colour, mark, p.N, s.Name, p.D, s.Meta.Kind, p.N)
		for _, pr := range problems {
			m := p.R + "✗" + p.N
			if pr.Warn {
				m = p.Y + "⚠" + p.N
			}
			rel := pr.Path
			if r, err := filepath.Rel(e.Cwd, pr.Path); err == nil && !strings.HasPrefix(r, "..") {
				rel = r
			}
			fmt.Fprintf(e.Stdout, "    %s %s:%d %s\n", m, rel, pr.Line, pr.Reason)
		}
	}
	if failed > 0 {
		fmt.Fprintf(e.Stdout, "\n%s%d of %d skill(s) break the contract%s\n", p.R, failed, len(paths), p.N)
		return ExitGate
	}
	fmt.Fprintf(e.Stdout, "\n%s%d skill(s) conform%s\n", p.G, len(paths), p.N)
	return ExitOK
}

// runSkillNew scaffolds a skill that already passes validation, so a team's
// extension starts inside the contract instead of being fitted to it later.
func runSkillNew(e Env, args []string) int {
	f := fs("skill new", e)
	kind := f.String("kind", "orchestrator", "orchestrator or practice")
	requires := f.String("requires", "", "orchestrator: the item state it needs, e.g. specified")
	produces := f.String("produces", "", "orchestrator: the state it leaves the item in")
	desc := f.String("description", "", "REQUIRED: what it does, then when to use it")
	dir := f.String("dir", ".claude/skills", "where to write it, relative to the repository root")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	if f.NArg() == 0 || strings.TrimSpace(*desc) == "" {
		fmt.Fprintln(e.Stderr, `usage: circle skill new <name> --description "..." [--kind orchestrator|practice] [--requires <state>] [--produces <state>]`)
		return ExitError
	}
	name := f.Arg(0)
	root := e.Cwd
	if r, err := contract.FindRoot(e.Cwd); err == nil {
		root = r
	}
	path := filepath.Join(root, *dir, name, "SKILL.md")
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(e.Stderr, "circle: %s already exists\n", path)
		return ExitPrecond
	}
	o := skillOptions()
	for _, st := range []string{*requires, *produces} {
		if st != "" && !o.States[st] {
			fmt.Fprintf(e.Stderr, "circle: %q is not a lifecycle state — one of %s\n", st, stateNames())
			return ExitError
		}
	}
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "description: %s\n", *desc)
	if *kind == "orchestrator" {
		b.WriteString("disable-model-invocation: true\n")
	}
	b.WriteString("allowed-tools: Read Grep Glob Bash(circle *)\nmetadata:\n  circle:\n")
	fmt.Fprintf(&b, "    kind: %s\n", *kind)
	if *requires != "" {
		fmt.Fprintf(&b, "    requires: %s\n", *requires)
	}
	if *produces != "" {
		fmt.Fprintf(&b, "    produces: %s\n", *produces)
	}
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "# %s\n\n", name)
	if *requires != "" {
		fmt.Fprintf(&b, "```!\ncircle item require --state %s\n```\n\n", *requires)
	}
	b.WriteString("```!\ncircle preflight --context\n```\n\n")
	b.WriteString("## Steps\n\n1. <what the agent does first> — done when <a condition it can check>.\n\n")
	if *produces != "" {
		fmt.Fprintf(&b, "## Done\n\n`circle item show` reports `%s`. Then run `circle next`.\n", *produces)
	} else {
		b.WriteString("## Done\n\n<the checkable condition>. Then run `circle next`.\n")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	rel, _ := filepath.Rel(root, path)
	fmt.Fprintf(e.Stdout, "wrote %s — fill the steps, then `circle skill validate %s`\n", rel, filepath.Dir(rel))
	return ExitOK
}
