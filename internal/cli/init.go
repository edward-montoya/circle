package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/edwardmontoya/circle/internal/contract"
)

func init() {
	register(Command{"init", "Scaffold .circle/ and detect the three contracts", runInit})
	register(Command{"doctor", "Verify git, docker, gh and the plugin wiring", runDoctor})
}

// detected is what init inferred from the repository, before a human confirms.
type detected struct {
	Compose     string
	AppServices []string
	Infra       []string
	Gates       map[string]string
	Bootstrap   string
	Docs        []string
	Definitions []string
	Validations []string
	RunSkill    string
}

var (
	svcKey   = regexp.MustCompile(`^  ([A-Za-z0-9._-]+):\s*$`)
	buildKey = regexp.MustCompile(`^    build:`)
	imageKey = regexp.MustCompile(`^    image:`)
)

// classifyCompose applies D-10: `build:` present ⇒ application candidate.
// The heuristic proposes; a human confirms. Never silent — a wrong mark
// corrupts the execution contract.
func classifyCompose(path string) (app, infra []string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil
	}
	lines := strings.Split(string(b), "\n")
	inside := false
	cur := ""
	hasBuild := false
	flush := func() {
		if cur == "" {
			return
		}
		if hasBuild {
			app = append(app, cur)
		} else {
			infra = append(infra, cur)
		}
	}
	for _, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		if strings.TrimSpace(line) == "services:" && !strings.HasPrefix(line, " ") {
			inside = true
			continue
		}
		if !inside {
			continue
		}
		if line != "" && !strings.HasPrefix(line, " ") {
			break
		}
		if m := svcKey.FindStringSubmatch(line); m != nil {
			flush()
			cur, hasBuild = m[1], false
			continue
		}
		if buildKey.MatchString(line) {
			hasBuild = true
		}
		_ = imageKey
	}
	flush()
	return
}

func detect(root string) detected {
	d := detected{Gates: map[string]string{}}

	for _, name := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml"} {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			d.Compose = name
			d.AppServices, d.Infra = classifyCompose(filepath.Join(root, name))
			break
		}
	}

	if _, err := os.Stat(filepath.Join(root, "pyproject.toml")); err == nil {
		d.Gates["test:unit"] = "pytest"
		d.Bootstrap = "pip install -e \".[dev]\""
		b, _ := os.ReadFile(filepath.Join(root, "pyproject.toml"))
		if strings.Contains(string(b), "[tool.ruff]") {
			d.Gates["lint"] = "ruff check ."
			d.Gates["format"] = "ruff format --check ."
		}
		if strings.Contains(string(b), "[tool.uv") {
			d.Bootstrap = "uv sync"
		}
	}
	if _, err := os.Stat(filepath.Join(root, "package.json")); err == nil {
		d.Gates["typecheck"] = "npx tsc --noEmit"
		if d.Bootstrap == "" {
			d.Bootstrap = "npm ci"
		}
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
		d.Gates["test:unit"] = "go test ./..."
		d.Gates["lint"] = "go vet ./..."
	}

	for _, p := range []string{"README.md", "docs/", "CLAUDE.md", "AGENTS.md"} {
		if _, err := os.Stat(filepath.Join(root, strings.TrimSuffix(p, "/"))); err == nil {
			d.Docs = append(d.Docs, p)
		}
	}
	// A new project usually has exactly one artifact: the thing someone wrote
	// down before any code existed. It is the definition of what should be
	// built, and leaving it unregistered wastes the only context available.
	for _, p := range []string{
		"REQUIREMENTS.md", "PRD.md", "SPEC.md", "DESIGN.md",
		"docs/prd/", "docs/adr/", "docs/architecture/", "docs/specs/",
	} {
		if _, err := os.Stat(filepath.Join(root, strings.TrimSuffix(p, "/"))); err == nil {
			d.Definitions = append(d.Definitions, p)
		}
	}
	for _, p := range []string{"tests/", "test/", "e2e/", ".github/workflows/"} {
		if _, err := os.Stat(filepath.Join(root, strings.TrimSuffix(p, "/"))); err == nil {
			d.Validations = append(d.Validations, p)
		}
	}

	// D-25: a repo that already ran /run-skill-generator has the launch recipe
	// recorded. Reading it is the fastest path to a passing preflight, and turns
	// the closest competing feature into a detection source.
	matches, _ := filepath.Glob(filepath.Join(root, ".claude", "skills", "run-*", "SKILL.md"))
	if len(matches) > 0 {
		d.RunSkill, _ = filepath.Rel(root, matches[0])
	}
	return d
}

func runInit(e Env, args []string) int {
	f := fs("init", e)
	force := f.Bool("force", false, "overwrite an existing contract")
	detectOnly := f.Bool("detect-only", false, "print what would be written")
	if err := parse(f, args); err != nil {
		return ExitError
	}

	root, err := contract.FindRoot(e.Cwd)
	if err != nil {
		fmt.Fprintln(e.Stderr, "circle: not a git repository")
		return ExitPrecond
	}
	target := filepath.Join(root, contract.Dir, "project.toml")
	if _, err := os.Stat(target); err == nil && !*force && !*detectOnly {
		fmt.Fprintf(e.Stderr, "circle: %s already exists (use --force)\n", target)
		return ExitError
	}

	d := detect(root)
	p := NewPalette(e.Stdout)
	fmt.Fprintf(e.Stdout, "\n%sCircle%s  %sscanning %s%s\n\n", p.B, p.N, p.D, root, p.N)

	if d.Compose != "" {
		fmt.Fprintf(e.Stdout, "  %s✓%s compose      %s · %d app · %d infrastructure\n",
			p.G, p.N, d.Compose, len(d.AppServices), len(d.Infra))
		for _, s := range d.AppServices {
			fmt.Fprintf(e.Stdout, "      %s%-22s build: present → app candidate%s\n", p.D, s, p.N)
		}
		for _, s := range d.Infra {
			fmt.Fprintf(e.Stdout, "      %s%-22s image only     → infrastructure%s\n", p.D, s, p.N)
		}
	} else {
		fmt.Fprintf(e.Stdout, "  %s⚠%s compose      none found\n", p.Y, p.N)
	}
	if d.RunSkill != "" {
		fmt.Fprintf(e.Stdout, "  %s✓%s run recipe   %s\n", p.G, p.N, d.RunSkill)
	}
	names := make([]string, 0, len(d.Gates))
	for n := range d.Gates {
		names = append(names, n)
	}
	sort.Strings(names)
	if len(names) == 0 {
		fmt.Fprintf(e.Stdout, "  %s⚠%s quality      no gates found — nothing to prove yet\n", p.Y, p.N)
	} else {
		fmt.Fprintf(e.Stdout, "  %s✓%s quality      %d gate(s) detected\n", p.G, p.N, len(names))
	}
	for _, n := range names {
		fmt.Fprintf(e.Stdout, "      %s%-12s %s%s\n", p.D, n, d.Gates[n], p.N)
	}

	if *detectOnly {
		fmt.Fprintf(e.Stdout, "\n%sdetect-only: nothing written%s\n\n", p.D, p.N)
		return ExitOK
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	if err := os.WriteFile(target, []byte(renderContract(root, d)), 0o644); err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	fmt.Fprintf(e.Stdout, "\n  %s✓%s wrote        .circle/project.toml %s— commit this%s\n", p.G, p.N, p.D, p.N)
	fmt.Fprintf(e.Stdout, "\n%sNext%s\n  %scircle preflight%s   validate the three contracts\n\n",
		p.B, p.N, p.B, p.N)
	return ExitOK
}

func renderContract(root string, d detected) string {
	var b strings.Builder
	q := func(xs []string) string {
		out := make([]string, len(xs))
		for i, x := range xs {
			out[i] = fmt.Sprintf("%q", x)
		}
		return strings.Join(out, ", ")
	}
	fmt.Fprintf(&b, "# Circle AI — the replicable contract for %s.\n", filepath.Base(root))
	b.WriteString("# Detected by `circle init`. Every value is editable; preflight reports gaps\n")
	b.WriteString("# rather than inventing tooling this repo does not have.\n\n")
	fmt.Fprintf(&b, "[project]\nname = %q\ncontract_version = 1\n\n", filepath.Base(root))

	b.WriteString("[knowledge]\n")
	fmt.Fprintf(&b, "docs        = [%s]\n", q(d.Docs))
	if len(d.Definitions) > 0 {
		fmt.Fprintf(&b, "definitions = [%s]\n", q(d.Definitions))
	} else {
		b.WriteString("definitions = []   # what SHOULD be built: ADRs, PRDs, specs\n")
	}
	fmt.Fprintf(&b, "validations = [%s]\n\n", q(d.Validations))

	// Writing `up = docker compose ...` for a repo with no compose file would be
	// exactly the invention this contract's own header promises not to make.
	if d.Compose == "" {
		b.WriteString("# [execution] — nothing to run yet. Declare compose, up and down\n")
		b.WriteString("# once the project has something that starts, then re-run\n")
		b.WriteString("# `circle init --force` to re-detect.\n\n")
	} else {
		b.WriteString("[execution]\n")
		fmt.Fprintf(&b, "compose      = %q\n", d.Compose)
		fmt.Fprintf(&b, "app_services = [%s]\n", q(d.AppServices))
		b.WriteString("up           = \"docker compose up --build -d\"\n")
		b.WriteString("down         = \"docker compose down\"\n\n")
	}

	b.WriteString("[quality]\n")
	if d.Bootstrap != "" {
		fmt.Fprintf(&b, "bootstrap = %q   # makes the gates runnable on a fresh clone\n", d.Bootstrap)
	}
	for _, k := range []string{"format", "lint", "typecheck"} {
		if v, ok := d.Gates[k]; ok {
			fmt.Fprintf(&b, "%-9s = %q\n", k, v)
		}
	}
	if v, ok := d.Gates["test:unit"]; ok {
		fmt.Fprintf(&b, "\n[quality.test]\nunit = %q\n", v)
	}
	return b.String()
}

func runDoctor(e Env, args []string) int {
	p := NewPalette(e.Stdout)
	fmt.Fprintf(e.Stdout, "\n%sDOCTOR%s\n\n", p.B, p.N)
	required := []string{"git"}
	optional := []string{"docker", "gh", "jq"}
	missing := 0
	for _, tool := range required {
		if path, err := exec.LookPath(tool); err == nil {
			fmt.Fprintf(e.Stdout, "  %s✓%s %-8s %s%s%s\n", p.G, p.N, tool, p.D, path, p.N)
		} else {
			missing++
			fmt.Fprintf(e.Stdout, "  %s✗%s %-8s required\n", p.R, p.N, tool)
		}
	}
	for _, tool := range optional {
		if path, err := exec.LookPath(tool); err == nil {
			fmt.Fprintf(e.Stdout, "  %s✓%s %-8s %s%s%s\n", p.G, p.N, tool, p.D, path, p.N)
		} else {
			// Degrades to local-only rather than failing: an OSS tool that
			// hard-fails on a missing gh is unusable on half the repos it lands in.
			fmt.Fprintf(e.Stdout, "  %s○%s %-8s absent — degrades to local-only\n", p.D, p.N, tool)
		}
	}
	if repo, err := contract.Open(e.Cwd); err == nil {
		fmt.Fprintf(e.Stdout, "\n  %s✓%s contract %s\n", p.G, p.N, repo.Contract.String())
	} else {
		fmt.Fprintf(e.Stdout, "\n  %s○%s contract not scaffolded — run `circle init`\n", p.D, p.N)
	}
	fmt.Fprintln(e.Stdout)
	if missing > 0 {
		return ExitExternal
	}
	return ExitOK
}
