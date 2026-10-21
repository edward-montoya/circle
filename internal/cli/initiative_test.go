package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var idRe = regexp.MustCompile(`created (\S+)`)

func newDecision(t *testing.T, root string, args ...string) string {
	t.Helper()
	out := mustRun(t, root, append([]string{"decision", "new"}, args...)...)
	m := idRe.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("no id in:\n%s", out)
	}
	return m[1]
}

// The whole map, across sessions: chart, wire, claim, resolve, rule out, and
// finally produce an item.
func TestInitiativeMapLifecycle(t *testing.T) {
	root := newGitRepo(t)
	if code, _ := run(t, root, "initiative", "new", "billing"); code != ExitError {
		t.Fatal("an initiative with no destination must be refused")
	}
	mustRun(t, root, "initiative", "new", "billing", "--title", "Billing", "--destination", "A spec for usage billing")

	provider := newDecision(t, root, "--initiative", "billing", "--type", "research", "--title", "Payment provider", "--question", "Which provider fits?")
	model := newDecision(t, root, "--initiative", "billing", "--type", "grill", "--title", "Pricing model", "--question", "Seats or usage?")
	sso := newDecision(t, root, "--initiative", "billing", "--type", "grill", "--title", "SSO", "--question", "Do we need SSO?")
	mustRun(t, root, "decision", "link", model, "--blocked-by", provider)

	front := mustRun(t, root, "decision", "frontier")
	if !strings.Contains(front, "Payment provider") || strings.Contains(front, "Pricing model") {
		t.Fatalf("the blocked decision must wait behind its blocker:\n%s", front)
	}

	// A claim in one worktree takes the decision off every frontier.
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", "chart billing")
	wt := filepath.Join(t.TempDir(), "other")
	git(t, root, "worktree", "add", "-q", wt, "-b", "other")
	mustRun(t, root, "decision", "claim", provider)
	if out := mustRun(t, wt, "decision", "frontier"); strings.Contains(out, "Payment provider") {
		t.Fatalf("claimed in the main checkout, still offered in the worktree:\n%s", out)
	}
	if code, _ := run(t, wt, "decision", "claim", provider); code != ExitGate {
		t.Fatal("second claim from another worktree must be refused")
	}

	answer := filepath.Join(t.TempDir(), "a.md")
	_ = os.WriteFile(answer, []byte("Stripe: metered billing is native.\n"), 0o644)
	mustRun(t, root, "decision", "resolve", provider, "--answer", answer, "--gist", "Stripe, for metered billing")
	if out := mustRun(t, root, "decision", "frontier"); !strings.Contains(out, "Pricing model") {
		t.Fatalf("resolving the blocker should open the next decision:\n%s", out)
	}
	mustRun(t, root, "decision", "scope-out", sso, "--why", "enterprise is a later effort")

	show := mustRun(t, root, "initiative", "show", "billing")
	for _, want := range []string{"A spec for usage billing", "Payment provider (`" + provider + "`): Stripe, for metered billing",
		"[grill] Pricing model", "SSO (`" + sso + "`) — enterprise is a later effort"} {
		if !strings.Contains(show, want) {
			t.Errorf("map missing %q:\n%s", want, show)
		}
	}

	if code, _ := run(t, root, "item", "new", "x", "--title", "x", "--initiative", "nope"); code != ExitPrecond {
		t.Fatal("an item may not name an initiative that does not exist")
	}
	mustRun(t, root, "item", "new", "usage-meter", "--title", "Usage meter", "--initiative", "billing")
	if out := mustRun(t, root, "initiative", "show", "billing"); !strings.Contains(out, "Usage meter (`usage-meter`) — shaping") {
		t.Fatalf("the map should list its items:\n%s", out)
	}
}

// A decision session writes answers and domain language, and code only on a
// prototype branch.
func TestDecisionSessionWritesNoCodeOutsideAPrototypeBranch(t *testing.T) {
	root := newGitRepo(t)
	planItem(t, root, "alpha", "src/a/**")
	approve(t, root, "alpha")
	mustRun(t, root, "initiative", "new", "big", "--destination", "d")
	d := newDecision(t, root, "--initiative", "big", "--type", "prototype", "--question", "How should it feel?")
	t.Setenv("CIRCLE_SESSION", "s-dec")
	mustRun(t, root, "decision", "claim", d)

	hook := func(rel string) string {
		p := `{"session_id":"s-dec","cwd":"` + root + `","tool_name":"Write","tool_input":{"file_path":"` + filepath.Join(root, rel) + `"}}`
		_, out := runStdin(t, root, p, "gate", "check", "--hook")
		return out
	}
	if out := hook("src/proto/ui.tsx"); !strings.Contains(out, "produces decisions, not code") {
		t.Fatalf("code outside a prototype branch should be denied:\n%s", out)
	}
	if out := hook("docs/adr/0001-feel.md"); strings.Contains(out, "deny") {
		t.Fatalf("an ADR should be writable:\n%s", out)
	}
	git(t, root, "add", "-A")
	git(t, root, "commit", "-q", "-m", "wip")
	git(t, root, "checkout", "-q", "-b", "prototype/feel")
	if out := hook("src/proto/ui.tsx"); strings.Contains(out, "deny") {
		t.Fatalf("a prototype branch takes throwaway code:\n%s", out)
	}
}
