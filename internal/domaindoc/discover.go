package domaindoc

import (
	"bufio"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Term is one entry in a DOMAIN.md's language.
type Term struct {
	Name  string
	Avoid []string
	Line  int
}

// Domain is one DOMAIN.md: a bounded context covering the directory it sits in.
type Domain struct {
	Name    string
	Path    string // repo-relative path of the DOMAIN.md
	Scope   string // directory it covers; "" is the whole repository
	Terms   []Term
	Modules []string // paths declared under "## Modules"
	text    string
}

// ADR is one architecture decision record.
type ADR struct {
	Path   string
	Number string
	Title  string
	Status string
	Domain string // the domain whose scope contains it; "" is system-wide
	text   string
}

// Doc is an architecture document.
type Doc struct {
	Path  string
	Title string
	text  string
}

// Module is a unit of code: the architecture the domain model must account for.
type Module struct {
	Path   string
	Domain string // the domain it belongs to, by declaration or location
	By     string // what documents it; "" means nothing does
}

// Model is the whole domain model of a repository.
type Model struct {
	Domains      []Domain
	ADRs         []ADR
	Architecture []Doc
	Modules      []Module
}

// skipDirs are never part of the domain model or the code it describes.
var skipDirs = map[string]bool{
	".git": true, ".circle": true, ".claude": true, "node_modules": true, "vendor": true,
	"dist": true, "build": true, "target": true, "out": true, "coverage": true,
	".venv": true, "venv": true, "__pycache__": true, ".next": true, ".idea": true,
}

// Files lists the repository's files, repo-relative with forward slashes.
//
// git ls-files when there is a repository, so ignored files are ignored and new
// untracked ones are seen; a walk otherwise.
func Files(root string) []string {
	cmd := exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard")
	cmd.Dir = root
	if out, err := cmd.Output(); err == nil {
		var files []string
		seen := map[string]bool{}
		for _, l := range strings.Split(string(out), "\n") {
			if l = strings.TrimSpace(l); l != "" && !skipped(l) && !seen[l] {
				seen[l] = true
				files = append(files, l)
			}
		}
		sort.Strings(files)
		return files
	}
	var files []string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && skipDirs[d.Name()] {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			rel, _ := filepath.Rel(root, p)
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(files)
	return files
}

func skipped(rel string) bool {
	for _, part := range strings.Split(path.Dir(rel), "/") {
		if skipDirs[part] {
			return true
		}
	}
	return false
}

// Discover reads the domain model from the repository.
func Discover(root string) Model {
	files := Files(root)
	var m Model
	for _, f := range files {
		switch {
		case path.Base(f) == FileName:
			m.Domains = append(m.Domains, readDomain(root, f))
		case isADR(f):
			m.ADRs = append(m.ADRs, readADR(root, f))
		case isArchitecture(f):
			text := read(root, f)
			m.Architecture = append(m.Architecture, Doc{Path: f, Title: firstHeading(text, f), text: text})
		}
	}
	for i := range m.ADRs {
		if d := owner(m.Domains, path.Dir(path.Dir(m.ADRs[i].Path))); d != nil && d.Scope != "" {
			m.ADRs[i].Domain = d.Name
		}
	}
	m.Modules = modules(files, m)
	return m
}

func isADR(rel string) bool {
	return strings.HasSuffix(rel, ".md") && inDir(rel, "adr") && !strings.EqualFold(path.Base(rel), "README.md")
}

func isArchitecture(rel string) bool {
	return strings.EqualFold(path.Base(rel), "ARCHITECTURE.md") ||
		(strings.HasSuffix(rel, ".md") && inDir(rel, "architecture"))
}

func read(root, rel string) string {
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return ""
	}
	return string(b)
}

var (
	headingRe = regexp.MustCompile(`(?m)^#\s+(.+?)\s*$`)
	termRe    = regexp.MustCompile(`^\*\*([^*]+)\*\*\s*:`)
	avoidRe   = regexp.MustCompile(`^_Avoid_\s*:\s*(.+)$`)
	tickRe    = regexp.MustCompile("`([^`]+)`")
	statusRe  = regexp.MustCompile(`(?i)^\*\*Status:?\*\*:?\s*(.+)$`)
	adrNumRe  = regexp.MustCompile(`^(\d+)`)
)

func firstHeading(text, fallback string) string {
	if m := headingRe.FindStringSubmatch(text); m != nil {
		return m[1]
	}
	return fallback
}

// readDomain parses a DOMAIN.md. Terms are "**Term**:" lines; synonyms to avoid
// are the "_Avoid_:" line under them; modules are backticked paths under a
// "## Modules" heading. Everything else is prose for humans.
func readDomain(root, rel string) Domain {
	text := read(root, rel)
	scope := path.Dir(rel)
	if scope == "." {
		scope = ""
	}
	name := strings.TrimSpace(strings.TrimSuffix(firstHeading(text, ""), " domain"))
	if name == "" {
		name = path.Base(scope)
		if scope == "" {
			name = "root"
		}
	}
	d := Domain{Name: name, Path: rel, Scope: scope, text: text}
	section := ""
	sc := bufio.NewScanner(strings.NewReader(text))
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "## ") {
			section = strings.ToLower(strings.TrimSpace(line[3:]))
			continue
		}
		if m := termRe.FindStringSubmatch(line); m != nil && section != "modules" {
			d.Terms = append(d.Terms, Term{Name: strings.TrimSpace(m[1]), Line: n})
			continue
		}
		if m := avoidRe.FindStringSubmatch(line); m != nil && len(d.Terms) > 0 {
			for _, a := range strings.Split(m[1], ",") {
				if a = strings.Trim(strings.TrimSpace(a), "\"'`*_"); a != "" {
					d.Terms[len(d.Terms)-1].Avoid = append(d.Terms[len(d.Terms)-1].Avoid, a)
				}
			}
			continue
		}
		if section == "modules" {
			for _, m := range tickRe.FindAllStringSubmatch(line, -1) {
				d.Modules = append(d.Modules, strings.TrimSuffix(m[1], "/"))
			}
		}
	}
	return d
}

func readADR(root, rel string) ADR {
	text := read(root, rel)
	a := ADR{Path: rel, Title: firstHeading(text, path.Base(rel)), text: text}
	if m := adrNumRe.FindStringSubmatch(path.Base(rel)); m != nil {
		a.Number = m[1]
	}
	// "# 0003 · Use Postgres" — the number is already in its own column.
	if a.Number != "" {
		a.Title = strings.TrimSpace(strings.TrimLeft(strings.TrimPrefix(a.Title, a.Number), " ·-:."))
	}
	for _, l := range strings.Split(text, "\n") {
		if m := statusRe.FindStringSubmatch(strings.TrimSpace(l)); m != nil {
			a.Status = strings.TrimSpace(m[1])
			break
		}
	}
	return a
}

// owner is the domain whose scope most closely contains dir.
func owner(ds []Domain, dir string) *Domain {
	if dir == "." {
		dir = ""
	}
	var best *Domain
	for i := range ds {
		s := ds[i].Scope
		if s == "" || dir == s || strings.HasPrefix(dir+"/", s+"/") {
			if best == nil || len(s) > len(best.Scope) {
				best = &ds[i]
			}
		}
	}
	return best
}

// sourceExt are the languages whose files make up modules.
var sourceExt = map[string]bool{
	".go": true, ".ts": true, ".tsx": true, ".js": true, ".jsx": true, ".mjs": true,
	".py": true, ".rb": true, ".java": true, ".kt": true, ".cs": true, ".rs": true,
	".php": true, ".swift": true, ".scala": true, ".ex": true, ".exs": true,
}

// containers hold modules one level down: internal/task, src/orders,
// packages/api. Anything else at the top level is a module itself.
var containers = map[string]bool{
	"src": true, "internal": true, "pkg": true, "lib": true, "app": true, "apps": true,
	"packages": true, "services": true, "modules": true, "cmd": true, "components": true,
	"features": true, "domain": true, "domains": true,
}

// notModules are directories of code that is not architecture.
var notModules = map[string]bool{
	"test": true, "tests": true, "__tests__": true, "spec": true, "specs": true, "testdata": true,
	"fixtures": true, "e2e": true, "scripts": true, "script": true, "docs": true,
	"examples": true, "example": true, "tools": true, "migrations": true, "templates": true,
	"trials": true, "mocks": true,
}

// ModuleOf names the module a source file belongs to, or "" when it belongs to
// none: a root-level file, a test, a script.
func ModuleOf(rel string) string {
	if !sourceExt[path.Ext(rel)] || isTestFile(rel) {
		return ""
	}
	parts := strings.Split(path.Dir(rel), "/")
	if parts[0] == "." {
		return ""
	}
	for _, p := range parts {
		if notModules[p] {
			return ""
		}
	}
	if containers[parts[0]] {
		if len(parts) < 2 {
			return ""
		}
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}

func isTestFile(rel string) bool {
	b := path.Base(rel)
	return strings.HasSuffix(b, "_test.go") || strings.Contains(b, ".test.") ||
		strings.Contains(b, ".spec.") || strings.HasPrefix(b, "test_")
}

// modules finds every module and what documents it.
//
// A module is documented when a DOMAIN.md declares it, sits inside it, or names
// it — or when an ADR or architecture document names its path. Location alone
// is not enough: a root DOMAIN.md would otherwise cover every module in the
// repository without saying a word about any of them.
func modules(files []string, m Model) []Module {
	seen := map[string]bool{}
	var out []Module
	for _, f := range files {
		mod := ModuleOf(f)
		if mod == "" || seen[mod] {
			continue
		}
		seen[mod] = true
		out = append(out, documentModule(mod, m))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func documentModule(mod string, m Model) Module {
	mo := Module{Path: mod}
	if d := owner(m.Domains, mod); d != nil {
		mo.Domain = d.Name
	}
	for _, d := range m.Domains {
		for _, dm := range d.Modules {
			if dm == mod {
				mo.Domain, mo.By = d.Name, d.Path
				return mo
			}
		}
		if d.Scope != "" && (d.Scope == mod || strings.HasPrefix(d.Scope, mod+"/")) {
			mo.Domain, mo.By = d.Name, d.Path
			return mo
		}
		if mentions(d.text, mod) {
			mo.By = d.Path
			return mo
		}
	}
	for _, a := range m.ADRs {
		if mentions(a.text, mod) {
			mo.By = a.Path
			return mo
		}
	}
	for _, a := range m.Architecture {
		if mentions(a.text, mod) {
			mo.By = a.Path
			return mo
		}
	}
	return mo
}

// mentions reports whether text names a module path as a path.
func mentions(text, mod string) bool {
	if text == "" {
		return false
	}
	return strings.Contains(text, "`"+mod+"`") || strings.Contains(text, "`"+mod+"/`") ||
		strings.Contains(text, mod+"/")
}
