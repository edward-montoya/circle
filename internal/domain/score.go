package domain

import "fmt"

// Status is the only indicator v0.1 ships (D-27).
//
// Unknown and Complexity are cut: Status is ~100% deterministic and carries the
// whole quality argument, and the risk table predicts the other two get ignored.
//
// The governing rule (D-5): Go computes the numbers. The LLM only classifies,
// and in v0.1 it contributes nothing to this number at all.
type Status struct {
	Gates   Component
	Goal    Component
	Tasks   Component
	PRAndCI Component
	Forced  bool // preflight was bypassed; caps the score
}

// Component is one weighted part of the score, with the evidence behind it.
type Component struct {
	Weight   int
	Achieved int // 0..Weight
	Detail   string
	// Deterministic is false only when a value could not be backed by an
	// executed command. Anything false must be visible in `score explain`.
	Deterministic bool
}

const (
	WeightGates = 40
	WeightGoal  = 30
	WeightTasks = 20
	WeightPRCI  = 10

	// ForcedCap is the ceiling once preflight has been bypassed. A recorded
	// bypass with no consequence is just paperwork, so it costs something.
	ForcedCap = 60
)

// ComputeStatus is pure arithmetic over evidence. No model input, no inference.
func ComputeStatus(gatesPass, gatesTotal, goalMet, goalTotal, tasksClosed, tasksTotal int, prHealth int, forced bool) Status {
	pro := func(n, d, w int) int {
		if d <= 0 {
			return 0
		}
		return n * w / d
	}
	s := Status{
		Gates: Component{
			Weight: WeightGates, Achieved: pro(gatesPass, gatesTotal, WeightGates),
			Detail:        fmt.Sprintf("%d/%d green", gatesPass, gatesTotal),
			Deterministic: true,
		},
		Goal: Component{
			Weight: WeightGoal, Achieved: pro(goalMet, goalTotal, WeightGoal),
			Detail:        fmt.Sprintf("%d of %d verified", goalMet, goalTotal),
			Deterministic: true,
		},
		Tasks: Component{
			Weight: WeightTasks, Achieved: pro(tasksClosed, tasksTotal, WeightTasks),
			Detail:        fmt.Sprintf("%d of %d closed", tasksClosed, tasksTotal),
			Deterministic: true,
		},
		PRAndCI: Component{
			Weight: WeightPRCI, Achieved: prHealth,
			Detail:        fmt.Sprintf("%d/%d", prHealth, WeightPRCI),
			Deterministic: true,
		},
		Forced: forced,
	}
	return s
}

func (s Status) Total() int {
	t := s.Gates.Achieved + s.Goal.Achieved + s.Tasks.Achieved + s.PRAndCI.Achieved
	if s.Forced && t > ForcedCap {
		return ForcedCap
	}
	return t
}

// DeterministicPct is the share of the score backed by an executed command.
// v0.1 should always report 100 — if it ever does not, something inferred a
// number, and that is a bug rather than a feature.
func (s Status) DeterministicPct() int {
	var det, all int
	for _, c := range s.Components() {
		all += c.Weight
		if c.Deterministic {
			det += c.Weight
		}
	}
	if all == 0 {
		return 0
	}
	return det * 100 / all
}

func (s Status) Components() []Component {
	return []Component{s.Gates, s.Goal, s.Tasks, s.PRAndCI}
}

func (s Status) Labels() []string {
	return []string{"quality gates", "goal criteria", "task closure", "pr & ci"}
}
