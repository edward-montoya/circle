package domain

import "testing"

// quality is a contract with two declared gates, used across the table below.
func quality() Quality {
	return Quality{
		Lint: "ruff check .",
		Test: map[string]string{"unit": "pytest", "e2e": "playwright test"},
	}
}

func TestTaskValidate(t *testing.T) {
	valid := Task{
		ID: "task-1.1", Title: "add limiter",
		Goal:   Goal{Statement: "A limited client gets a 429."},
		Verify: Verification{Kind: VerifyE2E, Gate: "test:e2e"},
	}

	tests := []struct {
		name  string
		mut   func(*Task)
		field string // the field expected to fail; "" means valid
	}{
		{"valid automated task", func(*Task) {}, ""},
		{"missing id", func(t *Task) { t.ID = "" }, "id"},
		{"missing title", func(t *Task) { t.Title = "" }, "title"},

		// The two rules the whole task contract exists for.
		{"missing goal", func(t *Task) { t.Goal.Statement = "" }, "goal.statement"},
		{"whitespace goal", func(t *Task) { t.Goal.Statement = "   \n" }, "goal.statement"},
		{"missing verification kind", func(t *Task) { t.Verify.Kind = "" }, "verification.kind"},
		{"unknown verification kind", func(t *Task) { t.Verify.Kind = "vibes" }, "verification.kind"},
		{"missing gate", func(t *Task) { t.Verify.Gate = "" }, "verification.gate"},

		// A gate that is not declared can never run, so a task pointing at one
		// could never close honestly.
		{"gate not in contract", func(t *Task) { t.Verify.Gate = "test:smoke" }, "verification.gate"},

		// Manual is permitted, never free (D-37).
		{"manual without justification", func(t *Task) {
			t.Verify = Verification{Kind: VerifyManual, Reviewer: "eng@acme.co"}
		}, "verification.justification"},
		{"manual without reviewer", func(t *Task) {
			t.Verify = Verification{Kind: VerifyManual, Justification: "wording"}
		}, "verification.reviewer"},
		{"manual complete", func(t *Task) {
			t.Verify = Verification{Kind: VerifyManual, Justification: "wording", Reviewer: "eng@acme.co"}
		}, ""},
		{"manual needs no gate", func(t *Task) {
			t.Verify = Verification{Kind: VerifyManual, Justification: "w", Reviewer: "r", Gate: ""}
		}, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := valid
			tc.mut(&task)
			errs := task.Validate(quality())

			if tc.field == "" {
				if len(errs) != 0 {
					t.Fatalf("expected valid, got %v", errs)
				}
				return
			}
			for _, e := range errs {
				if e.Field == tc.field {
					return
				}
			}
			t.Fatalf("expected a failure on %q, got %v", tc.field, errs)
		})
	}
}

func TestVerifyKindAutomated(t *testing.T) {
	tests := []struct {
		kind VerifyKind
		auto bool
	}{
		{VerifyUnit, true},
		{VerifyIntegration, true},
		{VerifyE2E, true},
		{VerifyManual, false},
		{"", false},
	}
	for _, tc := range tests {
		if got := tc.kind.Automated(); got != tc.auto {
			t.Errorf("%q.Automated() = %v, want %v", tc.kind, got, tc.auto)
		}
	}
}

func TestBlocked(t *testing.T) {
	idx := map[string]Task{
		"a": {ID: "a", State: StateClosed},
		"b": {ID: "b", State: StateReady},
	}
	tests := []struct {
		name string
		deps []string
		want bool
	}{
		{"no dependencies", nil, false},
		{"dependency closed", []string{"a"}, false},
		{"dependency open", []string{"b"}, true},
		{"one of two open", []string{"a", "b"}, true},
		{"dependency missing entirely", []string{"ghost"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := Task{ID: "x", DependsOn: tc.deps}
			if got := task.Blocked(idx); got != tc.want {
				t.Errorf("Blocked() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMachineCheckable(t *testing.T) {
	ts := []Task{
		{Verify: Verification{Kind: VerifyUnit}},
		{Verify: Verification{Kind: VerifyE2E}},
		{Verify: Verification{Kind: VerifyManual}},
	}
	auto, total := MachineCheckable(ts)
	if auto != 2 || total != 3 {
		t.Fatalf("got %d/%d, want 2/3", auto, total)
	}
}

func TestQualityGatesExcludesBootstrap(t *testing.T) {
	// Bootstrap installs the gates; it is not one. Counting it would let a
	// contract inflate its gate count with an install command.
	q := Quality{Bootstrap: "uv sync", Lint: "ruff check ."}
	for _, g := range q.Gates() {
		if g.Name == "bootstrap" {
			t.Fatal("bootstrap must not be reported as a gate")
		}
	}
	if len(q.Gates()) != 1 {
		t.Fatalf("got %d gates, want 1", len(q.Gates()))
	}
}
