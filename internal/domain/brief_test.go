package domain

import "testing"

func task(id string, paths ...string) Task {
	return Task{
		ID: id, Title: "t " + id, Paths: paths,
		Goal:   Goal{Statement: "does " + id},
		Verify: Verification{Kind: VerifyUnit, Gate: "test:unit"},
	}
}

func TestComputeRadiusIsTheUnionAndIsStable(t *testing.T) {
	tasks := []Task{
		task("b", "src/mw/**", "src/app.ts"),
		task("a", "src/app.ts", "Makefile"),
	}
	got := ComputeRadius(tasks)
	want := []string{"Makefile", "src/app.ts", "src/mw/**"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v (order must be stable)", got, want)
		}
	}
}

// The hash is what binds an approval to a plan. It must move when anything a
// reviewer read changes, and must NOT move when the plan is merely executed —
// otherwise implementing an approved plan would revoke its own approval.
func TestPlanHashChangesOnlyWhenThePlanChanges(t *testing.T) {
	base := []Task{task("a", "src/**"), task("b", "docs/**")}
	radius := ComputeRadius(base)
	h := ComputePlanHash(base, radius)

	t.Run("stable across reordering", func(t *testing.T) {
		swapped := []Task{base[1], base[0]}
		if got := ComputePlanHash(swapped, radius); got != h {
			t.Error("reordering the same tasks changed the hash")
		}
	})

	t.Run("stable while the plan is executed", func(t *testing.T) {
		running := []Task{base[0], base[1]}
		running[0].State = StateClosed
		running[0].ClaimedBy = "someone"
		running[0].ClosedBy = "test:unit@now"
		running[0].ClosedAt = "2026-09-02T00:00:00Z"
		if got := ComputePlanHash(running, radius); got != h {
			t.Error("executing the plan invalidated its own approval")
		}
	})

	mutations := []struct {
		name string
		mut  func([]Task) []Task
	}{
		{"a goal is reworded", func(ts []Task) []Task {
			ts[0].Goal.Statement = "something else"
			return ts
		}},
		{"the verification gate changes", func(ts []Task) []Task {
			ts[0].Verify.Gate = "test:e2e"
			return ts
		}},
		{"the verification kind changes", func(ts []Task) []Task {
			ts[0].Verify.Kind = VerifyManual
			return ts
		}},
		{"a path is added", func(ts []Task) []Task {
			ts[0].Paths = append(ts[0].Paths, "infra/**")
			return ts
		}},
		{"a dependency is added", func(ts []Task) []Task {
			ts[1].DependsOn = []string{"a"}
			return ts
		}},
		{"a task is added", func(ts []Task) []Task {
			return append(ts, task("c", "x/**"))
		}},
		{"a task is removed", func(ts []Task) []Task { return ts[:1] }},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			ts := []Task{task("a", "src/**"), task("b", "docs/**")}
			ts = tc.mut(ts)
			if got := ComputePlanHash(ts, ComputeRadius(ts)); got == h {
				t.Errorf("%s did not invalidate the approval", tc.name)
			}
		})
	}
}

func TestInRadius(t *testing.T) {
	radius := []string{"src/mw/**", "src/app.ts", "docs/", "*.md"}
	tests := []struct {
		path string
		want bool
	}{
		{"src/mw/rate-limit.ts", true},
		{"src/mw/deep/nested.ts", true},
		{"src/app.ts", true},
		{"docs/api/search.md", true},
		{"README.md", true},
		{"src/other.ts", false},
		{"infra/terraform/main.tf", false},
		{"srcx/mw/thing.ts", false}, // prefix must not match a sibling directory
		{"src/mw", false},           // the directory itself is not a file in it
	}
	for _, tc := range tests {
		if got := InRadius(tc.path, radius); got != tc.want {
			t.Errorf("InRadius(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestInRadiusBareDirectoryCoversContents(t *testing.T) {
	if !InRadius("services/api/main.go", []string{"services/api"}) {
		t.Error("a bare directory pattern must cover its contents")
	}
}

func TestDeriveRisksAreFactsNotInventions(t *testing.T) {
	tasks := []Task{task("a", "docker-compose.yml")}
	tasks[0].Verify = Verification{Kind: VerifyManual, Justification: "wording", Reviewer: "r"}
	risks := DeriveRisks(tasks, ComputeRadius(tasks), Knowledge{}, 0)

	found := map[string]bool{}
	for _, r := range risks {
		switch {
		case contains(r.Text, "human judgement"):
			found["manual"] = true
		case contains(r.Text, "No definitions"):
			found["definitions"] = true
		case contains(r.Text, "coverage floor"):
			found["coverage"] = true
		case contains(r.Text, "docker-compose.yml"):
			found["sensitive"] = true
		}
	}
	for _, k := range []string{"manual", "definitions", "coverage", "sensitive"} {
		if !found[k] {
			t.Errorf("risk %q not derived; got %+v", k, risks)
		}
	}
}

func TestDeriveRisksStaysQuietWhenThereIsNothingToSay(t *testing.T) {
	tasks := []Task{task("a", "src/**")}
	risks := DeriveRisks(tasks, ComputeRadius(tasks),
		Knowledge{Definitions: []string{"docs/adr/"}}, 80)
	if len(risks) != 0 {
		t.Errorf("a clean plan should raise no risks, got %+v", risks)
	}
}

func TestDeriveRisksCatchesAGraphWithNoEntryPoint(t *testing.T) {
	a, b := task("a"), task("b")
	a.DependsOn, b.DependsOn = []string{"b"}, []string{"a"}
	risks := DeriveRisks([]Task{a, b}, nil,
		Knowledge{Definitions: []string{"x"}}, 80)
	for _, r := range risks {
		if contains(r.Text, "no entry point") {
			return
		}
	}
	t.Errorf("a fully blocked graph must be flagged, got %+v", risks)
}

func contains(hay, needle string) bool {
	return len(hay) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(hay); i++ {
			if hay[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
