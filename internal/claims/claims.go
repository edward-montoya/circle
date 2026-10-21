// Package claims records who holds which task right now.
//
// A claim is live coordination, not history. It used to be a line in
// tasks.jsonl, which sits in each worktree's own .circle/ — so two implementers
// in two worktrees each saw their own copy, both read "unclaimed", and both took
// the same task. Claims now live in the shared git directory under a lock, where
// every worktree of the repository sees the same ledger. They are not
// committed: who is working on what this minute does not belong in history, and
// a closure — which does — is still written to the task file.
package claims

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/edwardmontoya/circle/internal/shared"
)

// Claim is one held task.
type Claim struct {
	Task     string    `json:"task"`
	Item     string    `json:"item"`
	Holder   string    `json:"holder"`
	Session  string    `json:"session,omitempty"`
	Worktree string    `json:"worktree,omitempty"`
	At       time.Time `json:"at"`
}

type ledger struct {
	Claims []Claim `json:"claims"`
}

// Path is the ledger for the repository containing dir.
func Path(dir string) string { return filepath.Join(shared.Dir(dir), "claims.json") }

// HeldError is returned when someone else holds the task.
type HeldError struct{ Claim Claim }

func (e HeldError) Error() string {
	where := e.Claim.Holder
	if e.Claim.Worktree != "" {
		where += " in " + e.Claim.Worktree
	}
	return fmt.Sprintf("%s is already claimed by %s since %s",
		e.Claim.Task, where, e.Claim.At.Format(time.RFC3339))
}

// same reports whether two claims come from one holder. A session id is the
// sharpest identity; without one, the worktree is.
func same(a, b Claim) bool {
	if a.Session != "" && b.Session != "" {
		return a.Session == b.Session
	}
	return a.Holder == b.Holder && a.Worktree == b.Worktree
}

// Take claims a task. Taking a task you already hold is a no-op; taking one
// someone else holds fails with HeldError unless steal is set.
func Take(dir string, c Claim, steal bool) error {
	var l ledger
	return shared.Update(Path(dir), &l, func() error {
		for i, have := range l.Claims {
			if have.Task != c.Task {
				continue
			}
			if !same(have, c) && !steal {
				return HeldError{have}
			}
			l.Claims[i] = c
			return nil
		}
		l.Claims = append(l.Claims, c)
		return nil
	})
}

// Release drops the claim on a task, whoever holds it. Closing a task releases
// it; so does an explicit `circle task release`.
func Release(dir, taskID string) error {
	var l ledger
	return shared.Update(Path(dir), &l, func() error {
		kept := l.Claims[:0]
		for _, c := range l.Claims {
			if c.Task != taskID {
				kept = append(kept, c)
			}
		}
		l.Claims = kept
		return nil
	})
}

// All reads the ledger without locking. A reader racing a writer sees either
// the old ledger or the new one, never a torn one, because writes rename.
func All(dir string) map[string]Claim {
	var l ledger
	_ = shared.Read(Path(dir), &l)
	out := make(map[string]Claim, len(l.Claims))
	for _, c := range l.Claims {
		out[c.Task] = c
	}
	return out
}
