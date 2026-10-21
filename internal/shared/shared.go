// Package shared holds state every worktree of a repository must agree on.
//
// Each worktree has its own .circle/, so anything written there is invisible to
// the others — fine for durable state that travels by merge, wrong for state
// that coordinates live work: who holds which task, which session works on
// which item. That state lives beside the *common* git directory, the one path
// every worktree of a repository resolves to the same place. The port ledger
// established the pattern (internal/worktree/ports.go); this package is it,
// generalised.
package shared

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Dir returns <git-common-dir>/circle for the repository containing start.
//
// Outside git there are no worktrees to coordinate, so the fallback is the
// repository's own runtime directory — correct for the one checkout there is.
func Dir(start string) string {
	cmd := exec.Command("git", "rev-parse", "--git-common-dir")
	cmd.Dir = start
	out, err := cmd.Output()
	if err != nil {
		return filepath.Join(start, ".circle", "runtime", "shared")
	}
	common := strings.TrimSpace(string(out))
	if !filepath.IsAbs(common) {
		common = filepath.Join(start, common)
	}
	return filepath.Join(common, "circle")
}

// Read decodes a JSON file into v. A missing file leaves v untouched and is not
// an error: an empty ledger is the normal starting state.
func Read(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(b, v)
}

// Write encodes v to path atomically: write, then rename, so a crash cannot
// leave a truncated file that silently drops every record at once.
func Write(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Update reads path into v, calls fn, and writes v back — all under an
// exclusive flock.
//
// Parallel agents are the designed case, so this is a real lock rather than a
// check-then-write: without it two worktrees can both read "unclaimed" and both
// take the same task.
func Update(path string, v any, fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	lockPath := path + ".lock"
	lf, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer lf.Close()

	deadline := time.Now().Add(10 * time.Second)
	for {
		err = syscall.Flock(int(lf.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for the lock at %s", lockPath)
		}
		time.Sleep(50 * time.Millisecond)
	}
	defer func() { _ = syscall.Flock(int(lf.Fd()), syscall.LOCK_UN) }()

	if err := Read(path, v); err != nil {
		return err
	}
	if err := fn(); err != nil {
		return err
	}
	return Write(path, v)
}
