package domain

import (
	"fmt"
	"strings"
)

// TaskState is the lifecycle of a unit of work.
type TaskState string

const (
	StateReady   TaskState = "ready"
	StateClaimed TaskState = "claimed"
	StateClosed  TaskState = "closed"
	StateBlocked TaskState = "blocked"
)

// VerifyKind is how a task proves itself. Closed enum on purpose: the model
// selects from it, it does not invent members (same rule as D-5).
type VerifyKind string

const (
	VerifyUnit        VerifyKind = "unit"
	VerifyIntegration VerifyKind = "integration"
	VerifyE2E         VerifyKind = "e2e"
	VerifyManual      VerifyKind = "manual"
)

func (v VerifyKind) Valid() bool {
	switch v {
	case VerifyUnit, VerifyIntegration, VerifyE2E, VerifyManual:
		return true
	}
	return false
}

// Automated reports whether closing this task can be backed by a command.
func (v VerifyKind) Automated() bool { return v != VerifyManual && v != "" }

// VerifyKinds lists the enum for error messages.
func VerifyKinds() string { return "unit, integration, e2e, manual" }

// Task is the unit of work (D-35, D-36).
//
// One word — task. Items contain tasks; a standalone task is a task with no
// parent. "Unit" was a third name for this and is gone.
//
// The two required blocks are the point: a task without a goal is a prompt, and
// a task without a verification closes on the agent's word.
type Task struct {
	ID        string       `toml:"id"        json:"id"`
	Parent    string       `toml:"parent"    json:"parent,omitempty"`
	Title     string       `toml:"title"     json:"title"`
	State     TaskState    `toml:"state"     json:"state"`
	DependsOn []string     `toml:"depends_on" json:"depends_on,omitempty"`
	Goal      Goal         `toml:"goal"      json:"goal"`
	Verify    Verification `toml:"verification" json:"verification"`

	ClaimedBy string `toml:"claimed_by" json:"claimed_by,omitempty"`
	ClosedAt  string `toml:"closed_at"  json:"closed_at,omitempty"`
	// ClosedBy names the gate run that justified closure. Empty on an open task;
	// a closed task with an empty value means the hook was bypassed.
	ClosedBy string `toml:"closed_by" json:"closed_by,omitempty"`
}

// Goal is what "done" means for this task alone. One sentence.
type Goal struct {
	Statement string `toml:"statement" json:"statement"`
}

// Verification is how the task proves itself.
type Verification struct {
	Kind    VerifyKind `toml:"kind"    json:"kind"`
	Gate    string     `toml:"gate"    json:"gate"`
	Command string     `toml:"command" json:"command,omitempty"`
	Expect  string     `toml:"expect"  json:"expect,omitempty"`

	// Manual only. Both are required when Kind is manual, so that skipping
	// automation is visible rather than free (D-37).
	Justification string `toml:"justification" json:"justification,omitempty"`
	Reviewer      string `toml:"reviewer"      json:"reviewer,omitempty"`
}

// ValidationError names the field that failed so the CLI can print a fix.
type ValidationError struct {
	Field  string
	Reason string
	Fix    string
}

func (e ValidationError) Error() string {
	if e.Fix != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Field, e.Reason, e.Fix)
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Reason)
}

// Validate enforces the task contract.
//
// This is what the TaskCreated hook calls. A non-empty result means exit 2 and
// the task is never created — the rule is structural, not advisory.
func (t Task) Validate(q Quality) []ValidationError {
	var errs []ValidationError

	if strings.TrimSpace(t.ID) == "" {
		errs = append(errs, ValidationError{"id", "empty", "give the task an id"})
	}
	if strings.TrimSpace(t.Title) == "" {
		errs = append(errs, ValidationError{"title", "empty", "give the task a title"})
	}

	if strings.TrimSpace(t.Goal.Statement) == "" {
		errs = append(errs, ValidationError{
			"goal.statement", "empty",
			"one sentence: what does done mean for this task alone?",
		})
	}

	switch {
	case t.Verify.Kind == "":
		errs = append(errs, ValidationError{
			"verification.kind", "empty", "one of: " + VerifyKinds(),
		})
	case !t.Verify.Kind.Valid():
		errs = append(errs, ValidationError{
			"verification.kind", "unknown value " + string(t.Verify.Kind),
			"one of: " + VerifyKinds(),
		})
	}

	if t.Verify.Kind == VerifyManual {
		// Manual is permitted, never free.
		if strings.TrimSpace(t.Verify.Justification) == "" {
			errs = append(errs, ValidationError{
				"verification.justification", "required when kind is manual",
				"say why this cannot be proven by a command",
			})
		}
		if strings.TrimSpace(t.Verify.Reviewer) == "" {
			errs = append(errs, ValidationError{
				"verification.reviewer", "required when kind is manual",
				"name who will judge it",
			})
		}
	} else if t.Verify.Kind.Automated() {
		if strings.TrimSpace(t.Verify.Gate) == "" {
			errs = append(errs, ValidationError{
				"verification.gate", "empty",
				"name a gate from the quality contract: " + q.GateNames(),
			})
		} else if _, ok := q.Gate(t.Verify.Gate); !ok {
			// A gate that is not declared cannot ever run, so a task pointing at
			// one could never close honestly.
			errs = append(errs, ValidationError{
				"verification.gate",
				"not declared in the quality contract: " + t.Verify.Gate,
				"declared gates: " + q.GateNames(),
			})
		}
	}

	return errs
}

// Blocked reports whether every dependency has closed.
func (t Task) Blocked(byID map[string]Task) bool {
	for _, dep := range t.DependsOn {
		d, ok := byID[dep]
		if !ok || d.State != StateClosed {
			return true
		}
	}
	return false
}

// MachineCheckable is the share of tasks whose closure a command can prove.
// This is the honest version of the "≥70% machine-checkable" target: it counts
// tasks, not prose criteria, and manual drags it down by design.
func MachineCheckable(tasks []Task) (auto, total int) {
	for _, t := range tasks {
		total++
		if t.Verify.Kind.Automated() {
			auto++
		}
	}
	return
}
