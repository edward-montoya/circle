package domain

import "testing"

func TestComputeStatus(t *testing.T) {
	tests := []struct {
		name                    string
		gatesPass, gatesTotal   int
		goalMet, goalTotal      int
		tasksClosed, tasksTotal int
		prHealth                int
		forced                  bool
		want                    int
	}{
		{"nothing done", 0, 5, 0, 4, 0, 4, 0, false, 0},
		{"everything done", 5, 5, 4, 4, 4, 4, 10, false, 100},
		{"gates only", 5, 5, 0, 4, 0, 4, 0, false, 40},
		{"no denominators is zero, not a panic", 0, 0, 0, 0, 0, 0, 0, false, 0},
		{"partial", 4, 5, 3, 4, 3, 4, 0, false, 32 + 22 + 15},

		// A recorded bypass with no consequence is paperwork. This one costs.
		{"forced caps a perfect score", 5, 5, 4, 4, 4, 4, 10, true, ForcedCap},
		{"forced does not raise a low score", 1, 5, 0, 4, 0, 4, 0, true, 8},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := ComputeStatus(tc.gatesPass, tc.gatesTotal, tc.goalMet, tc.goalTotal,
				tc.tasksClosed, tc.tasksTotal, tc.prHealth, tc.forced)
			if got := s.Total(); got != tc.want {
				t.Errorf("Total() = %d, want %d", got, tc.want)
			}
		})
	}
}

// v0.1 must always report 100% deterministic. If it ever does not, something
// inferred a number, and that is a bug rather than a feature.
func TestStatusIsFullyDeterministic(t *testing.T) {
	s := ComputeStatus(1, 2, 1, 2, 1, 2, 5, false)
	if got := s.DeterministicPct(); got != 100 {
		t.Fatalf("DeterministicPct() = %d, want 100 — a component was not backed by evidence", got)
	}
}

func TestWeightsSumToOneHundred(t *testing.T) {
	if got := WeightGates + WeightGoal + WeightTasks + WeightPRCI; got != 100 {
		t.Fatalf("weights sum to %d, want 100", got)
	}
}
