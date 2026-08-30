package contract

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/edwardmontoya/circle/internal/domain"
)

// repoWith builds a throwaway repository on disk. Validation is inherently I/O
// against a working tree, so the tests build one rather than mock it.
func repoWith(t *testing.T, contractTOML string, files map[string]string) *Repo {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, Dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, Dir, "project.toml"),
		[]byte(contractTOML), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return r
}

const compose = `services:
  api:
    build: .
  web:
    build: ./frontend
  redis:
    image: redis:7
`

// find returns the check for a section/label pair.
func find(res Result, section, label string) (Check, bool) {
	for _, c := range res.Checks {
		if c.Section == section && c.Label == label {
			return c, true
		}
	}
	return Check{}, false
}

func TestComposeServices(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(p, []byte(compose), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := composeServices(p)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"api", "web", "redis"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("service %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestValidateAppServicesMustExistInCompose(t *testing.T) {
	// A typo here silently corrupts the execution contract, so it blocks rather
	// than warns — and the error names the services that do exist.
	r := repoWith(t, `
[execution]
compose = "docker-compose.yml"
app_services = ["api", "wbe"]
up = "docker compose up"
down = "docker compose down"
[quality]
lint = "true"
`, map[string]string{"docker-compose.yml": compose})

	res := Validate(r)
	c, ok := find(res, "execution", "app_services")
	if !ok {
		t.Fatal("no app_services check")
	}
	if c.Severity != Fail {
		t.Errorf("severity = %q, want fail", c.Severity)
	}
	if !res.Blocked() {
		t.Error("a typo in app_services must block")
	}
}

func TestValidateDeadKnowledgePathBlocks(t *testing.T) {
	r := repoWith(t, `
[execution]
compose = "docker-compose.yml"
app_services = ["api"]
up = "u"
down = "d"
[quality]
lint = "true"
[knowledge]
docs = ["README.md", "docs/nope/"]
`, map[string]string{
		"docker-compose.yml": compose,
		"README.md":          "# hi",
	})

	res := Validate(r)
	if !res.Blocked() {
		t.Fatal("a knowledge path resolving to nothing must block")
	}
	c, _ := find(res, "knowledge", "paths")
	if c.Detail != "1 of 2 resolve" {
		t.Errorf("ratio = %q, want \"1 of 2 resolve\"", c.Detail)
	}
}

// The Phase 0 finding: a gate whose binary is absent is not runnable *yet*.
// With a bootstrap that is advisory; without one the contract cannot be honoured
// on a fresh clone, which is the whole promise, so it blocks.
func TestUnrunnableGateBlocksOnlyWithoutBootstrap(t *testing.T) {
	base := `
[execution]
compose = "docker-compose.yml"
app_services = ["api"]
up = "u"
down = "d"
`
	files := map[string]string{"docker-compose.yml": compose}

	t.Run("no bootstrap blocks", func(t *testing.T) {
		r := repoWith(t, base+"\n[quality]\nlint = \"definitely-not-a-real-binary-xyz\"\n", files)
		res := Validate(r)
		if !res.Blocked() {
			t.Error("an unrunnable gate with no bootstrap must block")
		}
	})

	t.Run("bootstrap downgrades to advisory", func(t *testing.T) {
		r := repoWith(t, base+"\n[quality]\nbootstrap = \"uv sync\"\nlint = \"definitely-not-a-real-binary-xyz\"\n", files)
		res := Validate(r)
		if res.Blocked() {
			t.Error("with a bootstrap declared the gate is advisory, not blocking")
		}
		c, _ := find(res, "quality", "lint")
		if c.Fix != "run: uv sync" {
			t.Errorf("fix = %q, want the bootstrap command", c.Fix)
		}
	})
}

func TestBootstrapIsNeverAGate(t *testing.T) {
	r := repoWith(t, `
[execution]
compose = "docker-compose.yml"
app_services = ["api"]
up = "u"
down = "d"
[quality]
bootstrap = "uv sync"
lint = "true"
`, map[string]string{"docker-compose.yml": compose})

	for _, g := range r.Contract.Quality.Gates() {
		if g.Name == "bootstrap" {
			t.Fatal("bootstrap counted as a gate — a contract could inflate its gate count with an install command")
		}
	}
}

func TestBinaryOf(t *testing.T) {
	tests := []struct{ cmd, want string }{
		{"pytest", "pytest"},
		{"cd frontend && npx tsc -b", "npx"},
		{"cd a && cd b && ruff check .", "ruff"},
		{"CI=1 pytest -q", "pytest"},
		{"", ""},
	}
	for _, tc := range tests {
		if got := binaryOf(tc.cmd); got != tc.want {
			t.Errorf("binaryOf(%q) = %q, want %q", tc.cmd, got, tc.want)
		}
	}
}

func TestForcedPreflightIsSurfaced(t *testing.T) {
	// A bypass with no consequence is paperwork. It must be visible.
	r := repoWith(t, `
[execution]
compose = "docker-compose.yml"
app_services = ["api"]
up = "u"
down = "d"
[quality]
lint = "true"
`, map[string]string{"docker-compose.yml": compose})

	if err := os.MkdirAll(r.RuntimeDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(r.RuntimeDir("forced"), []byte("demo in 20 min"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := Validate(r)
	if !res.Forced {
		t.Fatal("a recorded bypass must set Forced so the score can be capped")
	}
	if c, ok := find(res, "preflight", "forced"); !ok || c.Detail != "demo in 20 min" {
		t.Error("the recorded reason must be surfaced, not just the fact of a bypass")
	}
}

func TestFindRootPrefersNearestCircleDir(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(filepath.Join(root, Dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := FindRoot(nested)
	if err != nil {
		t.Fatal(err)
	}
	// Resolution walks up from cwd. Inside a worktree, CLAUDE_PROJECT_DIR still
	// points at the main checkout while cwd follows Claude — trusting the former
	// would validate the wrong repository.
	want, _ := filepath.EvalSymlinks(root)
	gotResolved, _ := filepath.EvalSymlinks(got)
	if gotResolved != want {
		t.Errorf("FindRoot = %q, want %q", gotResolved, want)
	}
}

func TestLoadMissingContract(t *testing.T) {
	if _, err := Load(t.TempDir()); err != ErrNoContract {
		t.Errorf("err = %v, want ErrNoContract", err)
	}
}

func TestGatesAreStablyOrdered(t *testing.T) {
	// Map iteration is random in Go; a report whose line order changes between
	// runs is unreadable and unhashable.
	q := domain.Quality{
		Lint:      "l",
		Format:    "f",
		Typecheck: "t",
		Test:      map[string]string{"unit": "u", "e2e": "e", "integration": "i"},
	}
	first := q.Gates()
	for i := 0; i < 20; i++ {
		got := q.Gates()
		for j := range first {
			if got[j].Name != first[j].Name {
				t.Fatalf("gate order is not stable: %v vs %v", got, first)
			}
		}
	}
}
