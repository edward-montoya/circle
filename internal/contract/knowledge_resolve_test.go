package contract

import (
	"os"
	"path/filepath"
	"testing"
)

// mkdir creates an empty directory inside the repo, which repoWith cannot do:
// it takes a file map, and an empty directory has no files in it. That is the
// exact state this file is about.
func mkdir(t *testing.T, r *Repo, rel string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(r.Root, rel), 0o755); err != nil {
		t.Fatal(err)
	}
}

// `circle init` writes docs = ["docs/"] on every new project. A bare Stat on
// that directory answered "resolved", so an empty docs folder reported
// `✓ docs/ → 1` — a registry with nothing in it wearing a checkmark, which is
// the failure the drift checker exists to prevent one level up.
func TestKnowledgeCountsFilesNotDirectories(t *testing.T) {
	minimal := "[project]\nname = \"x\"\ncontract_version = 1\n"

	t.Run("an empty directory resolves to zero and warns", func(t *testing.T) {
		r := repoWith(t, minimal+"[knowledge]\ndocs = [\"docs/\"]\n", nil)
		mkdir(t, r, "docs")

		res := Validate(r)
		c, ok := find(res, "knowledge", "docs")
		if !ok {
			t.Fatal("no docs check reported")
		}
		if c.Severity != Warn {
			t.Errorf("severity = %q, want warn — empty is honest, not broken", c.Severity)
		}
		if c.Detail != "docs/ → 0 files" {
			t.Errorf("detail = %q, want a file count of zero", c.Detail)
		}
		// Advisory, not blocking: a young project must still be able to write.
		if res.Blocked() {
			t.Errorf("an empty docs folder must not block; got %+v", res.Checks)
		}
	})

	t.Run("a directory counts the files beneath it, at any depth", func(t *testing.T) {
		r := repoWith(t, minimal+"[knowledge]\ndocs = [\"docs/\"]\n", map[string]string{
			"docs/a.md":        "a",
			"docs/b.md":        "b",
			"docs/adr/0001.md": "c",
			"docs/adr/0002.md": "d",
		})
		res := Validate(r)
		c, _ := find(res, "knowledge", "docs")
		if c.Severity != OK {
			t.Fatalf("severity = %q, want ok", c.Severity)
		}
		if c.Detail != "docs/ → 4" {
			t.Errorf("detail = %q, want all four documents counted", c.Detail)
		}
	})

	t.Run("a path that does not exist is still a failure", func(t *testing.T) {
		// The distinction that matters: absent is a lie and blocks; present but
		// empty is honest and warns.
		r := repoWith(t, minimal+"[knowledge]\ndocs = [\"docs/nope.md\"]\n", nil)
		res := Validate(r)
		c, _ := find(res, "knowledge", "docs")
		if c.Severity != Fail {
			t.Fatalf("severity = %q, want fail", c.Severity)
		}
		if !res.Blocked() {
			t.Error("a registered path that does not exist must block")
		}
	})

	t.Run("a single file resolves to one", func(t *testing.T) {
		r := repoWith(t, minimal+"[knowledge]\ndefinitions = [\"ARCH.md\"]\n",
			map[string]string{"ARCH.md": "x"})
		res := Validate(r)
		c, _ := find(res, "knowledge", "definitions")
		if c.Severity != OK || c.Detail != "ARCH.md → 1" {
			t.Fatalf("check = %+v, want ok with one file", c)
		}
	})

	t.Run("a glob counts its matches", func(t *testing.T) {
		r := repoWith(t, minimal+"[knowledge]\ndocs = [\"docs/*.md\"]\n", map[string]string{
			"docs/a.md": "a", "docs/b.md": "b", "docs/c.txt": "c",
		})
		res := Validate(r)
		c, _ := find(res, "knowledge", "docs")
		if c.Detail != "docs/*.md → 2" {
			t.Errorf("detail = %q, want only the two .md files", c.Detail)
		}
	})
}
