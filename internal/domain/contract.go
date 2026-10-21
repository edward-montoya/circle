// Package domain holds the pure model: no I/O, no git, no processes.
//
// Everything in here is safe to construct in a test without touching a disk.
// The three contracts below are the product — project.toml is what makes a
// workflow replicable, and the rest of the binary exists to validate, enforce,
// and report on them.
package domain

import (
	"fmt"
	"sort"
	"strings"
)

// Contract is the whole of .circle/project.toml.
type Contract struct {
	Project   Project   `toml:"project"`
	Knowledge Knowledge `toml:"knowledge"`
	Execution Execution `toml:"execution"`
	Quality   Quality   `toml:"quality"`
	Gate      Gate      `toml:"gate"`
	Domain    DomainCfg `toml:"domain"`
}

// DomainCfg tunes how strictly new concepts must be written into a DOMAIN.md.
//
// The index itself is not configurable: it is always regenerated, because a
// domain model the repository has to remember to update is one it will forget.
type DomainCfg struct {
	// Enforce is "block" (the default) or "advise". Under block, a write that
	// introduces an undocumented concept is fed back to the agent as a blocking
	// reason; under advise it arrives as context.
	Enforce string `toml:"enforce,omitempty"`
	// Ignore lists identifier globs that are not domain concepts: "*Options",
	// "*Props", "Mock*". Technical suffixes like Service or Handler are already
	// stripped before matching, so this is for what is left.
	Ignore []string `toml:"ignore,omitempty"`
}

// Blocks reports whether undocumented concepts block.
func (d DomainCfg) Blocks() bool { return d.Enforce != "advise" }

type Project struct {
	Name            string `toml:"name"`
	ContractVersion int    `toml:"contract_version"`
}

// Knowledge answers: where is the truth about this project?
//
// Three classes, deliberately separate. Docs describe how the system works
// today; definitions describe what should be built; validations describe how
// correctness is proven. Coverage of definitions against a work item is what
// drives the Unknown score in v0.2.
type Knowledge struct {
	Docs        []string `toml:"docs,omitempty"`
	Definitions []string `toml:"definitions,omitempty"`
	Validations []string `toml:"validations,omitempty"`
	// Standards are the judgement-call rules a reviewer applies — the ones no
	// gate can check. Optional: a repository with good gates may need none.
	Standards []string `toml:"standards,omitempty"`
}

// Classes returns the registries in a stable order, so reports and
// hashes do not depend on map iteration.
func (k Knowledge) Classes() []struct {
	Name  string
	Paths []string
} {
	return []struct {
		Name  string
		Paths []string
	}{
		{"docs", k.Docs},
		{"definitions", k.Definitions},
		{"validations", k.Validations},
		{"standards", k.Standards},
	}
}

// Execution answers: how do I run this, in isolation?
type Execution struct {
	Compose     string   `toml:"compose"`
	AppServices []string `toml:"app_services,omitempty"`
	Bootstrap   string   `toml:"bootstrap,omitempty"`
	Up          string   `toml:"up"`
	UpFull      string   `toml:"up_full,omitempty"`
	Down        string   `toml:"down"`
	URL         string   `toml:"url,omitempty"`
	PortsFixed  bool     `toml:"ports_fixed,omitempty"`
	Worktree    Worktree `toml:"worktree"`
}

// Worktree carries the isolation settings a WorktreeCreate hook applies.
// Claude Code creates the worktree; Circle only adds the Compose namespace and
// the port allocation (D-23).
type Worktree struct {
	Isolation string `toml:"isolation,omitempty"` // "compose-project" or "" for none
	PortRange [2]int `toml:"port_range,omitempty"`
}

// Quality answers: how do I prove it is correct?
//
// Bootstrap is not a gate. It is how the gates become runnable on a fresh
// clone — a Phase 0 finding: without it, preflight reports a green contract a
// stranger cannot actually run.
type Quality struct {
	Bootstrap string            `toml:"bootstrap,omitempty"`
	Format    string            `toml:"format,omitempty"`
	Lint      string            `toml:"lint,omitempty"`
	Typecheck string            `toml:"typecheck,omitempty"`
	Test      map[string]string `toml:"test"`
	Coverage  Coverage          `toml:"coverage"`
	Extra     map[string]string `toml:"extra"`
}

type Coverage struct {
	Min int `toml:"min,omitempty"`
}

// Gate carries enforcement policy.
type Gate struct {
	// RequireSecondApprover forbids the plan's author from approving it (D-29).
	RequireSecondApprover bool `toml:"require_second_approver,omitempty"`
	// StrictDrift turns a file touched outside the approved blast radius from a
	// warning into a denial.
	StrictDrift bool `toml:"strict_drift,omitempty"`
}

// GateSpec is one named quality gate: a name and the command that proves it.
type GateSpec struct {
	Name    string
	Command string
}

// Gates returns every declared gate in a stable order.
//
// Bootstrap is excluded on purpose. It installs the gates; it is not one.
func (q Quality) Gates() []GateSpec {
	var out []GateSpec
	add := func(name, cmd string) {
		if strings.TrimSpace(cmd) != "" {
			out = append(out, GateSpec{Name: name, Command: cmd})
		}
	}
	add("format", q.Format)
	add("lint", q.Lint)
	add("typecheck", q.Typecheck)

	names := make([]string, 0, len(q.Test))
	for n := range q.Test {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		add("test:"+n, q.Test[n])
	}

	extra := make([]string, 0, len(q.Extra))
	for n := range q.Extra {
		extra = append(extra, n)
	}
	sort.Strings(extra)
	for _, n := range extra {
		add(n, q.Extra[n])
	}
	return out
}

// Gate finds a gate by name.
func (q Quality) Gate(name string) (GateSpec, bool) {
	for _, g := range q.Gates() {
		if g.Name == name {
			return g, true
		}
	}
	return GateSpec{}, false
}

// GateNames is used in error messages so a bad reference can name the real ones.
func (q Quality) GateNames() string {
	gs := q.Gates()
	names := make([]string, len(gs))
	for i, g := range gs {
		names[i] = g.Name
	}
	if len(names) == 0 {
		return "(none declared)"
	}
	return strings.Join(names, ", ")
}

func (c Contract) String() string {
	return fmt.Sprintf("%s (contract v%d, %d gates)",
		c.Project.Name, c.Project.ContractVersion, len(c.Quality.Gates()))
}
