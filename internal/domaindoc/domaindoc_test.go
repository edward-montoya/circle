package domaindoc

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func gitRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, files)
	for _, args := range [][]string{
		{"init", "-q"}, {"config", "user.email", "d@x"}, {"config", "user.name", "d"},
		{"add", "."}, {"commit", "-q", "-m", "init", "--allow-empty"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return root
}

const ordersDomain = `# Orders domain

## Language

**Order**:
A customer's request to buy one or more products.
_Avoid_: purchase, basket

**Refund policy**:
The rules deciding how much of an order comes back.

## Modules

- ` + "`src/orders`" + `
`

func TestWords(t *testing.T) {
	cases := map[string]string{
		"OrderCancellationPolicy": "order cancellation policy",
		"HTTPServer":              "http server",
		"IOrderService":           "i order service",
		"refund_policy":           "refund policy",
	}
	for in, want := range cases {
		if got := strings.Join(Words(in), " "); got != want {
			t.Errorf("Words(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCoveredStripsPlumbingAndToleratesPlurals(t *testing.T) {
	root := gitRepo(t, map[string]string{"src/orders/DOMAIN.md": ordersDomain})
	v := VocabularyOf(Discover(root))
	for _, name := range []string{"Order", "Orders", "OrderRepository", "IOrderService", "RefundPolicyHandler", "Config"} {
		if ok, _ := v.Covered(name); !ok {
			t.Errorf("%s should be covered", name)
		}
	}
	if ok, _ := v.Covered("Shipment"); ok {
		t.Error("Shipment is not in the language")
	}
	if ok, use := v.Covered("BasketService"); ok || use != "Order" {
		t.Errorf("Basket is a banned synonym of Order: ok=%v use=%q", ok, use)
	}
}

func TestConceptsPerLanguage(t *testing.T) {
	goSrc := "package x\n\ntype Order struct{}\ntype (\n\tShipment struct{}\n\tcarrier struct{}\n)\nfunc F() {}\n"
	if got := Concepts("x/a.go", goSrc); len(got) != 2 || got[0].Name != "Order" || got[1].Name != "Shipment" {
		t.Errorf("go: %+v", got)
	}
	tsSrc := "export interface Invoice {}\nexport default class Ledger {}\nclass Hidden {}\n"
	if got := Concepts("x/a.ts", tsSrc); len(got) != 2 {
		t.Errorf("ts: %+v", got)
	}
	if got := Concepts("x/a_test.go", goSrc); got != nil {
		t.Error("test files declare no domain concepts")
	}
}

func TestDiscoverIndexesDomainsADRsAndModules(t *testing.T) {
	root := gitRepo(t, map[string]string{
		"src/orders/DOMAIN.md":          ordersDomain,
		"src/orders/order.ts":           "export class Order {}\n",
		"src/billing/invoice.ts":        "export class Invoice {}\n",
		"docs/adr/0001-use-postgres.md": "# 0001 · Use Postgres for orders\n\n**Status:** accepted\n",
		"ARCHITECTURE.md":               "# Architecture\n\nThe `src/billing/` module bills.\n",
	})
	m := Discover(root)
	if len(m.Domains) != 1 || m.Domains[0].Name != "Orders" || len(m.Domains[0].Terms) != 2 {
		t.Fatalf("domains: %+v", m.Domains)
	}
	if len(m.ADRs) != 1 || m.ADRs[0].Number != "0001" || m.ADRs[0].Title != "Use Postgres for orders" || m.ADRs[0].Status != "accepted" {
		t.Fatalf("adrs: %+v", m.ADRs)
	}
	by := map[string]string{}
	for _, mo := range m.Modules {
		by[mo.Path] = mo.By
	}
	if by["src/orders"] != "src/orders/DOMAIN.md" || by["src/billing"] != "ARCHITECTURE.md" {
		t.Fatalf("modules: %+v", m.Modules)
	}
	idx := Render(m)
	if Render(Discover(root)) != idx {
		t.Fatal("the index must be deterministic")
	}
	for _, want := range []string{"**Orders**", "Use Postgres for orders", "`src/billing/`", "_avoid_: purchase, basket"} {
		if !strings.Contains(idx, want) {
			t.Errorf("index is missing %q:\n%s", want, idx)
		}
	}
}

// A root DOMAIN.md must not silently cover a module it never mentions.
func TestRootDomainDoesNotCoverUnmentionedModules(t *testing.T) {
	root := gitRepo(t, map[string]string{
		"DOMAIN.md":           "# Shop\n\n## Language\n\n**Order**:\nA request to buy.\n",
		"src/orders/order.go": "package orders\n\ntype Order struct{}\n",
	})
	for _, mo := range Discover(root).Modules {
		if mo.Path == "src/orders" && mo.By != "" {
			t.Fatalf("root DOMAIN.md covered a module it never names: %+v", mo)
		}
	}
}

// Only what an edit introduces is flagged: legacy types already in the file at
// HEAD are not, so touching an old file does not block on old debt.
func TestCheckFlagsOnlyNewConcepts(t *testing.T) {
	root := gitRepo(t, map[string]string{
		"src/orders/DOMAIN.md": ordersDomain,
		"src/orders/order.go":  "package orders\n\ntype Order struct{}\ntype LegacyThing struct{}\n",
	})
	m := Discover(root)
	text := "package orders\n\ntype Order struct{}\ntype LegacyThing struct{}\ntype Shipment struct{}\ntype Basket struct{}\ntype RefundPolicyService struct{}\n"
	got := Check(root, m, "src/orders/order.go", text, nil)
	kinds := map[string]string{}
	for _, f := range got {
		kinds[f.Name] = f.Kind
	}
	if kinds["Shipment"] != "concept" || kinds["Basket"] != "avoided-term" {
		t.Fatalf("findings: %+v", got)
	}
	if _, ok := kinds["LegacyThing"]; ok {
		t.Fatal("a type already at HEAD was flagged")
	}
	if _, ok := kinds["RefundPolicyService"]; ok {
		t.Fatal("a covered concept was flagged")
	}
	if got[0].Suggest != "src/orders/DOMAIN.md" {
		t.Fatalf("suggested %q", got[0].Suggest)
	}
	if fs := Check(root, m, "src/orders/order.go", text, []string{"Ship*", "Basket"}); len(fs) != 0 {
		t.Fatalf("ignored names were flagged: %+v", fs)
	}
}

func TestCheckFlagsANewUndocumentedModule(t *testing.T) {
	root := gitRepo(t, map[string]string{"src/orders/DOMAIN.md": ordersDomain})
	write(t, root, map[string]string{"src/shipping/route.go": "package shipping\n\nfunc plan() {}\n"})
	m := Discover(root)
	got := Check(root, m, "src/shipping/route.go", "package shipping\n\nfunc plan() {}\n", nil)
	if len(got) != 1 || got[0].Kind != "module" || got[0].Suggest != "src/shipping/DOMAIN.md" {
		t.Fatalf("findings: %+v", got)
	}
}
