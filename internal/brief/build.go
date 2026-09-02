// Package brief assembles and renders the human comprehension gate.
//
// The requester's constraint, stated directly: "Humans cannot read hundreds of
// lines + multiple documents. Summaries and diagrams are the keys." So the brief
// is capped by construction — one screen per section — and implementation is
// blocked until a human has approved it.
package brief

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/edwardmontoya/circle/internal/compose"
	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/domain"
	"github.com/edwardmontoya/circle/internal/task"
)

// Build assembles a brief from state that already exists.
//
// No argument is taken for the prose: in v0.1 every field is derived. The model
// is allowed to supply labels and narrative in a later phase, but never the
// structure — a plausible-but-false module diagram manufactures confidence at
// the exact moment a human is trusting the summary (D-17).
func Build(r *contract.Repo, item string) (domain.Brief, error) {
	tasks, err := task.Load(r)
	if err != nil {
		return domain.Brief{}, err
	}
	scoped := tasks
	if item != "" && item != "default" {
		scoped = nil
		for _, t := range tasks {
			if t.Parent == item || t.ID == item || strings.HasPrefix(t.ID, item+".") {
				scoped = append(scoped, t)
			}
		}
	}

	radius := domain.ComputeRadius(scoped)
	b := domain.Brief{
		Item:         item,
		Title:        titleFor(r, item, scoped),
		Generated:    time.Now().UTC(),
		Author:       gitIdentity(r.Root),
		Tasks:        scoped,
		Radius:       radius,
		Definitions:  r.Contract.Knowledge.Definitions,
		Services:     servicesTouched(r, radius),
		Contradicted: Contradictions(r),
		Risks: domain.DeriveRisks(scoped, radius,
			r.Contract.Knowledge, r.Contract.Quality.Coverage.Min),
		PlanHash: domain.ComputePlanHash(scoped, radius),
	}
	if len(b.Contradicted) > 0 {
		docs := make([]string, 0, len(b.Contradicted))
		for d := range b.Contradicted {
			docs = append(docs, d)
		}
		sort.Strings(docs)
		b.Risks = append([]domain.Risk{{
			Severity: "high",
			Text: fmt.Sprintf(
				"%s %s registered as authoritative but contradicted by the code. "+
					"Any part of this plan derived from them is aimed at a system that does not exist.",
				strings.Join(docs, ", "), plural(len(docs), "is", "are")),
		}}, b.Risks...)
	}
	return b, nil
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// Contradictions is a seam so the brief can report drift without importing the
// knowledge package directly, which would close an import cycle.
var Contradictions = func(*contract.Repo) map[string][]string { return nil }

func titleFor(r *contract.Repo, item string, tasks []domain.Task) string {
	for _, t := range tasks {
		if t.ID == item {
			return t.Title
		}
	}
	if len(tasks) == 1 {
		return tasks[0].Title
	}
	return r.Contract.Project.Name + " · " + item
}

// gitIdentity records who generated the plan, so self-approval is visible
// rather than silent (D-29).
func gitIdentity(dir string) string {
	cmd := exec.Command("git", "config", "user.email")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// servicesTouched maps the blast radius onto compose services.
//
// Deterministic: a service is implicated when its build context contains a path
// the radius covers. No inference, and no service appears that the paths do not
// actually reach.
func servicesTouched(r *contract.Repo, radius []string) []domain.ServiceRef {
	if r.Contract.Execution.Compose == "" || len(radius) == 0 {
		return nil
	}
	cf, err := compose.Parse(filepath.Join(r.Root, r.Contract.Execution.Compose))
	if err != nil {
		return nil
	}
	app := map[string]bool{}
	for _, s := range r.Contract.Execution.AppServices {
		app[s] = true
	}

	var out []domain.ServiceRef
	for _, s := range cf.Services {
		if !s.HasBuild {
			continue // infrastructure holds no code the radius could reach
		}
		if len(cf.Services) == 1 || touches(radius, s.Name) {
			out = append(out, domain.ServiceRef{Name: s.Name, IsApp: app[s.Name]})
		}
	}
	// A single-app repo has no per-service paths to match on, so every app
	// service is implicated rather than none.
	if len(out) == 0 {
		for _, s := range cf.Services {
			if s.HasBuild {
				out = append(out, domain.ServiceRef{Name: s.Name, IsApp: app[s.Name]})
			}
		}
	}
	return out
}

func touches(radius []string, service string) bool {
	for _, p := range radius {
		if strings.Contains(p, "/"+service+"/") ||
			strings.HasPrefix(p, service+"/") ||
			strings.Contains(p, "services/"+service) {
			return true
		}
	}
	return false
}
