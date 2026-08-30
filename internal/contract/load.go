// Package contract loads and validates .circle/project.toml.
//
// Everything here is I/O against a repository. The rules themselves live in
// domain; this package only reads the disk and reports what it found.
package contract

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/edwardmontoya/circle/internal/domain"
)

// Dir is the state directory name. Kept as a constant because hooks, the app
// and the CLI must all agree on it.
const Dir = ".circle"

var ErrNoContract = errors.New("no .circle/project.toml")

// Repo is a resolved Circle repository.
type Repo struct {
	Root     string // repository root, resolved from cwd — never CLAUDE_PROJECT_DIR
	Contract domain.Contract
}

// FindRoot walks up from start looking for .circle/, falling back to the git
// toplevel.
//
// Resolution starts from the working directory on purpose. Inside a git
// worktree ${CLAUDE_PROJECT_DIR} still points at the main checkout while cwd
// follows Claude — a hook that trusts the former reads and writes the wrong
// repository during exactly the parallel work worktrees exist to enable.
func FindRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if st, err := os.Stat(filepath.Join(dir, Dir)); err == nil && st.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	// Not scaffolded yet: fall back to the git toplevel so `circle init` has
	// somewhere sensible to write.
	cmd := exec.Command("git", "-C", start, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", ErrNoContract
	}
	return strings.TrimSpace(string(out)), nil
}

// Load reads and parses the contract at root.
func Load(root string) (*Repo, error) {
	path := filepath.Join(root, Dir, "project.toml")
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoContract
		}
		return nil, err
	}
	var c domain.Contract
	if _, err := toml.Decode(string(b), &c); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &Repo{Root: root, Contract: c}, nil
}

// Open resolves the root from a directory and loads the contract.
func Open(cwd string) (*Repo, error) {
	root, err := FindRoot(cwd)
	if err != nil {
		return nil, err
	}
	return Load(root)
}

// Path joins a repo-relative path.
func (r *Repo) Path(parts ...string) string {
	return filepath.Join(append([]string{r.Root}, parts...)...)
}

// StateDir is .circle/, which is committed.
func (r *Repo) StateDir(parts ...string) string {
	return r.Path(append([]string{Dir}, parts...)...)
}

// RuntimeDir is .circle/runtime/, which is gitignored and regenerable. The
// volatile/durable split is what makes "share via repo" honest.
func (r *Repo) RuntimeDir(parts ...string) string {
	return r.StateDir(append([]string{"runtime"}, parts...)...)
}
