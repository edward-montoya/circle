// Package session decides which item a Claude Code session is working on.
//
// Every enforcing path used to read the item "default", which made two items in
// one repository impossible: they shared a plan, so changing one voided the
// other's approval, and a session on one could write into the other. A session
// now resolves to exactly one item, and the gate enforces that item's approval.
package session

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/shared"
	"github.com/edwardmontoya/circle/internal/task"
)

// Binding ties a session to an item, or to an initiative decision.
type Binding struct {
	Session  string    `json:"session"`
	Item     string    `json:"item,omitempty"`
	Decision string    `json:"decision,omitempty"`
	Worktree string    `json:"worktree,omitempty"`
	At       time.Time `json:"at"`
}

// Source says how an item was resolved, so `circle session current` can show
// why — a binding the user forgot about is otherwise invisible.
type Source string

const (
	FromFlag     Source = "flag"
	FromBinding  Source = "binding"
	FromWorktree Source = "worktree"
	FromEnv      Source = "env"
	FromDefault  Source = "default"
)

// EnvSession carries the session id into the agent's Bash commands. The
// SessionStart hook writes it to $CLAUDE_ENV_FILE, which Claude Code sources
// before every command it runs.
const EnvSession = "CIRCLE_SESSION"

// EnvItem pins an item for a whole shell, below a binding.
const EnvItem = "CIRCLE_ITEM"

func path(dir, sid string) string {
	return filepath.Join(shared.Dir(dir), "sessions", sanitize(sid)+".json")
}

// sanitize keeps a session id from escaping the sessions directory.
func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		}
		return '_'
	}, s)
}

// ID is this process's session id: the hook payload's when there is one,
// otherwise the one the SessionStart hook exported.
func ID(fromPayload string) string {
	if fromPayload != "" {
		return fromPayload
	}
	return os.Getenv(EnvSession)
}

// Bind records a binding. It lives in the shared git directory so the same
// session resolves the same item from any worktree it touches.
func Bind(dir string, b Binding) error {
	b.At = time.Now().UTC()
	return shared.Write(path(dir, b.Session), b)
}

// Lookup reads a session's binding, if any.
func Lookup(dir, sid string) (Binding, bool) {
	if sid == "" {
		return Binding{}, false
	}
	var b Binding
	if err := shared.Read(path(dir, sid), &b); err != nil || b.Session == "" {
		return Binding{}, false
	}
	return b, true
}

// Resolve finds the item a session works on. First match wins:
//
//  1. an explicit --item flag
//  2. the session's binding (`circle session bind`)
//  3. the worktree's name — `claude --worktree feat-1` works on item feat-1,
//     and a worktree named after a task works on that task's item
//  4. $CIRCLE_ITEM
//  5. the default item, so a repository with one item behaves as it always has
func Resolve(r *contract.Repo, flag, sid string) (string, Source) {
	if flag != "" {
		return flag, FromFlag
	}
	if b, ok := Lookup(r.Root, sid); ok && b.Item != "" {
		return b.Item, FromBinding
	}
	if name := WorktreeName(r.Root); name != "" {
		if Exists(r, name) {
			return name, FromWorktree
		}
		if t, err := task.Get(r, name); err == nil {
			return task.ItemOf(t), FromWorktree
		}
	}
	if v := os.Getenv(EnvItem); v != "" {
		return v, FromEnv
	}
	return task.DefaultItem, FromDefault
}

// Exists reports whether an item has been created.
func Exists(r *contract.Repo, item string) bool {
	st, err := os.Stat(r.StateDir("items", item))
	return err == nil && st.IsDir()
}

// WorktreeName is the name of the linked worktree containing dir, or "" in the
// main checkout. Git keeps each linked worktree's private state under
// <common>/worktrees/<name>, so that directory's base name is the name Claude
// Code gave it.
func WorktreeName(dir string) string {
	gitDir, err1 := gitPath(dir, "--git-dir")
	common, err2 := gitPath(dir, "--git-common-dir")
	if err1 != nil || err2 != nil || gitDir == common {
		return ""
	}
	return filepath.Base(gitDir)
}

func gitPath(dir, flag string) (string, error) {
	cmd := exec.Command("git", "rev-parse", flag)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	p := strings.TrimSpace(string(out))
	if !filepath.IsAbs(p) {
		p = filepath.Join(dir, p)
	}
	return filepath.Clean(p), nil
}
