package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/domain"
	"github.com/edwardmontoya/circle/internal/domaindoc"
	"github.com/edwardmontoya/circle/internal/events"
	"github.com/edwardmontoya/circle/internal/initiative"
	"github.com/edwardmontoya/circle/internal/session"
	"github.com/edwardmontoya/circle/internal/task"
	"github.com/edwardmontoya/circle/internal/timeline"
)

func init() {
	register(Command{"session start", "SessionStart hook target: export the session id, open the item's timeline", runSessionStart})
	register(Command{"session bind", "Bind this session to one item or decision", runSessionBind})
	register(Command{"session current", "The item this session works on, and why", runSessionCurrent})
}

// sessionPayload is the subset of the SessionStart hook input we read.
type sessionPayload struct {
	SessionID string `json:"session_id"`
	Cwd       string `json:"cwd"`
	Source    string `json:"source"`
}

// runSessionStart runs on every session start. It never fails the session:
// a hook error here would stop Claude Code from opening at all.
func runSessionStart(e Env, args []string) int {
	f := fs("session start", e)
	hook := f.Bool("hook", false, "read a SessionStart payload from stdin")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	var p sessionPayload
	if *hook {
		b, _ := io.ReadAll(e.Stdin)
		_ = json.Unmarshal(b, &p)
	}
	cwd := e.Cwd
	if p.Cwd != "" {
		cwd = p.Cwd
	}
	repo, err := contract.Open(cwd)
	if err != nil {
		return ExitOK // not a Circle repository: nothing to set up
	}
	sid := session.ID(p.SessionID)

	// Claude Code sources $CLAUDE_ENV_FILE before every Bash command, so this is
	// how the session id reaches the `circle` commands the agent runs — the
	// only way they can find this session's binding.
	if envFile := os.Getenv("CLAUDE_ENV_FILE"); envFile != "" && sid != "" {
		if fh, err := os.OpenFile(envFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			fmt.Fprintf(fh, "export %s=%s\n", session.EnvSession, sid)
			// Machine-local variables reach the agent's own commands too, not
			// only the gates: `docker compose up` typed by the agent needs the
			// same proxy CA directory the bootstrap does.
			for k, v := range localEnv(repo) {
				if os.Getenv(k) == "" {
					fmt.Fprintf(fh, "export %s=%q\n", k, v)
				}
			}
			fh.Close()
		}
	}

	id, src := session.Resolve(repo, "", sid)

	// Edits made outside Claude Code — a pull, a hand edit — are caught here.
	_, _, _ = domaindoc.Sync(repo.Root)

	// One timeline per item, opened once. Reopening on every session would move
	// the baseline and drop the commits of every earlier session on the item.
	if t, err := timeline.Load(repo, id); err == nil && t.Baseline == "" {
		_, _ = timeline.Open(repo, id)
	}

	ev := domain.NewEvent(domain.EvSessionOpen)
	ev.Session, ev.Item, ev.Reason = sid, id, string(src)
	_ = events.Append(repo, ev)

	// SessionStart stdout becomes context for the model.
	if id == task.DefaultItem && src == session.FromDefault {
		fmt.Fprintln(e.Stdout, "circle: this session is not bound to an item. `circle item list` shows the open ones; `/circle:start` binds one.")
	} else {
		fmt.Fprintf(e.Stdout, "circle: this session works on item %s (from %s).\n", id, src)
	}
	return ExitOK
}

func runSessionBind(e Env, args []string) int {
	f := fs("session bind", e)
	itemID := f.String("item", "", "the item this session works on")
	decision := f.String("decision", "", "or the initiative decision it works on")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	if (*itemID == "") == (*decision == "") {
		fmt.Fprintln(e.Stderr, "usage: circle session bind --item <id> | --decision <id>")
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	return bindSession(e, repo, *itemID, *decision)
}

func bindSession(e Env, repo *contract.Repo, itemID, decision string) int {
	sid := session.ID("")
	if sid == "" {
		fmt.Fprintf(e.Stderr, "circle: no session id — $%s is unset.\n", session.EnvSession)
		fmt.Fprintln(e.Stderr, "Inside Claude Code the SessionStart hook sets it; is the circle plugin enabled?")
		fmt.Fprintf(e.Stderr, "Outside it, export %s=%s for this shell instead.\n", session.EnvItem, itemID)
		return ExitPrecond
	}
	if decision != "" {
		if _, err := initiative.GetDecision(repo, decision); err != nil {
			fmt.Fprintf(e.Stderr, "circle: %v — `circle decision frontier` lists the open ones\n", err)
			return ExitPrecond
		}
	}
	if itemID != "" && itemID != task.DefaultItem && !session.Exists(repo, itemID) {
		fmt.Fprintf(e.Stderr, "circle: no item %q — `circle item new %s --title \"...\"`\n", itemID, itemID)
		return ExitPrecond
	}
	b := session.Binding{Session: sid, Item: itemID, Decision: decision, Worktree: session.WorktreeName(repo.Root)}
	if err := session.Bind(repo.Root, b); err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	if itemID != "" {
		if t, err := timeline.Load(repo, itemID); err == nil && t.Baseline == "" {
			_, _ = timeline.Open(repo, itemID)
		}
		fmt.Fprintf(e.Stdout, "session bound to item %s — writes are now checked against its plan\n", itemID)
	} else {
		fmt.Fprintf(e.Stdout, "session bound to decision %s\n", decision)
	}
	return ExitOK
}

func runSessionCurrent(e Env, args []string) int {
	f := fs("session current", e)
	verbose := f.Bool("why", false, "say how the item was resolved")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	id, src := currentItem(repo, "")
	// Bare by default, so skills can inject it: $(circle session current).
	if *verbose {
		fmt.Fprintf(e.Stdout, "%s (from %s)\n", id, src)
		if b, ok := session.Lookup(repo.Root, session.ID("")); ok && b.Decision != "" {
			fmt.Fprintf(e.Stdout, "decision %s\n", b.Decision)
		}
		return ExitOK
	}
	fmt.Fprintln(e.Stdout, id)
	return ExitOK
}
