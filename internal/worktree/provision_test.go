package worktree

import (
	"os"
	"strings"
	"testing"
)

func plan() Plan {
	return Plan{
		Item: "task-118", Project: "circle-task-118", Worktree: "/tmp/wt",
		Ports: []Remap{
			{Service: "api", Container: 8000, From: 8000, To: 42000},
			{Service: "web", Container: 80, From: 8080, To: 42001},
		},
	}
}

// The bug this test exists for: Compose's default merge strategy for a sequence
// is APPEND. Without `!override` the original host port stays published next to
// the new one, so two worktrees still collide — and `docker compose config`
// reports four published ports where there should be two. The stack reads as
// isolated and is not, which is the exact failure mode the whole phase is
// meant to eliminate.
func TestOverrideReplacesPortsRatherThanAppending(t *testing.T) {
	out := RenderOverride(plan())
	if strings.Count(out, "ports: !override") != 2 {
		t.Fatalf("every ports list needs !override, got:\n%s", out)
	}
	if strings.Contains(out, "ports:\n") && !strings.Contains(out, "ports: !override") {
		t.Error("a bare `ports:` key would append to the original list")
	}
}

func TestOverrideCarriesOnlyRemappedPorts(t *testing.T) {
	out := RenderOverride(plan())
	for _, want := range []string{`"42000:8000"`, `"42001:80"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in:\n%s", want, out)
		}
	}
	// The original host ports may appear only inside the trailing `# was N`
	// comment, never as a published mapping.
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, "- \"") {
			continue
		}
		mapping := line
		if i := strings.Index(line, "#"); i >= 0 {
			mapping = line[:i]
		}
		if strings.Contains(mapping, `"8000:8000"`) || strings.Contains(mapping, `"8080:80"`) {
			t.Errorf("original port still published: %q", line)
		}
	}
}

func TestOverrideWithNoPortsIsStillValidCompose(t *testing.T) {
	out := RenderOverride(Plan{Item: "x", Project: "circle-x"})
	if !strings.Contains(out, "services: {}") {
		t.Errorf("an empty override must still parse as compose:\n%s", out)
	}
}

func TestOverridePreservesHostBinding(t *testing.T) {
	p := plan()
	p.Ports = []Remap{{Service: "api", Container: 8000, From: 4000, To: 42000, Bind: "127.0.0.1"}}
	if out := RenderOverride(p); !strings.Contains(out, `"127.0.0.1:42000:8000"`) {
		t.Errorf("host binding lost:\n%s", out)
	}
}

func TestRenderMapRecordsSkippedEntries(t *testing.T) {
	p := plan()
	p.Skipped = map[string][]string{"api": {"3000-3005:3000-3005"}}
	out := RenderMap(p)
	if !strings.Contains(out, "[[skipped]]") || !strings.Contains(out, "3000-3005") {
		t.Errorf("an unparseable entry must be recorded, not dropped:\n%s", out)
	}
}

func TestUpsertEnvPreservesExistingKeys(t *testing.T) {
	// The worktree may already carry a .env via .worktreeinclude; clobbering it
	// would take the developer's own configuration with it.
	dir := t.TempDir()
	path := dir + "/.env"
	if err := writeFile(path, "SECRET=keep\nCOMPOSE_PROJECT_NAME=old\n"); err != nil {
		t.Fatal(err)
	}
	if err := upsertEnv(path, "COMPOSE_PROJECT_NAME", "circle-new"); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, path)
	if !strings.Contains(got, "SECRET=keep") {
		t.Errorf("existing key lost:\n%s", got)
	}
	if !strings.Contains(got, "COMPOSE_PROJECT_NAME=circle-new") {
		t.Errorf("key not updated:\n%s", got)
	}
	if strings.Contains(got, "COMPOSE_PROJECT_NAME=old") {
		t.Errorf("stale value left behind:\n%s", got)
	}
}

func TestUpsertEnvCreatesMissingFile(t *testing.T) {
	path := t.TempDir() + "/.env"
	if err := upsertEnv(path, "COMPOSE_PROJECT_NAME", "circle-x"); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, path); !strings.Contains(got, "circle-x") {
		t.Errorf("got %q", got)
	}
}

func writeFile(path, body string) error { return os.WriteFile(path, []byte(body), 0o644) }

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
