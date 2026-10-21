// Package skill checks a skill against the Circle skill contract.
//
// This is what makes Circle extensible rather than a fixed toolset: a team's own
// skill declares the item state it needs and the state it produces, opens with
// the gate injection, and calls only commands that exist — and then it gets the
// same enforcement as a built-in one. The contract is checked, not described.
package skill

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Meta is what a SKILL.md declares.
type Meta struct {
	Description    string
	UserOnly       bool // disable-model-invocation: true
	Kind           string
	Requires       []string
	Produces       string
	Calls          []string
	HasCircleBlock bool
}

// Skill is a parsed SKILL.md.
type Skill struct {
	Path     string
	Name     string
	Meta     Meta
	Body     string
	bodyLine int // line number where the body starts
}

// Problem is one contract violation.
type Problem struct {
	Path   string
	Line   int
	Reason string
	Warn   bool // advisory: reported, does not fail validation
}

func (p Problem) String() string {
	return fmt.Sprintf("%s:%d %s", p.Path, p.Line, p.Reason)
}

// Parse reads a SKILL.md. The frontmatter is the small YAML subset skills use:
// top-level scalars, and a nested metadata.circle block. No YAML dependency —
// the binary ships to strangers' machines with one dependency, and this subset
// does not need one.
func Parse(path string) (Skill, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Skill{}, err
	}
	s := Skill{Path: path, Name: filepath.Base(filepath.Dir(path))}
	lines := strings.Split(string(b), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return s, fmt.Errorf("%s: no frontmatter", path)
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return s, fmt.Errorf("%s: frontmatter is not closed", path)
	}
	var stack []string // nesting path by indentation
	var indents []int
	for _, raw := range lines[1:end] {
		if strings.TrimSpace(raw) == "" || strings.HasPrefix(strings.TrimSpace(raw), "#") {
			continue
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " "))
		for len(indents) > 0 && indent <= indents[len(indents)-1] {
			indents, stack = indents[:len(indents)-1], stack[:len(stack)-1]
		}
		kv := strings.SplitN(strings.TrimSpace(raw), ":", 2)
		key := strings.TrimSpace(kv[0])
		val := ""
		if len(kv) == 2 {
			val = strings.Trim(strings.TrimSpace(kv[1]), `"'`)
		}
		full := strings.Join(append(append([]string{}, stack...), key), ".")
		if val == "" {
			stack, indents = append(stack, key), append(indents, indent)
			continue
		}
		switch full {
		case "description":
			s.Meta.Description = val
		case "disable-model-invocation":
			s.Meta.UserOnly = val == "true"
		case "metadata.circle.kind":
			s.Meta.Kind = val
		case "metadata.circle.requires":
			for _, r := range strings.Split(val, "|") {
				if r = strings.TrimSpace(r); r != "" {
					s.Meta.Requires = append(s.Meta.Requires, r)
				}
			}
		case "metadata.circle.produces":
			s.Meta.Produces = val
		case "metadata.circle.calls":
			for _, c := range strings.Split(strings.Trim(val, "[]"), ",") {
				if c = strings.TrimSpace(c); c != "" {
					s.Meta.Calls = append(s.Meta.Calls, c)
				}
			}
		}
		if strings.HasPrefix(full, "metadata.circle.") {
			s.Meta.HasCircleBlock = true
		}
	}
	s.Body = strings.Join(lines[end+1:], "\n")
	s.bodyLine = end + 2
	return s, nil
}

var (
	linkRe   = regexp.MustCompile(`\]\(([A-Za-z0-9_./-]+\.md)\)`)
	statesRe = regexp.MustCompile(`--state\s+([a-z]+)`)
)

// Block is one fenced code block in a skill body.
type Block struct {
	Injected bool // ```! — runs at invocation, before the model sees the skill
	Line     int
	Lines    []string
}

// Blocks lists the fenced code blocks in the body.
func (s Skill) Blocks() []Block {
	var out []Block
	var cur *Block
	sc := bufio.NewScanner(strings.NewReader(s.Body))
	for n := s.bodyLine; sc.Scan(); n++ {
		t := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(t, "```") {
			if cur == nil {
				cur = &Block{Injected: t == "```!", Line: n}
			} else {
				out = append(out, *cur)
				cur = nil
			}
			continue
		}
		if cur != nil {
			cur.Lines = append(cur.Lines, sc.Text())
		}
	}
	return out
}

// Options carries what validation checks against.
type Options struct {
	// Commands is every registered `circle` command name ("task claim").
	Commands map[string]bool
	// States is every item lifecycle state.
	States map[string]bool
	// Slots is every lifecycle slot name.
	Slots map[string]bool
}

// budget is the compaction budget a skill body should stay under: Claude Code
// re-attaches skills after compaction within roughly 5k tokens each.
const budget = 20000

// Validate checks one skill against the contract.
func Validate(s Skill, o Options) []Problem {
	var ps []Problem
	add := func(line int, warn bool, format string, a ...any) {
		ps = append(ps, Problem{Path: s.Path, Line: line, Reason: fmt.Sprintf(format, a...), Warn: warn})
	}
	if strings.TrimSpace(s.Meta.Description) == "" {
		add(1, false, "description is empty — it is the only thing that decides when the skill fires")
	}
	if !s.Meta.HasCircleBlock {
		add(1, false, "no metadata.circle block — declare kind, and for an orchestrator requires/produces")
	}
	switch s.Meta.Kind {
	case "orchestrator":
		if !s.Meta.UserOnly {
			add(1, false, "an orchestrator moves item state, so only a human may invoke it: set disable-model-invocation: true")
		}
	case "practice":
		if s.Meta.UserOnly {
			add(1, false, "a practice is reached for by the model: remove disable-model-invocation")
		}
		if len(s.Meta.Requires) > 0 || s.Meta.Produces != "" {
			add(1, false, "a practice never changes item state: drop requires/produces")
		}
	case "":
	default:
		add(1, false, "metadata.circle.kind %q: orchestrator or practice", s.Meta.Kind)
	}

	var gated []string
	for _, r := range s.Meta.Requires {
		if r == "none" {
			continue
		}
		if !o.States[r] {
			add(1, false, "requires %q is not a lifecycle state", r)
		}
		gated = append(gated, r)
	}
	if s.Meta.Produces != "" && !o.States[s.Meta.Produces] {
		add(1, false, "produces %q is not a lifecycle state", s.Meta.Produces)
	}
	for _, c := range s.Meta.Calls {
		if strings.HasPrefix(c, "lifecycle.") && !o.Slots[strings.TrimPrefix(c, "lifecycle.")] {
			add(1, false, "calls %q: no such lifecycle slot", c)
		}
	}

	blocks := s.Blocks()
	// The gate injection must come first and cover exactly the declared states:
	// an injected command exiting 2 aborts the skill before the model sees it.
	if len(gated) > 0 && !contains(s.Meta.Requires, "none") {
		var first *Block
		for i := range blocks {
			if blocks[i].Injected {
				first = &blocks[i]
				break
			}
		}
		if first == nil || len(first.Lines) == 0 || !strings.HasPrefix(strings.TrimSpace(first.Lines[0]), "circle item require") {
			add(s.bodyLine, false, "requires %s but the first injected block is not `circle item require`", strings.Join(gated, "|"))
		} else {
			var got []string
			for _, m := range statesRe.FindAllStringSubmatch(first.Lines[0], -1) {
				got = append(got, m[1])
			}
			for _, g := range gated {
				if !contains(got, g) {
					add(first.Line, false, "the gate injection does not accept %q, which requires declares", g)
				}
			}
		}
	}

	// Every circle command a skill names must exist. A skill that calls a
	// command the binary does not have aborts at invocation, or worse, teaches
	// the model a command that will fail.
	for _, b := range blocks {
		for i, line := range b.Lines {
			for _, cmd := range circleCommands(line) {
				// "circle run foo" is the one-word command run with an argument.
				if first := strings.Fields(cmd)[0]; !o.Commands[cmd] && !o.Commands[first] {
					add(b.Line+1+i, false, "`circle %s` is not a circle command", cmd)
				}
			}
		}
	}

	for _, m := range linkRe.FindAllStringSubmatch(s.Body, -1) {
		if strings.Contains(m[1], "://") || strings.HasPrefix(m[1], "../") {
			continue
		}
		if _, err := os.Stat(filepath.Join(filepath.Dir(s.Path), m[1])); err != nil {
			add(s.bodyLine, false, "links %s, which does not exist beside the skill", m[1])
		}
	}
	if len(s.Body) > budget {
		add(s.bodyLine, true, "body is %d bytes; past ~%d it risks being dropped after compaction — move reference into a sibling file", len(s.Body), budget)
	}
	return ps
}

var cmdRe = regexp.MustCompile(`(?:^|[\s(;&|$` + "`" + `])circle\s+([a-z][a-z-]*)(?:\s+([a-z][a-z-]*))?`)

// circleCommands finds the circle command names on a line: "task claim" for
// `circle task claim x`, "next" for `circle next`.
func circleCommands(line string) []string {
	var out []string
	for _, m := range cmdRe.FindAllStringSubmatch(line, -1) {
		if m[2] != "" {
			out = append(out, m[1]+" "+m[2])
		} else {
			out = append(out, m[1])
		}
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

// Find lists the SKILL.md files under the given roots.
func Find(roots ...string) []string {
	var out []string
	for _, r := range roots {
		if st, err := os.Stat(r); err == nil && !st.IsDir() {
			out = append(out, r)
			continue
		}
		matches, _ := filepath.Glob(filepath.Join(r, "*", "SKILL.md"))
		out = append(out, matches...)
	}
	return out
}
