package contract

import "testing"

// A floor is a number. Without a command behind it, nothing compares anything to
// it — so ticking it green reported a measurement that was never taken. That is
// the inference this framework refuses to make about every other number it
// prints.
func TestCoverageFloorNeedsSomethingToMeasureIt(t *testing.T) {
	// Runnable, so incubation is not softening anything.
	base := `[project]
name = "x"
contract_version = 1
[execution]
compose = "docker-compose.yml"
app_services = ["api"]
up = "up"
down = "down"
[quality]
lint = "true"
`
	files := map[string]string{"docker-compose.yml": compose}

	t.Run("a floor with no command warns rather than ticking", func(t *testing.T) {
		r := repoWith(t, base+"[quality.coverage]\nmin = 70\n", files)
		c, ok := find(Validate(r), "quality", "coverage floor")
		if !ok {
			t.Fatal("no coverage check reported")
		}
		if c.Severity != Warn {
			t.Fatalf("severity = %q, want warn — nothing measures this", c.Severity)
		}
		if c.Fix == "" {
			t.Error("the advisory must say what to add; it was the only one that did not")
		}
	})

	t.Run("a floor with a command is a measurement", func(t *testing.T) {
		r := repoWith(t, base+"[quality.coverage]\nmin = 70\ncommand = \"true\"\n", files)
		c, _ := find(Validate(r), "quality", "coverage floor")
		if c.Severity != OK {
			t.Fatalf("severity = %q, want ok", c.Severity)
		}
	})

	t.Run("a command with no floor has nothing to compare against", func(t *testing.T) {
		r := repoWith(t, base+"[quality.coverage]\ncommand = \"true\"\n", files)
		c, _ := find(Validate(r), "quality", "coverage floor")
		if c.Severity != Warn {
			t.Fatalf("severity = %q, want warn", c.Severity)
		}
	})

	t.Run("neither still asks for both", func(t *testing.T) {
		r := repoWith(t, base, files)
		c, ok := find(Validate(r), "quality", "coverage floor")
		if !ok || c.Severity != Warn {
			t.Fatalf("check = %+v, want a warn", c)
		}
	})
}

// A declared coverage command must be executable by `quality run`, or the result
// is never recorded and the floor is still unproven.
func TestCoverageCommandBecomesAGate(t *testing.T) {
	r := repoWith(t, `[project]
name = "x"
[quality]
lint = "go vet ./..."
[quality.coverage]
min = 80
command = "go test -cover ./..."
`, nil)

	var found bool
	for _, g := range r.Contract.Quality.Gates() {
		if g.Name == "coverage" {
			found = true
			if g.Command != "go test -cover ./..." {
				t.Errorf("command = %q", g.Command)
			}
		}
	}
	if !found {
		t.Fatal("a declared coverage command must appear as a gate")
	}

	// And no phantom gate when it is not declared.
	r2 := repoWith(t, "[project]\nname = \"x\"\n[quality]\nlint = \"true\"\n", nil)
	for _, g := range r2.Contract.Quality.Gates() {
		if g.Name == "coverage" {
			t.Fatal("an undeclared coverage command must not produce a gate")
		}
	}
}
