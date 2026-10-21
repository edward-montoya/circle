package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/domaindoc"
)

func init() {
	register(Command{"domain index", "Regenerate .circle/DOMAIN-INDEX.md from every DOMAIN.md, ADR and module", runDomainIndex})
	register(Command{"domain check", "New concepts and modules the domain model does not cover", runDomainCheck})
	register(Command{"domain sync", "PostToolUse hook target: refresh the index, flag new concepts", runDomainSync})

	// Preflight reports the domain model alongside the other knowledge checks.
	contract.DomainChecker = func(r *contract.Repo) (domains, adrs, undocumented int, fresh bool) {
		m, fresh := domaindoc.Fresh(r.Root)
		for _, mo := range m.Modules {
			if mo.By == "" {
				undocumented++
			}
		}
		return len(m.Domains), len(m.ADRs), undocumented, fresh
	}
}

func runDomainIndex(e Env, args []string) int {
	f := fs("domain index", e)
	check := f.Bool("check", false, "exit 2 if the index is out of date, write nothing. For CI")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	if *check {
		if _, fresh := domaindoc.Fresh(repo.Root); !fresh {
			fmt.Fprintf(e.Stderr, "circle: .circle/%s is out of date — run `circle domain index` and commit it\n", domaindoc.IndexName)
			return ExitGate
		}
		fmt.Fprintf(e.Stdout, "✓ .circle/%s is current\n", domaindoc.IndexName)
		return ExitOK
	}
	m, changed, err := domaindoc.Sync(repo.Root)
	if err != nil {
		fmt.Fprintf(e.Stderr, "circle: %v\n", err)
		return ExitError
	}
	undocumented := 0
	for _, mo := range m.Modules {
		if mo.By == "" {
			undocumented++
		}
	}
	verb := "unchanged"
	if changed {
		verb = "updated"
	}
	p := NewPalette(e.Stdout)
	fmt.Fprintf(e.Stdout, "%s✓%s .circle/%s %s\n", p.G, p.N, domaindoc.IndexName, verb)
	fmt.Fprintf(e.Stdout, "  %d domain(s) · %d ADR(s) · %d architecture doc(s) · %d module(s)",
		len(m.Domains), len(m.ADRs), len(m.Architecture), len(m.Modules))
	if undocumented > 0 {
		fmt.Fprintf(e.Stdout, " · %s%d undocumented%s", p.Y, undocumented, p.N)
	}
	fmt.Fprintln(e.Stdout)
	return ExitOK
}

func runDomainCheck(e Env, args []string) int {
	f := fs("domain check", e)
	since := f.String("since", "HEAD", "judge everything changed since this revision")
	asJSON := f.Bool("json", false, "machine-readable")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	repo, code := openRepo(e)
	if repo == nil {
		return code
	}
	m := domaindoc.Discover(repo.Root)
	cfg := repo.Contract.Domain
	findings := domaindoc.CheckSince(repo.Root, m, *since, cfg.Ignore)
	if *asJSON {
		writeJSON(e, findings)
	} else if len(findings) == 0 {
		fmt.Fprintf(e.Stdout, "✓ every concept and module changed since %s is in the domain model\n", *since)
	} else {
		p := NewPalette(e.Stdout)
		fmt.Fprintf(e.Stdout, "%s%d gap(s) in the domain model since %s%s\n\n", p.Y, len(findings), *since, p.N)
		for _, fd := range findings {
			fmt.Fprintf(e.Stdout, "  %s⚠%s %s\n", p.Y, p.N, fd)
		}
		fmt.Fprintf(e.Stdout, "\n%sWrite each into its DOMAIN.md (the circle:domain practice), or list a\n"+
			"non-concept under [domain] ignore in .circle/project.toml.%s\n", p.D, p.N)
	}
	if len(findings) > 0 && cfg.Blocks() {
		return ExitGate
	}
	return ExitOK
}

// writePayload is the subset of a PostToolUse input we read.
type writePayload struct {
	Cwd       string `json:"cwd"`
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		FilePath string `json:"file_path"`
	} `json:"tool_input"`
}

// runDomainSync runs after every file write. Two jobs:
//
//  1. Keep the index current. It is regenerated on every write rather than on
//     domain-doc writes alone, because a new module changes the index too — and
//     an index that has to be remembered is one that will be forgotten.
//  2. Catch a new concept the moment it is written. A type the file did not
//     declare before, with no term in any DOMAIN.md, is fed straight back to the
//     agent, so the domain model is written alongside the code rather than in a
//     cleanup pass nobody schedules.
//
// It never fails the write: the file is already on disk. Under [domain]
// enforce = "block" the finding comes back as a blocking reason the agent must
// act on; under "advise" it arrives as context.
func runDomainSync(e Env, args []string) int {
	f := fs("domain sync", e)
	hook := f.Bool("hook", false, "read a PostToolUse payload from stdin")
	if err := parse(f, args); err != nil {
		return ExitError
	}
	cwd, target := e.Cwd, ""
	if *hook {
		var p writePayload
		b, _ := io.ReadAll(e.Stdin)
		if json.Unmarshal(b, &p) != nil {
			return ExitOK
		}
		if p.Cwd != "" {
			cwd = p.Cwd
		}
		target = p.ToolInput.FilePath
	}
	repo, err := contract.Open(cwd)
	if err != nil {
		return ExitOK
	}
	m, _, err := domaindoc.Sync(repo.Root)
	if err != nil || target == "" {
		return ExitOK
	}
	rel, err := filepath.Rel(repo.Root, target)
	if err != nil || strings.HasPrefix(rel, "..") {
		return ExitOK
	}
	rel = filepath.ToSlash(rel)
	text, err := os.ReadFile(target)
	if err != nil {
		return ExitOK
	}
	findings := domaindoc.Check(repo.Root, m, rel, string(text), repo.Contract.Domain.Ignore)
	if len(findings) == 0 {
		return ExitOK
	}

	var b strings.Builder
	b.WriteString("circle: this write adds to the code what the domain model does not describe yet.\n")
	for _, fd := range findings {
		b.WriteString("  - " + fd.String() + "\n")
	}
	b.WriteString("Write each one into its DOMAIN.md now — one or two sentences on what it *is* in the domain, " +
		"not how it is built (the circle:domain practice). If one is plumbing rather than a concept, " +
		"add a glob for it under [domain] ignore in .circle/project.toml.")

	if repo.Contract.Domain.Blocks() {
		out, _ := json.Marshal(map[string]string{"decision": "block", "reason": b.String()})
		fmt.Fprintln(e.Stdout, string(out))
		return ExitOK
	}
	out, _ := json.Marshal(map[string]any{"hookSpecificOutput": map[string]string{
		"hookEventName": "PostToolUse", "additionalContext": b.String(),
	}})
	fmt.Fprintln(e.Stdout, string(out))
	return ExitOK
}
