package domaindoc

import (
	"bufio"
	"fmt"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// Concept is a named type the code declares: the place a domain concept turns
// into code.
type Concept struct {
	Name string
	Line int
}

// declRes find exported type declarations per language. Regexes, not parsers:
// the check must be fast enough for a hook on every write, and a declaration
// line is regular enough in every language here.
var declRes = map[string]*regexp.Regexp{
	".go":    regexp.MustCompile(`^type\s+([A-Z][A-Za-z0-9_]*)\b`),
	".ts":    regexp.MustCompile(`^export\s+(?:default\s+)?(?:declare\s+)?(?:abstract\s+)?(?:class|interface|type|enum)\s+([A-Z][A-Za-z0-9_]*)`),
	".tsx":   regexp.MustCompile(`^export\s+(?:default\s+)?(?:abstract\s+)?(?:class|interface|type|enum)\s+([A-Z][A-Za-z0-9_]*)`),
	".js":    regexp.MustCompile(`^export\s+(?:default\s+)?class\s+([A-Z][A-Za-z0-9_]*)`),
	".jsx":   regexp.MustCompile(`^export\s+(?:default\s+)?class\s+([A-Z][A-Za-z0-9_]*)`),
	".mjs":   regexp.MustCompile(`^export\s+(?:default\s+)?class\s+([A-Z][A-Za-z0-9_]*)`),
	".py":    regexp.MustCompile(`^class\s+([A-Z][A-Za-z0-9_]*)`),
	".rb":    regexp.MustCompile(`^\s*class\s+([A-Z][A-Za-z0-9_]*)`),
	".java":  regexp.MustCompile(`^\s*public\s+(?:abstract\s+|final\s+|sealed\s+)*(?:class|interface|enum|record)\s+([A-Z][A-Za-z0-9_]*)`),
	".kt":    regexp.MustCompile(`^\s*(?:data\s+|sealed\s+|abstract\s+|open\s+|enum\s+)*(?:class|interface|object)\s+([A-Z][A-Za-z0-9_]*)`),
	".cs":    regexp.MustCompile(`^\s*public\s+(?:abstract\s+|sealed\s+|static\s+|partial\s+)*(?:class|interface|enum|record|struct)\s+([A-Z][A-Za-z0-9_]*)`),
	".rs":    regexp.MustCompile(`^pub\s+(?:struct|enum|trait|type)\s+([A-Z][A-Za-z0-9_]*)`),
	".php":   regexp.MustCompile(`^(?:final\s+|abstract\s+)?(?:class|interface|enum|trait)\s+([A-Z][A-Za-z0-9_]*)`),
	".swift": regexp.MustCompile(`^\s*(?:public\s+|open\s+)?(?:final\s+)?(?:class|struct|enum|protocol)\s+([A-Z][A-Za-z0-9_]*)`),
	".scala": regexp.MustCompile(`^\s*(?:case\s+|sealed\s+|abstract\s+)*(?:class|trait|object)\s+([A-Z][A-Za-z0-9_]*)`),
	".ex":    regexp.MustCompile(`^\s*defmodule\s+(?:[A-Z][A-Za-z0-9]*\.)*([A-Z][A-Za-z0-9_]*)`),
}

// goGroupRe catches the names inside a `type ( ... )` block.
var goGroupRe = regexp.MustCompile(`^\s+([A-Z][A-Za-z0-9_]*)\s+(?:struct|interface|func|map|\[|\*|[a-z])`)

// Concepts lists the exported types a source file declares.
func Concepts(rel, text string) []Concept {
	re := declRes[path.Ext(rel)]
	if re == nil || isTestFile(rel) {
		return nil
	}
	var out []Concept
	inGoGroup := false
	sc := bufio.NewScanner(strings.NewReader(text))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if path.Ext(rel) == ".go" {
			switch {
			case strings.HasPrefix(line, "type ("):
				inGoGroup = true
				continue
			case inGoGroup && strings.HasPrefix(line, ")"):
				inGoGroup = false
				continue
			case inGoGroup:
				if m := goGroupRe.FindStringSubmatch(line); m != nil {
					out = append(out, Concept{m[1], n})
				}
				continue
			}
		}
		if m := re.FindStringSubmatch(line); m != nil {
			out = append(out, Concept{m[1], n})
		}
	}
	return out
}

// technical are words that say how a concept is built, not what it is. They are
// stripped from both ends of a name before it is matched against the language:
// OrderRepository and IOrderService are both the Order.
var technical = map[string]bool{
	"service": true, "services": true, "handler": true, "handlers": true, "controller": true,
	"repository": true, "repo": true, "store": true, "manager": true, "provider": true,
	"factory": true, "builder": true, "client": true, "server": true, "adapter": true,
	"port": true, "gateway": true, "dto": true, "impl": true, "base": true, "abstract": true,
	"default": true, "config": true, "configuration": true, "options": true, "opts": true,
	"settings": true, "params": true, "args": true, "request": true, "response": true,
	"req": true, "res": true, "result": true, "error": true, "err": true, "exception": true,
	"props": true, "state": true, "view": true, "component": true, "page": true, "screen": true,
	"model": true, "entity": true, "schema": true, "record": true, "spec": true, "input": true,
	"output": true, "payload": true, "data": true, "info": true, "util": true, "utils": true,
	"helper": true, "helpers": true, "middleware": true, "module": true, "router": true,
	"route": true, "routes": true, "api": true, "interface": true, "type": true, "types": true,
	"mock": true, "fake": true, "stub": true, "test": true, "context": true, "ctx": true,
	"i": true, "listener": true, "subscriber": true, "worker": true, "job": true,
}

// Words splits an identifier or a term into lower-case words:
// "OrderCancellationPolicy" → order cancellation policy; "HTTPServer" → http server.
func Words(s string) []string {
	var words []string
	var cur []rune
	rs := []rune(s)
	flush := func() {
		if len(cur) > 0 {
			words = append(words, strings.ToLower(string(cur)))
			cur = cur[:0]
		}
	}
	for i, r := range rs {
		switch {
		case !unicode.IsLetter(r) && !unicode.IsDigit(r):
			flush()
		case unicode.IsUpper(r) && len(cur) > 0 &&
			(unicode.IsLower(rs[i-1]) || (i+1 < len(rs) && unicode.IsLower(rs[i+1]))):
			flush()
			cur = append(cur, r)
		default:
			cur = append(cur, r)
		}
	}
	flush()
	return words
}

func core(words []string) []string {
	for len(words) > 0 && technical[words[0]] {
		words = words[1:]
	}
	for len(words) > 0 && technical[words[len(words)-1]] {
		words = words[:len(words)-1]
	}
	return words
}

// key normalises words for comparison, tolerating a plural last word.
func key(words []string) string {
	if len(words) == 0 {
		return ""
	}
	w := append([]string(nil), words...)
	last := w[len(w)-1]
	switch {
	case strings.HasSuffix(last, "ies") && len(last) > 4:
		last = strings.TrimSuffix(last, "ies") + "y"
	case strings.HasSuffix(last, "ses") || strings.HasSuffix(last, "xes"):
		last = strings.TrimSuffix(last, "es")
	case strings.HasSuffix(last, "s") && !strings.HasSuffix(last, "ss") && len(last) > 3:
		last = strings.TrimSuffix(last, "s")
	}
	w[len(w)-1] = last
	return strings.Join(w, " ")
}

// Vocabulary is every term across every domain, by normalised key.
type Vocabulary struct {
	terms map[string]string // key → term as written
	avoid map[string]string // key of a banned synonym → the term to use instead
}

// VocabularyOf builds the lookup from a model.
func VocabularyOf(m Model) Vocabulary {
	v := Vocabulary{terms: map[string]string{}, avoid: map[string]string{}}
	for _, d := range m.Domains {
		for _, t := range d.Terms {
			v.terms[key(Words(t.Name))] = t.Name
			for _, a := range t.Avoid {
				v.avoid[key(Words(a))] = t.Name
			}
		}
	}
	return v
}

// Finding is a concept or module the domain model does not account for.
type Finding struct {
	Kind    string // "concept", "avoided-term", "module"
	Path    string
	Line    int
	Name    string
	Use     string // for avoided-term: the term to use instead
	Suggest string // the DOMAIN.md this belongs in
}

func (f Finding) String() string {
	switch f.Kind {
	case "avoided-term":
		return fmt.Sprintf("%s:%d %s uses a term the domain bans — call it %s", f.Path, f.Line, f.Name, f.Use)
	case "module":
		return fmt.Sprintf("%s/ is a new module no DOMAIN.md, ADR or architecture doc describes — add it to %s", f.Name, f.Suggest)
	}
	return fmt.Sprintf("%s:%d %s is a new concept missing from the domain model — add it to %s", f.Path, f.Line, f.Name, f.Suggest)
}

// Covered reports whether a concept name is in the language. A name whose core
// is empty after stripping technical words (a bare "Config") is plumbing, not a
// concept, and is covered by definition.
func (v Vocabulary) Covered(name string) (bool, string) {
	c := core(Words(name))
	if len(c) == 0 {
		return true, ""
	}
	k := key(c)
	if _, ok := v.terms[k]; ok {
		return true, ""
	}
	if use, ok := v.avoid[k]; ok {
		return false, use
	}
	return false, ""
}

// Suggest names the DOMAIN.md a file's concepts belong in: the nearest existing
// one, or a new one at the file's module.
func Suggest(m Model, rel string) string {
	dir := path.Dir(rel)
	if d := owner(m.Domains, dir); d != nil {
		return d.Path
	}
	if mod := ModuleOf(rel); mod != "" {
		return mod + "/" + FileName
	}
	return FileName
}

// Check finds what a change to one file introduces that the domain model does
// not account for, judged against HEAD.
//
// Only what is new counts: a name the file did not declare before, a module
// with no tracked file before. Requiring every existing type in a legacy file
// to be documented the first time someone touches it would block all work on
// day one; requiring every new one keeps the model current from then on.
func Check(root string, m Model, rel, text string, ignore []string) []Finding {
	return checkFile(root, m, VocabularyOf(m), "HEAD", rel, text, ignore, map[string]bool{})
}

// CheckSince checks every source file changed since rev, against rev, so a
// branch's whole contribution is judged at once — the shape a review or a CI
// gate needs.
func CheckSince(root string, m Model, rev string, ignore []string) []Finding {
	v := VocabularyOf(m)
	reported := map[string]bool{}
	var out []Finding
	for _, rel := range ChangedSince(root, rev) {
		out = append(out, checkFile(root, m, v, rev, rel, read(root, rel), ignore, reported)...)
	}
	return out
}

func checkFile(root string, m Model, v Vocabulary, rev, rel, text string, ignore []string, reported map[string]bool) []Finding {
	before := map[string]bool{}
	for _, c := range Concepts(rel, gitShow(root, rev, rel)) {
		before[c.Name] = true
	}
	var out []Finding
	for _, c := range Concepts(rel, text) {
		if before[c.Name] || ignored(c.Name, ignore) {
			continue
		}
		ok, use := v.Covered(c.Name)
		switch {
		case use != "":
			out = append(out, Finding{Kind: "avoided-term", Path: rel, Line: c.Line, Name: c.Name, Use: use})
		case !ok:
			out = append(out, Finding{Kind: "concept", Path: rel, Line: c.Line, Name: c.Name, Suggest: Suggest(m, rel)})
		}
	}
	if mod := ModuleOf(rel); mod != "" && !reported[mod] && !treeHas(root, rev, mod) {
		reported[mod] = true
		for _, mo := range m.Modules {
			if mo.Path == mod && mo.By == "" {
				out = append(out, Finding{Kind: "module", Path: rel, Name: mod, Suggest: Suggest(m, rel)})
			}
		}
	}
	return out
}

func ignored(name string, globs []string) bool {
	for _, g := range globs {
		if ok, _ := filepath.Match(g, name); ok {
			return true
		}
	}
	return false
}

func gitShow(root, rev, rel string) string {
	cmd := exec.Command("git", "show", rev+":"+rel)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// ChangedSince lists source files changed since a revision, working tree
// included.
func ChangedSince(root, rev string) []string {
	cmd := exec.Command("git", "diff", "--name-only", rev)
	cmd.Dir = root
	out, _ := cmd.Output()
	set := map[string]bool{}
	for _, l := range strings.Split(string(out), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			set[l] = true
		}
	}
	cmd = exec.Command("git", "ls-files", "--others", "--exclude-standard")
	cmd.Dir = root
	out, _ = cmd.Output()
	for _, l := range strings.Split(string(out), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			set[l] = true
		}
	}
	var files []string
	for f := range set {
		if declRes[path.Ext(f)] != nil && !skipped(f) {
			files = append(files, f)
		}
	}
	sort.Strings(files)
	return files
}

func treeHas(root, rev, mod string) bool {
	cmd := exec.Command("git", "ls-tree", "-r", "--name-only", rev, "--", mod)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return true // no git or no such revision: nothing can be called new
	}
	return strings.TrimSpace(string(out)) != ""
}
