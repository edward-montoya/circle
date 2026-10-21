package item

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// SpecSections are the sections spec.md must fill. Out of scope and risks are
// required on purpose: an empty one is a decision nobody made, not a section
// with nothing to say.
var SpecSections = []string{"Problem", "Solution", "Behaviours", "Seams", "Decisions", "Out of scope", "Risks"}

// Seam is one row of the spec's seam table.
type Seam struct {
	Name       string
	Behaviours []int
	Gate       string
	Line       int
}

// SpecProblem is one reason a spec cannot advance.
type SpecProblem struct {
	Line   int
	Reason string
}

var (
	commentRe   = regexp.MustCompile(`(?s)<!--.*?-->`)
	behaviourRe = regexp.MustCompile(`^(\d+)\.\s+\S`)
	numberRe    = regexp.MustCompile(`\d+`)
)

// CheckSpec validates a spec against the declared gates.
//
// The rule Matt's to-spec does not have: every seam names a gate the quality
// contract declares, or says manual. A seam no gate can run becomes a task that
// could never close honestly, and the place to catch it is here, before any task
// exists. Every behaviour must be proved by at least one seam.
func CheckSpec(text string, gates map[string]bool) ([]Seam, []SpecProblem) {
	sections := map[string][]string{}
	sectionLine := map[string]int{}
	current := ""
	clean := commentRe.ReplaceAllStringFunc(text, func(m string) string {
		return strings.Repeat("\n", strings.Count(m, "\n")) // keep line numbers
	})
	for i, line := range strings.Split(clean, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "## ") {
			current = strings.TrimSpace(t[3:])
			sectionLine[current] = i + 1
			continue
		}
		if current != "" && t != "" && !placeholder(t) {
			sections[current] = append(sections[current], fmt.Sprintf("%d\x00%s", i+1, t))
		}
	}

	var problems []SpecProblem
	for _, name := range SpecSections {
		if len(sections[name]) == 0 {
			problems = append(problems, SpecProblem{sectionLine[name], fmt.Sprintf("section %q is missing or empty", name)})
		}
	}

	behaviours := map[int]int{}
	for _, raw := range sections["Behaviours"] {
		n, t := split(raw)
		if m := behaviourRe.FindStringSubmatch(t); m != nil {
			id, _ := strconv.Atoi(m[1])
			behaviours[id] = n
		}
	}

	var seams []Seam
	for _, raw := range sections["Seams"] {
		n, t := split(raw)
		if !strings.HasPrefix(t, "|") || strings.Contains(t, "---") {
			continue
		}
		cells := strings.Split(strings.Trim(t, "|"), "|")
		for i := range cells {
			cells[i] = strings.Trim(strings.TrimSpace(cells[i]), "`")
		}
		if len(cells) < 3 || strings.EqualFold(cells[0], "seam") {
			continue
		}
		s := Seam{Name: cells[0], Gate: cells[2], Line: n}
		for _, d := range numberRe.FindAllString(cells[1], -1) {
			id, _ := strconv.Atoi(d)
			s.Behaviours = append(s.Behaviours, id)
		}
		seams = append(seams, s)
		gate := strings.ToLower(s.Gate)
		switch {
		case s.Gate == "":
			problems = append(problems, SpecProblem{n, fmt.Sprintf("seam %q names no gate", s.Name)})
		case strings.HasPrefix(gate, "manual"):
		case !gates[s.Gate]:
			problems = append(problems, SpecProblem{n, fmt.Sprintf(
				"seam %q names gate %q, which the quality contract does not declare", s.Name, s.Gate)})
		}
		if len(s.Behaviours) == 0 {
			problems = append(problems, SpecProblem{n, fmt.Sprintf("seam %q proves no numbered behaviour", s.Name)})
		}
	}
	if len(sections["Seams"]) > 0 && len(seams) == 0 {
		problems = append(problems, SpecProblem{sectionLine["Seams"], "the Seams section has no table rows"})
	}

	proved := map[int]bool{}
	for _, s := range seams {
		for _, b := range s.Behaviours {
			proved[b] = true
		}
	}
	for id, line := range behaviours {
		if !proved[id] {
			problems = append(problems, SpecProblem{line, fmt.Sprintf("behaviour %d is proved by no seam", id)})
		}
	}
	sortProblems(problems)
	return seams, problems
}

func split(raw string) (int, string) {
	parts := strings.SplitN(raw, "\x00", 2)
	n, _ := strconv.Atoi(parts[0])
	return n, parts[1]
}

// placeholder is a template line nobody filled in: "<From the user's perspective.>".
func placeholder(t string) bool {
	t = strings.TrimLeft(t, "-*0123456789. ")
	return strings.HasPrefix(t, "<") && strings.HasSuffix(t, ">")
}

func sortProblems(ps []SpecProblem) {
	for i := 1; i < len(ps); i++ {
		for j := i; j > 0 && ps[j].Line < ps[j-1].Line; j-- {
			ps[j], ps[j-1] = ps[j-1], ps[j]
		}
	}
}
