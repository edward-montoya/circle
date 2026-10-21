package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/edwardmontoya/circle/internal/claims"
	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/domain"
	"github.com/edwardmontoya/circle/internal/item"
	"github.com/edwardmontoya/circle/internal/task"
	"github.com/edwardmontoya/circle/internal/timeline"
)

func init() {
	register(Command{"handoff write", "Write the item's handoff skeleton from git, tasks and state", runHandoffWrite})
}

// runHandoffWrite generates the part of a handoff git already knows — what
// landed, what is uncommitted, what is phantom, which tasks are open and held —
// and leaves marked gaps for what only the session knows: why, and what next.
// The agent fills the gaps; it never rewrites the generated part from memory.
func runHandoffWrite(e Env, args []string) int {
	f := fs("handoff write", e)
	flag := f.String("item", "", "item id; defaults to this session's")
	focus := f.String("for", "", "what the next session is for")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	id, _ := currentItem(repo, *flag)
	body, err := renderHandoff(repo, id, *focus)
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitExternal
	}
	path := filepath.Join(item.Dir(repo, id), "handoff.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	fmt.Fprint(e.Stdout, body)
	rel, _ := filepath.Rel(repo.Root, path)
	fmt.Fprintf(e.Stdout, "\n(written to %s — fill every TODO, change nothing above them)\n", rel)
	return ExitOK
}

func renderHandoff(repo *contract.Repo, id, focus string) (string, error) {
	tl, err := timeline.Sync(repo, id, true)
	if err != nil {
		return "", err
	}
	it, _ := item.Load(repo, id)
	st := item.Derive(repo, id)
	var b strings.Builder
	fmt.Fprintf(&b, "# Handoff · %s\n\n", it.Title)
	fmt.Fprintf(&b, "- **Item:** `%s` — %s (%s)\n", id, st.State, st.Reason)
	if it.Initiative != "" {
		fmt.Fprintf(&b, "- **Initiative:** `%s` — `circle initiative show %s`\n", it.Initiative, it.Initiative)
	}
	fmt.Fprintf(&b, "- **Branch:** %s · baseline `%s`\n", strings.TrimPrefix(tl.Branch, "refs/heads/"), shortSHA(tl.Baseline))
	fmt.Fprintf(&b, "- **Written:** %s\n", time.Now().UTC().Format("2006-01-02 15:04 UTC"))
	if focus != "" {
		fmt.Fprintf(&b, "- **Next session is for:** %s\n", focus)
	}

	b.WriteString("\n## What landed (from git)\n\n")
	live, phantom := 0, 0
	for _, en := range tl.Entries {
		switch en.State {
		case domain.EntryLive:
			fmt.Fprintf(&b, "- `%s` %s\n", en.Short, en.Subject)
			live++
		case domain.EntryPhantom:
			fmt.Fprintf(&b, "- ⚠ **phantom** `%s` %s — reported, but git does not have it\n", en.Short, en.Subject)
			phantom++
		}
	}
	if live == 0 && phantom == 0 {
		b.WriteString("_no commits since the baseline_\n")
	}
	unc, _ := timeline.Uncommitted(repo.Root)
	if len(unc) > 0 {
		b.WriteString("\n**Uncommitted** — most sessions end here; say what each is:\n\n")
		for _, fc := range unc {
			fmt.Fprintf(&b, "- `%s` %s\n", fc.Status, fc.Path)
		}
	}

	b.WriteString("\n## Tasks\n\n")
	ts, _ := task.LoadItem(repo, id)
	held := claims.All(repo.Root)
	for _, t := range ts {
		state := string(t.State)
		if c, ok := held[t.ID]; ok && t.State != domain.StateClosed {
			state = "held by " + c.Holder
		}
		fmt.Fprintf(&b, "- `%s` %s — %s · gate `%s`\n", t.ID, t.Title, state, t.Verify.Gate)
	}
	if len(ts) == 0 {
		b.WriteString("_no tasks yet_\n")
	}
	all, _ := task.Load(repo)
	if ready := readyTasks(repo, all, id, false); len(ready) > 0 {
		fmt.Fprintf(&b, "\nFirst ready: `%s` — %s\n", ready[0].ID, ready[0].Goal.Statement)
	}

	b.WriteString("\n## Why\n\nTODO: decisions made this session that no spec, ADR or commit holds yet. Point, do not restate. If one deserves an ADR, say so here.\n")
	b.WriteString("\n## Next\n\nTODO: one line — the task or decision id and its goal.\n")
	b.WriteString("\n## Suggested skills\n\nTODO: which /circle: skills the next session runs, in order. `circle next` is the starting point.\n")
	return b.String(), nil
}
