// Package worktree adds the one thing Claude Code's native worktrees do not:
// Compose isolation (D-23).
//
// Claude Code already creates the worktree, copies gitignored files via
// .worktreeinclude, locks it while an agent runs, and cleans it up. Rebuilding
// any of that would be a second implementation that drifts. Circle contributes
// COMPOSE_PROJECT_NAME namespacing and a host-port allocation, and nothing else.
package worktree

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

// Lease is one item's claim on a set of host ports.
type Lease struct {
	Item     string         `json:"item"`
	Project  string         `json:"project"` // COMPOSE_PROJECT_NAME
	Worktree string         `json:"worktree"`
	Ports    map[string]int `json:"ports"` // "service:containerPort" -> host port
	At       time.Time      `json:"at"`
}

// Ledger is every live lease across every worktree of a repository.
type Ledger struct {
	Leases []Lease `json:"leases"`
}

// LedgerPath returns the shared allocation file.
//
// It lives beside the *common* git directory, not in .circle/runtime/. Each
// worktree has its own .circle/, so a per-worktree ledger could not see the
// other worktrees' ports — which is precisely the collision it exists to
// prevent. `git rev-parse --git-common-dir` is the one path every worktree of a
// repository agrees on.
func LedgerPath(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--git-common-dir")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("resolving the shared git dir: %w", err)
	}
	common := strings.TrimSpace(string(out))
	if !filepath.IsAbs(common) {
		common = filepath.Join(dir, common)
	}
	return filepath.Join(common, "circle-ports.json"), nil
}

// withLock runs fn while holding an exclusive lock on the ledger.
//
// Three agents provisioning at once is the designed case, so this is a real
// flock rather than a check-then-write. Without it two worktrees can pass the
// free-port probe simultaneously and both claim the same port.
func withLock(path string, fn func(*Ledger) error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	lockPath := path + ".lock"
	lf, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer lf.Close()

	// Block rather than fail: a provision that loses a race should wait its
	// turn, not abort the worktree creation.
	deadline := time.Now().Add(10 * time.Second)
	for {
		err = syscall.Flock(int(lf.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for the port ledger lock at %s", lockPath)
		}
		time.Sleep(50 * time.Millisecond)
	}
	defer func() { _ = syscall.Flock(int(lf.Fd()), syscall.LOCK_UN) }()

	var l Ledger
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &l)
	}
	if err := fn(&l); err != nil {
		return err
	}
	b, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	// Write-then-rename so a crash cannot leave a truncated ledger, which would
	// silently free every port at once.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// free reports whether a host port can actually be bound right now.
//
// The ledger is not enough: something outside Circle — the main checkout's own
// stack, another project, a stray container — may hold the port. Both checks
// are needed, and neither alone is sufficient.
func free(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

// Allocate claims a host port per key, from lo..hi, for one item.
//
// Re-provisioning the same item returns its existing lease unchanged, so a
// resumed session keeps the ports its running containers are already bound to.
func Allocate(dir, item, project, worktree string, keys []string, lo, hi int) (Lease, error) {
	path, err := LedgerPath(dir)
	if err != nil {
		return Lease{}, err
	}
	var lease Lease

	err = withLock(path, func(l *Ledger) error {
		for _, existing := range l.Leases {
			if existing.Item == item {
				lease = existing
				return nil
			}
		}

		taken := map[int]bool{}
		for _, ex := range l.Leases {
			for _, p := range ex.Ports {
				taken[p] = true
			}
		}

		lease = Lease{
			Item: item, Project: project, Worktree: worktree,
			Ports: map[string]int{}, At: time.Now().UTC(),
		}
		next := lo
		sort.Strings(keys)
		for _, k := range keys {
			for {
				if next > hi {
					return fmt.Errorf("port range %d-%d exhausted: %d already leased across worktrees",
						lo, hi, len(taken))
				}
				if !taken[next] && free(next) {
					break
				}
				next++
			}
			lease.Ports[k] = next
			taken[next] = true
			next++
		}
		l.Leases = append(l.Leases, lease)
		return nil
	})
	return lease, err
}

// Release drops an item's lease.
func Release(dir, item string) (Lease, error) {
	path, err := LedgerPath(dir)
	if err != nil {
		return Lease{}, err
	}
	var released Lease
	err = withLock(path, func(l *Ledger) error {
		var keep []Lease
		for _, ex := range l.Leases {
			if ex.Item == item {
				released = ex
				continue
			}
			keep = append(keep, ex)
		}
		l.Leases = keep
		return nil
	})
	return released, err
}

// List returns every live lease.
func List(dir string) ([]Lease, error) {
	path, err := LedgerPath(dir)
	if err != nil {
		return nil, err
	}
	var out []Lease
	err = withLock(path, func(l *Ledger) error {
		out = l.Leases
		return nil
	})
	return out, err
}

// Prune drops leases whose worktree directory no longer exists.
//
// A killed session leaves its lease behind, and without this the range leaks
// until it is exhausted. Called on provision so it is self-healing.
func Prune(dir string) (int, error) {
	path, err := LedgerPath(dir)
	if err != nil {
		return 0, err
	}
	removed := 0
	err = withLock(path, func(l *Ledger) error {
		var keep []Lease
		for _, ex := range l.Leases {
			if ex.Worktree != "" {
				if _, err := os.Stat(ex.Worktree); os.IsNotExist(err) {
					removed++
					continue
				}
			}
			keep = append(keep, ex)
		}
		l.Leases = keep
		return nil
	})
	return removed, err
}
