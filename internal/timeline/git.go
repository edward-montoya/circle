// Package timeline builds the change record from git, never from the agent.
//
// Claude Code has a documented failure mode where it reports commits that never
// persisted (anthropics/claude-code#44035). A timeline built by asking the agent
// what it did inherits that bug; one built from `git rev-list` catches it.
//
// Shelling out to git rather than using a library is deliberate: the timeline
// needs patch-id, reflog and worktree semantics identical to the user's own git,
// and a reimplementation that drifts is worse than a subprocess.
package timeline

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/edwardmontoya/circle/internal/domain"
)

// gitRaw runs a command in dir and returns stdout untouched.
//
// Untouched matters: `git status --porcelain` is column-oriented, and an
// unstaged modification is reported as " M path" with a leading space. Trimming
// the output shifts that line left by one, so every path loses its first
// character — which renders as a plausible-looking path rather than an error.
// A parser test cannot catch this, because the corruption happens before the
// parser ever sees the bytes.
func gitRaw(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s: %s", strings.Join(args, " "),
				strings.TrimSpace(string(ee.Stderr)))
		}
		return "", err
	}
	return string(out), nil
}

// git is gitRaw with the trailing newline removed, for the many commands whose
// output is a single value.
func git(dir string, args ...string) (string, error) {
	out, err := gitRaw(dir, args...)
	return strings.TrimSpace(out), err
}

// HeadSHA is the baseline recorded when a session or worktree opens.
func HeadSHA(dir string) (string, error) { return git(dir, "rev-parse", "HEAD") }

// Branch names the current branch.
func Branch(dir string) (string, error) {
	return git(dir, "rev-parse", "--abbrev-ref", "HEAD")
}

// Exists reports whether a commit is still reachable. A recorded SHA that fails
// this was amended, rebased or squashed away.
func Exists(dir, sha string) bool {
	_, err := git(dir, "cat-file", "-e", sha+"^{commit}")
	return err == nil
}

// Commits lists everything on HEAD since baseline, oldest first.
//
// This is the truth set. It finds commits regardless of who made them or how —
// including ones the human made in another terminal, which is precisely what
// interception at the tool layer would miss.
func Commits(dir, baseline string) ([]domain.Entry, error) {
	const sep = "\x1f"
	format := strings.Join([]string{"%H", "%h", "%s", "%an", "%aI"}, sep)
	out, err := git(dir, "rev-list", "--reverse", "--no-merges",
		"--pretty=format:"+format, baseline+"..HEAD")
	if err != nil {
		return nil, err
	}
	var entries []domain.Entry
	seq := 0
	for _, line := range strings.Split(out, "\n") {
		// rev-list --pretty emits a "commit <sha>" header line before each entry.
		if line == "" || strings.HasPrefix(line, "commit ") {
			continue
		}
		parts := strings.Split(line, sep)
		if len(parts) < 5 {
			continue
		}
		at, _ := time.Parse(time.RFC3339, parts[4])
		seq++
		entries = append(entries, domain.Entry{
			Seq: seq, SHA: parts[0], Short: parts[1], Subject: parts[2],
			Author: parts[3], At: at, State: domain.EntryLive,
		})
	}
	return entries, nil
}

// FilesOf returns the paths one commit touched, with rename detection.
// No diff content — the requirement was commit number and file list.
func FilesOf(dir, sha string) ([]domain.FileChange, error) {
	out, err := git(dir, "diff-tree", "--no-commit-id", "--name-status", "-r", "-M", sha)
	if err != nil {
		return nil, err
	}
	var files []domain.FileChange
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < 2 {
			continue
		}
		status := domain.ChangeStatus(cols[0][:1])
		fc := domain.FileChange{Status: status, Path: cols[len(cols)-1]}
		if status == domain.Renamed && len(cols) >= 3 {
			fc.From, fc.Path = cols[1], cols[2]
		}
		files = append(files, fc)
	}
	return files, nil
}

// Uncommitted is the working-tree tail.
//
// Most sessions end here. A timeline that omits it lies by omission at exactly
// the moment someone is resuming cold.
func Uncommitted(dir string) ([]domain.FileChange, error) {
	out, err := gitRaw(dir, "status", "--porcelain") // raw: columns are significant
	if err != nil {
		return nil, err
	}
	return ParsePorcelain(out), nil
}

// ParsePorcelain turns `git status --porcelain` output into file changes.
//
// Exported for testing: the format is fixed-width (two status columns, a space,
// then the path) and getting the offset wrong silently truncates the first
// character of every path, which looks like a rendering glitch rather than a
// parse bug.
func ParsePorcelain(out string) []domain.FileChange {
	var files []domain.FileChange
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		code := line[:2]
		path := strings.TrimSpace(line[3:])
		// A rename reads `R  old -> new`; the new path is what was touched.
		if idx := strings.Index(path, " -> "); idx >= 0 {
			path = path[idx+4:]
		}
		path = strings.Trim(path, `"`)

		status := domain.Modified
		switch {
		case code == "??", strings.ContainsRune(code, 'A'):
			status = domain.Added
		case strings.ContainsRune(code, 'D'):
			status = domain.Deleted
		case strings.ContainsRune(code, 'R'):
			status = domain.Renamed
		}
		files = append(files, domain.FileChange{Status: status, Path: path})
	}
	return files
}

// PatchID is the content identity of a commit, stable across amend and rebase.
// It is the first and best way to find where a rewritten commit went.
func PatchID(dir, sha string) (string, error) {
	show := exec.Command("git", "show", sha)
	show.Dir = dir
	pid := exec.Command("git", "patch-id", "--stable")
	pid.Dir = dir
	pipe, err := show.StdoutPipe()
	if err != nil {
		return "", err
	}
	pid.Stdin = pipe
	if err := show.Start(); err != nil {
		return "", err
	}
	out, err := pid.Output()
	_ = show.Wait()
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return "", fmt.Errorf("no patch-id for %s", sha)
	}
	return fields[0], nil
}

// ResolveRewrite finds where a vanished commit went.
//
// Order matters: patch-id is content-identity and survives amend and rebase;
// subject plus author-date is the fallback for a squash, which changes content.
// A commit that cannot be resolved is marked rewritten rather than deleted, so
// the timeline does not empty itself the moment a PR lands (D-33).
func ResolveRewrite(dir string, e domain.Entry, candidates []domain.Entry) string {
	if want, err := PatchID(dir, e.SHA); err == nil && want != "" {
		for _, c := range candidates {
			if got, err := PatchID(dir, c.SHA); err == nil && got == want {
				return c.SHA
			}
		}
	}
	for _, c := range candidates {
		if c.Subject == e.Subject && c.At.Equal(e.At) {
			return c.SHA
		}
	}
	return ""
}
