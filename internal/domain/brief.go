package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"
)

// Brief is the human comprehension gate.
//
// Not a report produced after planning — a gate standing between planning and
// any code being written. Everything in it is assembled from state already in
// .circle/; nothing here is hand-written, and in v0.1 nothing is model-written
// either. Structure is deterministic by design (D-17): a hallucinated module
// diagram is worse than none, because it manufactures confidence at the exact
// moment a human is trusting the summary.
type Brief struct {
	Item      string    `json:"item"`
	Title     string    `json:"title"`
	Generated time.Time `json:"generated"`
	Author    string    `json:"author"`

	Tasks       []Task       `json:"tasks"`
	Radius      []string     `json:"radius"`
	Services    []ServiceRef `json:"services"`
	Definitions []string     `json:"definitions"`
	Risks       []Risk       `json:"risks"`

	// Contradicted names definitions the repository disagrees with, keyed by
	// document. Listing a contradicted document under "Definitions consulted"
	// without saying so would manufacture confidence at the exact moment a human
	// is trusting the summary — the failure D-17 exists to prevent, arriving
	// through the knowledge registry instead of through a diagram.
	Contradicted map[string][]string `json:"contradicted,omitempty"`

	// PlanHash covers everything a human would have to re-read. Approval is
	// bound to it, so a changed plan silently invalidates the approval rather
	// than carrying a stale one forward (D-15).
	PlanHash string `json:"plan_hash"`
}

// ServiceRef is a compose service the work touches.
type ServiceRef struct {
	Name  string `json:"name"`
	IsApp bool   `json:"is_app"`
}

// Risk is a fact about the plan, derived rather than imagined.
type Risk struct {
	Severity string `json:"severity"` // high | medium | low
	Text     string `json:"text"`
}

// ComputeRadius is the union of every task's declared paths.
func ComputeRadius(tasks []Task) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range tasks {
		for _, p := range t.Paths {
			if !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	sort.Strings(out)
	return out
}

// ComputePlanHash is a stable digest of everything a reviewer approves.
//
// Deliberately excludes task state, claim and closure: implementing the plan
// must not invalidate the approval of it. Only a change to what was agreed —
// the tasks, their goals, how they will be proven, and where they may write —
// voids it.
func ComputePlanHash(tasks []Task, radius []string) string {
	rows := make([]string, 0, len(tasks))
	for _, t := range tasks {
		paths := append([]string(nil), t.Paths...)
		sort.Strings(paths)
		deps := append([]string(nil), t.DependsOn...)
		sort.Strings(deps)
		rows = append(rows, strings.Join([]string{
			t.ID, t.Title, t.Goal.Statement,
			string(t.Verify.Kind), t.Verify.Gate, t.Verify.Command,
			strings.Join(paths, ","), strings.Join(deps, ","),
		}, "\x1f"))
	}
	sort.Strings(rows)
	r := append([]string(nil), radius...)
	sort.Strings(r)

	h := sha256.New()
	h.Write([]byte("brief_v1\x1e"))
	h.Write([]byte(strings.Join(rows, "\x1e")))
	h.Write([]byte("\x1e"))
	h.Write([]byte(strings.Join(r, ",")))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// DeriveRisks reads risks off the plan rather than inventing them.
//
// Every entry below is a fact the state already knows. A model could phrase
// these better; it could also invent a fourth that is not true, which is the
// failure this design refuses.
func DeriveRisks(tasks []Task, radius []string, knowledge Knowledge, coverage int) []Risk {
	var out []Risk

	var manual []string
	for _, t := range tasks {
		if t.Verify.Kind == VerifyManual {
			manual = append(manual, t.ID)
		}
	}
	if len(manual) > 0 {
		out = append(out, Risk{"medium", fmt.Sprintf(
			"%d task(s) close on human judgement rather than a command: %s. Their closure is an assertion.",
			len(manual), strings.Join(manual, ", "))})
	}

	if len(knowledge.Definitions) == 0 {
		out = append(out, Risk{"high",
			"No definitions are registered. Nothing in this repository states what *should* be built, so the plan rests on inference."})
	}

	if coverage == 0 {
		out = append(out, Risk{"low",
			"No coverage floor is configured, so the status score cannot use coverage as evidence."})
	}

	// A radius that reaches config or infrastructure is worth a human's
	// attention regardless of how modest the diff looks.
	sensitive := map[string]string{
		"docker-compose.yml": "the execution contract's own substrate",
		"Makefile":           "the build entry point",
		".github":            "CI",
		"migrations":         "schema state",
	}
	for _, p := range radius {
		for needle, why := range sensitive {
			if strings.Contains(p, needle) {
				out = append(out, Risk{"medium",
					fmt.Sprintf("The radius includes %s — %s.", p, why)})
			}
		}
	}

	blocked := 0
	idx := map[string]Task{}
	for _, t := range tasks {
		idx[t.ID] = t
	}
	for _, t := range tasks {
		if t.Blocked(idx) {
			blocked++
		}
	}
	if blocked == len(tasks) && len(tasks) > 0 {
		out = append(out, Risk{"high",
			"Every task is blocked on another. The dependency graph has no entry point."})
	}
	return out
}

// InRadius reports whether a repo-relative path is covered.
//
// Supports a trailing /** for whole subtrees, and a bare directory prefix,
// because those are what people actually write.
func InRadius(rel string, radius []string) bool {
	rel = strings.TrimPrefix(rel, "./")
	for _, pat := range radius {
		pat = strings.TrimPrefix(pat, "./")
		if pat == rel {
			return true
		}
		if strings.HasSuffix(pat, "/**") {
			if strings.HasPrefix(rel, strings.TrimSuffix(pat, "**")) {
				return true
			}
			continue
		}
		if strings.HasSuffix(pat, "/") && strings.HasPrefix(rel, pat) {
			return true
		}
		if ok, _ := path.Match(pat, rel); ok {
			return true
		}
		// A directory pattern without a trailing slash still covers its contents.
		if !strings.ContainsAny(pat, "*?[") && strings.HasPrefix(rel, pat+"/") {
			return true
		}
	}
	return false
}
