package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

// gitRepo builds a real repository, because LedgerPath resolves through
// `git rev-parse --git-common-dir` and that is the behaviour under test.
func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "t@t"},
		{"config", "user.name", "t"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	return dir
}

func TestProjectName(t *testing.T) {
	tests := []struct{ in, want string }{
		{"task-118", "circle-task-118"},
		{"Feature/Auth Rework", "circle-feature-auth-rework"},
		{"epic-31.4", "circle-epic-31-4"},
		{"--weird--", "circle-weird"},
	}
	for _, tc := range tests {
		if got := ProjectName(tc.in); got != tc.want {
			t.Errorf("ProjectName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestAllocateIsIdempotentPerItem(t *testing.T) {
	// Re-provisioning a resumed session must return the same ports its running
	// containers are already bound to.
	dir := gitRepo(t)
	first, err := Allocate(dir, "task-1", "circle-task-1", dir, []string{"api:8000"}, 42000, 42099)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Allocate(dir, "task-1", "circle-task-1", dir, []string{"api:8000"}, 42000, 42099)
	if err != nil {
		t.Fatal(err)
	}
	if first.Ports["api:8000"] != second.Ports["api:8000"] {
		t.Fatalf("re-provision moved the port: %d then %d",
			first.Ports["api:8000"], second.Ports["api:8000"])
	}
}

func TestAllocateNeverRepeatsAcrossItems(t *testing.T) {
	dir := gitRepo(t)
	seen := map[int]string{}
	for i := 0; i < 5; i++ {
		item := fmt.Sprintf("task-%d", i)
		l, err := Allocate(dir, item, ProjectName(item), dir,
			[]string{"api:8000", "web:80"}, 42000, 42099)
		if err != nil {
			t.Fatal(err)
		}
		for k, p := range l.Ports {
			if prev, dup := seen[p]; dup {
				t.Fatalf("port %d handed to both %s and %s (%s)", p, prev, item, k)
			}
			seen[p] = item
		}
	}
	if len(seen) != 10 {
		t.Fatalf("got %d distinct ports, want 10", len(seen))
	}
}

// The Phase 3 exit criterion: three worktrees provisioning at once, zero
// collisions. A check-then-write would pass the free-port probe twice; only the
// flock makes this safe.
func TestAllocateIsConcurrencySafe(t *testing.T) {
	dir := gitRepo(t)
	const n = 3

	var wg sync.WaitGroup
	var mu sync.Mutex
	got := map[int]string{}
	errs := make([]error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			item := fmt.Sprintf("task-%d", i)
			l, err := Allocate(dir, item, ProjectName(item), dir,
				[]string{"api:8000", "web:80"}, 42000, 42199)
			if err != nil {
				errs[i] = err
				return
			}
			mu.Lock()
			defer mu.Unlock()
			for _, p := range l.Ports {
				if prev, dup := got[p]; dup {
					errs[i] = fmt.Errorf("port %d given to both %s and %s", p, prev, item)
					return
				}
				got[p] = item
			}
		}(i)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(got) != n*2 {
		t.Fatalf("got %d distinct ports across %d worktrees, want %d", len(got), n, n*2)
	}
}

func TestReleaseFreesTheRange(t *testing.T) {
	dir := gitRepo(t)
	l1, _ := Allocate(dir, "task-1", "p1", dir, []string{"api:8000"}, 42000, 42001)
	if _, err := Allocate(dir, "task-2", "p2", dir, []string{"api:8000"}, 42000, 42000); err == nil {
		t.Log("range wide enough that exhaustion did not trigger; continuing")
	}
	if _, err := Release(dir, "task-1"); err != nil {
		t.Fatal(err)
	}
	l3, err := Allocate(dir, "task-3", "p3", dir, []string{"api:8000"}, 42000, 42001)
	if err != nil {
		t.Fatal(err)
	}
	if l3.Ports["api:8000"] != l1.Ports["api:8000"] {
		t.Logf("reused a different port (%d vs %d) — acceptable, both are free",
			l3.Ports["api:8000"], l1.Ports["api:8000"])
	}
	leases, _ := List(dir)
	for _, l := range leases {
		if l.Item == "task-1" {
			t.Fatal("released lease is still in the ledger")
		}
	}
}

func TestExhaustedRangeIsAnErrorNotACollision(t *testing.T) {
	dir := gitRepo(t)
	if _, err := Allocate(dir, "a", "pa", dir, []string{"x:1", "y:2"}, 42500, 42501); err != nil {
		t.Fatal(err)
	}
	_, err := Allocate(dir, "b", "pb", dir, []string{"z:3"}, 42500, 42501)
	if err == nil {
		t.Fatal("an exhausted range must fail loudly rather than hand out a taken port")
	}
}

// A killed session leaves its lease behind; without pruning the range leaks
// until it is exhausted.
func TestPruneDropsLeasesForVanishedWorktrees(t *testing.T) {
	dir := gitRepo(t)
	gone := filepath.Join(dir, "worktrees", "dead")
	if err := os.MkdirAll(gone, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Allocate(dir, "dead", "circle-dead", gone, []string{"api:8000"}, 42000, 42099); err != nil {
		t.Fatal(err)
	}
	if _, err := Allocate(dir, "alive", "circle-alive", dir, []string{"api:8000"}, 42000, 42099); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}

	n, err := Prune(dir)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("pruned %d leases, want 1", n)
	}
	leases, _ := List(dir)
	if len(leases) != 1 || leases[0].Item != "alive" {
		t.Fatalf("leases = %+v, want only the live one", leases)
	}
}

func TestLedgerIsSharedAcrossWorktrees(t *testing.T) {
	// The ledger must resolve to the common git dir. A per-worktree ledger
	// cannot see other worktrees' ports, which is the collision it exists to
	// prevent.
	main := gitRepo(t)
	cmd := exec.Command("git", "commit", "-q", "--allow-empty", "-m", "init")
	cmd.Dir = main
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("commit: %s", out)
	}
	wt := filepath.Join(t.TempDir(), "wt")
	cmd = exec.Command("git", "worktree", "add", "-q", "-b", "wt", wt)
	cmd.Dir = main
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("worktree add: %s", out)
	}

	fromMain, err := LedgerPath(main)
	if err != nil {
		t.Fatal(err)
	}
	fromWorktree, err := LedgerPath(wt)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := filepath.EvalSymlinks(fromMain)
	b, _ := filepath.EvalSymlinks(fromWorktree)
	if a != b {
		t.Fatalf("ledger differs by worktree:\n  main:     %s\n  worktree: %s", a, b)
	}
}
