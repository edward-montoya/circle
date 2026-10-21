package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The post-write hook keeps the index current and hands a new, undocumented
// concept straight back to the agent.
func TestDomainSyncHookFlagsNewConceptsAndRefreshesTheIndex(t *testing.T) {
	root := newGitRepo(t)
	src := filepath.Join(root, "src", "orders", "order.go")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("package orders\n\ntype Shipment struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	payload := `{"cwd":"` + root + `","tool_name":"Write","tool_input":{"file_path":"` + src + `"}}`

	_, out := runStdin(t, root, payload, "domain", "sync", "--hook")
	if !strings.Contains(out, `"decision":"block"`) || !strings.Contains(out, "Shipment") ||
		!strings.Contains(out, "src/orders/DOMAIN.md") {
		t.Fatalf("expected a blocking finding naming Shipment and where it belongs:\n%s", out)
	}
	idx, err := os.ReadFile(filepath.Join(root, ".circle", "DOMAIN-INDEX.md"))
	if err != nil || !strings.Contains(string(idx), "`src/orders/`") {
		t.Fatalf("the index was not regenerated with the new module:\n%s", idx)
	}

	// Writing the term down clears it, and the index picks up the new domain.
	dom := filepath.Join(root, "src", "orders", "DOMAIN.md")
	if err := os.WriteFile(dom, []byte("# Orders\n\n## Language\n\n**Shipment**:\nGoods on their way.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, out := runStdin(t, root, payload, "domain", "sync", "--hook"); strings.TrimSpace(out) != "" {
		t.Fatalf("documented concept still flagged:\n%s", out)
	}
	if code, out := run(t, root, "domain", "index", "--check"); code != ExitOK {
		t.Fatalf("index should be current after the hook ran:\n%s", out)
	}
}

func TestDomainEnforceAdviseGivesContextNotABlock(t *testing.T) {
	root := newGitRepo(t)
	toml := filepath.Join(root, ".circle", "project.toml")
	b, _ := os.ReadFile(toml)
	if err := os.WriteFile(toml, append(b, []byte("\n[domain]\nenforce = \"advise\"\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "src", "orders", "order.go")
	_ = os.MkdirAll(filepath.Dir(src), 0o755)
	_ = os.WriteFile(src, []byte("package orders\n\ntype Shipment struct{}\n"), 0o644)
	payload := `{"cwd":"` + root + `","tool_name":"Write","tool_input":{"file_path":"` + src + `"}}`
	_, out := runStdin(t, root, payload, "domain", "sync", "--hook")
	if strings.Contains(out, `"block"`) || !strings.Contains(out, "additionalContext") {
		t.Fatalf("advise mode should return context, not a block:\n%s", out)
	}
	if code, _ := run(t, root, "domain", "check"); code != ExitOK {
		t.Fatal("advise mode: domain check must not fail")
	}
}
