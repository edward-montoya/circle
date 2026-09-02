package knowledge

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/edwardmontoya/circle/internal/contract"
)

func repoWith(t *testing.T, files map[string]string, definitions ...string) *contract.Repo {
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
	toml := "[project]\nname = \"t\"\n\n[execution]\ncompose = \"docker-compose.yml\"\n\n[knowledge]\ndefinitions = ["
	for i, d := range definitions {
		if i > 0 {
			toml += ", "
		}
		toml += "\"" + d + "\""
	}
	toml += "]\n"
	if err := os.MkdirAll(filepath.Join(root, contract.Dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, contract.Dir, "project.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := contract.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

const nodeCompose = `services:
  web:
    build: .
  mongo:
    image: mongo:7
`

func kinds(fs []Finding) map[Kind][]string {
	out := map[Kind][]string{}
	for _, f := range fs {
		out[f.Kind] = append(out[f.Kind], f.Claim)
	}
	return out
}

func has(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

// The scenario this whole package exists for: a document registered as
// authoritative that describes a completely different system. Before this, the
// registry reported `✓ definitions docs/architecture.md → 1` and preflight
// exited 0 — a green tick on a lie.
func TestDetectsAStackThatIsNotThere(t *testing.T) {
	r := repoWith(t, map[string]string{
		"docker-compose.yml": nodeCompose,
		"package.json":       `{"name":"x"}`,
		"docs/architecture.md": "# Architecture\n\n" +
			"The service is written in **Go** and stores data in **PostgreSQL**.\n" +
			"Background jobs run through a **RabbitMQ** queue.\n\n" +
			"- `internal/billing/` — invoice generation\n",
	}, "docs/architecture.md")

	got := kinds(mustVerify(t, r))

	for _, want := range []string{"Go", "PostgreSQL", "RabbitMQ"} {
		if !has(got[AbsentStack], want) {
			t.Errorf("did not detect %q as absent; got %v", want, got[AbsentStack])
		}
	}
	if !has(got[MissingPath], "internal/billing") {
		t.Errorf("did not detect the missing path; got %v", got[MissingPath])
	}
}

// Markdown writes technology names in bold constantly, and emphasis markers
// break word-boundary matching. A detector that misses **Go** misses most real
// documents.
func TestEmphasisDoesNotHideAClaim(t *testing.T) {
	r := repoWith(t, map[string]string{
		"docker-compose.yml": nodeCompose,
		"package.json":       `{}`,
		"docs/a.md":          "Written in **Go**, backed by _PostgreSQL_.\n",
	}, "docs/a.md")

	got := kinds(mustVerify(t, r))
	if !has(got[AbsentStack], "Go") || !has(got[AbsentStack], "PostgreSQL") {
		t.Fatalf("emphasis hid a claim; got %v", got[AbsentStack])
	}
}

// The cost of a false positive is higher than a miss: it teaches people to
// ignore the report. These are the cases that must stay silent.
func TestNoFalsePositives(t *testing.T) {
	r := repoWith(t, map[string]string{
		"docker-compose.yml": nodeCompose,
		"package.json":       `{"name":"x"}`,
		"src/index.js":       "//",
		"docs/a.md": "# Design\n\n" +
			"A **Node.js** service backed by **MongoDB**.\n" +
			"Entry point is `src/index.js`.\n" +
			"Docs live at `https://example.com/a/b`.\n" +
			"Run `npm --version` to check.\n",
	}, "docs/a.md")

	if fs := mustVerify(t, r); len(fs) != 0 {
		t.Fatalf("a document that agrees with the repo raised %d finding(s): %+v", len(fs), fs)
	}
}

func TestOnlyDefinitionsAreChecked(t *testing.T) {
	// Docs describe how the system works today and go stale harmlessly.
	// Definitions describe what should be built, and a wrong one misdirects the
	// plan — so only those are held to this standard.
	r := repoWith(t, map[string]string{
		"docker-compose.yml": nodeCompose,
		"package.json":       `{}`,
		"docs/old.md":        "This was once written in **Go** with **PostgreSQL**.\n",
	}) // registered under nothing
	if fs := mustVerify(t, r); len(fs) != 0 {
		t.Fatalf("an unregistered document must not be checked; got %+v", fs)
	}
}

func TestAgreeingDocumentProducesNothing(t *testing.T) {
	r := repoWith(t, map[string]string{
		"docker-compose.yml": nodeCompose,
		"package.json":       `{}`,
		"docs/a.md":          "A **Node** service on **MongoDB**.\n",
	}, "docs/a.md")
	if fs := mustVerify(t, r); len(fs) != 0 {
		t.Fatalf("got %+v, want none", fs)
	}
}

func TestFindingCarriesItsEvidence(t *testing.T) {
	// A finding a human cannot check in one second is a finding they will learn
	// to skip, so the line number and the excerpt are not optional.
	r := repoWith(t, map[string]string{
		"docker-compose.yml": nodeCompose,
		"package.json":       `{}`,
		"docs/a.md":          "line one\nline two\nStores data in **PostgreSQL**.\n",
	}, "docs/a.md")

	fs := mustVerify(t, r)
	if len(fs) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(fs), fs)
	}
	if fs[0].Line != 3 {
		t.Errorf("line = %d, want 3", fs[0].Line)
	}
	if fs[0].Excerpt == "" || fs[0].Reality == "" {
		t.Errorf("a finding without evidence is unactionable: %+v", fs[0])
	}
}

func mustVerify(t *testing.T, r *contract.Repo) []Finding {
	t.Helper()
	fs, err := Verify(r)
	if err != nil {
		t.Fatal(err)
	}
	return fs
}
