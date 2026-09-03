package contract

import "testing"

// A new project and a broken contract look identical to a naive check and
// deserve opposite treatment. These tests pin the difference, because getting it
// wrong in either direction is bad in a different way: block a new project and
// it cannot write its first file; wave a half-declared contract through and the
// gate is decorative.
func TestIncubatingLifecycle(t *testing.T) {
	minimal := `[project]
name = "new"
contract_version = 1
`
	t.Run("day zero: nothing declared, nothing on disk", func(t *testing.T) {
		r := repoWith(t, minimal, nil)
		res := Validate(r)

		if !res.Incubating {
			t.Fatal("an empty repository must be incubating, not broken")
		}
		if res.Blocked() {
			t.Errorf("incubating must not block; got %d failures: %+v",
				res.Count(Fail), res.Checks)
		}
		// The gaps are still reported — silence would be as dishonest as a pass.
		if c, ok := find(res, "execution", "compose"); !ok || c.Severity != Warn {
			t.Errorf("the missing execution contract must still be surfaced, got %+v", c)
		}
	})

	t.Run("a compose file appears: the gate closes again", func(t *testing.T) {
		r := repoWith(t, minimal, map[string]string{"docker-compose.yml": compose})
		res := Validate(r)

		if res.Incubating {
			t.Fatal("a repository with something to run is no longer incubating")
		}
		if !res.Blocked() {
			t.Fatal("a compose file that exists but is undeclared must block")
		}
		c, _ := find(res, "execution", "compose")
		if c.Severity != Fail {
			t.Errorf("severity = %q, want fail", c.Severity)
		}
		// The fix has to be the exact line to paste, not a category of advice.
		if c.Fix != `set execution.compose = "docker-compose.yml"` {
			t.Errorf("fix = %q, want the literal assignment", c.Fix)
		}
	})

	t.Run("declared but empty is a lie, not a young project", func(t *testing.T) {
		// The dangerous case: a contract that claims an execution model while the
		// file it names is absent. Incubating must not launder this.
		r := repoWith(t, minimal+`
[execution]
compose = "docker-compose.yml"
`, nil)
		res := Validate(r)
		if res.Incubating {
			t.Fatal("declaring a compose file makes the repo answerable for it")
		}
		if !res.Blocked() {
			t.Fatal("a contract naming a file that does not exist must block")
		}
	})

	t.Run("gates alone end incubation", func(t *testing.T) {
		r := repoWith(t, minimal+`
[quality]
lint = "true"
`, nil)
		res := Validate(r)
		if res.Incubating {
			t.Fatal("a repository with something to prove is no longer incubating")
		}
	})

	// Incubation softens the execution and quality checks because there is
	// genuinely nothing to run or prove yet. It must not reach sections that have
	// nothing to do with running: a knowledge path that resolves to nothing is
	// wrong on day zero for the same reason it is wrong on day ninety.
	t.Run("a young project still blocks on a knowledge path that resolves to nothing", func(t *testing.T) {
		r := repoWith(t, minimal+`
[knowledge]
docs = ["docs/does-not-exist.md"]
`, nil)
		res := Validate(r)
		if !res.Incubating {
			t.Fatal("no compose and no gates is still nothing to run or prove")
		}
		if !res.Blocked() {
			t.Fatalf("a knowledge path resolving to nothing must block; got %+v", res.Checks)
		}
	})

	t.Run("a young project with resolvable knowledge does not block", func(t *testing.T) {
		// The other half. Declaring knowledge early must not be punished by the
		// execution and quality gaps incubation is there to excuse.
		r := repoWith(t, minimal+`
[knowledge]
docs = ["README.md"]
`, map[string]string{"README.md": "# x\n"})
		res := Validate(r)
		if !res.Incubating {
			t.Fatal("declaring knowledge is not something to run or prove")
		}
		if res.Blocked() {
			t.Fatalf("a resolvable knowledge path must not block; got %+v", res.Checks)
		}
	})
}

// Once a project can run, it must be able to say how it is proven — otherwise no
// task can declare a verification, no task can be created, and the brief gate
// never engages. The framework would be installed and inert.
func TestRunnableWithoutGatesBlocks(t *testing.T) {
	r := repoWith(t, `[project]
name = "x"
[execution]
compose = "docker-compose.yml"
app_services = ["api"]
up = "docker compose up"
down = "docker compose down"
`, map[string]string{"docker-compose.yml": compose})

	res := Validate(r)
	if res.Incubating {
		t.Fatal("this repo has something to run")
	}
	if !res.Blocked() {
		t.Fatal("a runnable project with no gates must block")
	}
	c, ok := find(res, "quality", "gates")
	if !ok {
		t.Fatal("no gates check")
	}
	// The message has to earn the block by naming the consequence.
	if c.Fix == "" || len(c.Fix) < 40 {
		t.Errorf("fix = %q, want it to explain why a gate is required", c.Fix)
	}
}
