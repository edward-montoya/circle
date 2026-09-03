package domain

import "time"

// EventType is a closed set. The event schema is ours and versioned, so a
// change in Claude Code's internals cannot silently reshape our history (D-3).
type EventType string

const (
	EvGateRun        EventType = "gate.run"        // a quality gate executed
	EvGateDeny       EventType = "gate.deny"       // a write was denied
	EvPreflight      EventType = "preflight"       // preflight ran
	EvPreflightForce EventType = "preflight.force" // preflight was bypassed
	// EvPreflightForceClear is recorded rather than the force event being
	// deleted: the bypass happened, and the history says so even after it is
	// lifted.
	EvPreflightForceClear EventType = "preflight.force.clear" // a bypass was lifted
	EvTaskCreate          EventType = "task.create"
	EvTaskClaim           EventType = "task.claim"
	EvTaskClose           EventType = "task.close"
	EvBriefGenerate       EventType = "brief.generate"
	EvBriefApprove        EventType = "brief.approve"
	EvBriefReject         EventType = "brief.reject"
	EvSessionOpen         EventType = "session.open"
	EvSessionClose        EventType = "session.close"
	EvToolUse             EventType = "tool.use"
	EvWorktreeUp          EventType = "worktree.provision"
	EvWorktreeDown        EventType = "worktree.release"
)

// SchemaVersion is bumped when the event shape changes incompatibly.
const SchemaVersion = 1

// Event is one append-only record.
//
// Everything the status score reads is one of these. A number with no event
// behind it is an inference, and the framework does not report inferences.
type Event struct {
	Schema  int       `json:"schema"`
	At      time.Time `json:"at"`
	Type    EventType `json:"type"`
	Item    string    `json:"item,omitempty"`
	Task    string    `json:"task,omitempty"`
	Session string    `json:"session,omitempty"`

	// Gate results.
	Gate     string `json:"gate,omitempty"`
	Command  string `json:"command,omitempty"`
	ExitCode int    `json:"exit_code"`
	DurMS    int64  `json:"duration_ms,omitempty"`

	// Free-form context: the deny reason, the force reason, the approver.
	Actor  string `json:"actor,omitempty"`
	Reason string `json:"reason,omitempty"`
	Path   string `json:"path,omitempty"`
}

// Passed reports whether a gate.run event represents success.
func (e Event) Passed() bool { return e.Type == EvGateRun && e.ExitCode == 0 }

func NewEvent(t EventType) Event {
	return Event{Schema: SchemaVersion, At: time.Now().UTC(), Type: t}
}
