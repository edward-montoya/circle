package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func commitFile(t *testing.T, root, rel, body, msg string) {
	t.Helper()
	p := filepath.Join(root, rel)
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", msg)
}

func findings(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// buildAlpha takes item alpha from plan to reviewing, with one commit inside
// its radius and a DOMAIN.md that covers the concept it adds.
func buildAlpha(t *testing.T) string {
	t.Helper()
	root := newGitRepo(t)
	planItem(t, root, "alpha", "src/a/**")
	approve(t, root, "alpha")
	mustRun(t, root, "timeline", "open", "--item", "alpha")
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", "plan alpha")
	commitFile(t, root, "src/a/DOMAIN.md", "# A\n\n## Language\n\n**Widget**:\nA thing.\n", "alpha-1: domain")
	commitFile(t, root, "src/a/widget.go", "package a\n\ntype Widget struct{}\n", "alpha-1: add widget")
	mustRun(t, root, "task", "claim", "alpha-1")
	mustRun(t, root, "quality", "run")
	mustRun(t, root, "task", "close", "alpha-1")
	return root
}

func TestReviewCloseMarksTheItemDone(t *testing.T) {
	root := buildAlpha(t)
	tmp := t.TempDir()
	if code, out := run(t, root, "review", "close", "--item", "alpha"); code != ExitGate || !strings.Contains(out, "spec axis is not recorded") {
		t.Fatalf("close without judged axes: %d\n%s", code, out)
	}
	mustRun(t, root, "review", "record", "--item", "alpha", "--axis", "spec",
		"--file", findings(t, tmp, "spec.md", "- [blocking] behaviour 2 is missing\n"))
	mustRun(t, root, "review", "record", "--item", "alpha", "--axis", "standards",
		"--file", findings(t, tmp, "std.md", "- [nit] rename x\n"))
	if code, out := run(t, root, "review", "close", "--item", "alpha"); code != ExitGate || !strings.Contains(out, "1 blocking") {
		t.Fatalf("a blocking spec finding must hold the item: %d\n%s", code, out)
	}
	mustRun(t, root, "review", "record", "--item", "alpha", "--axis", "spec",
		"--file", findings(t, tmp, "spec2.md", "No findings.\n"))
	mustRun(t, root, "review", "close", "--item", "alpha")
	if out := mustRun(t, root, "item", "show", "alpha"); !strings.Contains(out, "done") {
		t.Fatalf("closed review should make the item done:\n%s", out)
	}
	if out := mustRun(t, root, "review", "drift", "--item", "alpha"); !strings.Contains(out, "3 commit(s)") ||
		strings.Contains(out, "names no task") {
		t.Fatalf("three commits since the baseline, the plan commit being bookkeeping:\n%s", out)
	}
}

func TestReviewTraceBlocksOnRadiusDriftAndDomainGaps(t *testing.T) {
	root := buildAlpha(t)
	commitFile(t, root, "src/b/other.go", "package b\n\ntype Gadget struct{}\n", "sneak in a change")
	out := mustRun(t, root, "review", "drift", "--item", "alpha")
	for _, want := range []string{"src/b/other.go is outside the approved radius", "commit names no task", "Gadget"} {
		if !strings.Contains(out, want) {
			t.Errorf("trace missing %q:\n%s", want, out)
		}
	}
	tmp := t.TempDir()
	mustRun(t, root, "review", "record", "--item", "alpha", "--axis", "spec", "--file", findings(t, tmp, "s.md", "ok\n"))
	mustRun(t, root, "review", "record", "--item", "alpha", "--axis", "standards", "--file", findings(t, tmp, "t.md", "ok\n"))
	if code, out := run(t, root, "review", "close", "--item", "alpha"); code != ExitGate || !strings.Contains(out, "trace axis") {
		t.Fatalf("drift must block close: %d\n%s", code, out)
	}
}

// A review of an older commit says nothing about what landed after it.
func TestReviewCloseRefusesAStaleAxis(t *testing.T) {
	root := buildAlpha(t)
	tmp := t.TempDir()
	mustRun(t, root, "review", "record", "--item", "alpha", "--axis", "spec", "--file", findings(t, tmp, "s.md", "ok\n"))
	mustRun(t, root, "review", "record", "--item", "alpha", "--axis", "standards", "--file", findings(t, tmp, "t.md", "ok\n"))
	commitFile(t, root, "src/a/more.go", "package a\n", "alpha-1: one more change")
	mustRun(t, root, "quality", "run")
	if code, out := run(t, root, "review", "close", "--item", "alpha"); code != ExitGate || !strings.Contains(out, "not HEAD") {
		t.Fatalf("an axis recorded before the last commit must not close the item: %d\n%s", code, out)
	}
	if out := mustRun(t, root, "review", "show", "--item", "alpha"); !strings.Contains(out, "STALE") {
		t.Fatalf("review show must flag the stale axis:\n%s", out)
	}
}

// An item whose timeline began before another item was merged in must not
// count that item's commits and files as its own drift.
func TestTraceIgnoresAnotherItemsMergedWork(t *testing.T) {
	root := buildAlpha(t)
	planItem(t, root, "beta", "src/b/**")
	approve(t, root, "beta")
	mustRun(t, root, "timeline", "open", "--item", "beta")
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", "plan beta")
	commitFile(t, root, "src/a/merged.go", "package a\n", "alpha-1: work that arrived by merge")
	commitFile(t, root, "src/b/b.go", "package b\n", "beta-1: beta's own work")
	out := mustRun(t, root, "review", "drift", "--item", "beta")
	if strings.Contains(out, "src/a/merged.go") || strings.Contains(out, "names no task") {
		t.Fatalf("alpha's commit was counted against beta:\n%s", out)
	}
	if !strings.Contains(out, "1 commit(s)") && !strings.Contains(out, "2 commit(s)") {
		t.Fatalf("beta's own commits should be counted:\n%s", out)
	}
}
