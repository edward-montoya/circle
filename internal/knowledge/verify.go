// Package knowledge checks registered documents against the repository.
//
// Circle's premise is that the repo tells the agent the truth. A registered
// definition that contradicts the code inverts that: the agent reads an
// authoritative-looking document, plans against it, and produces work aimed at a
// system that does not exist. A stale definition is worse than a missing one —
// an empty registry is honest, a wrong one is a lie with a checkmark.
//
// Everything here is deterministic. Following D-5, code detects and reports the
// evidence; a model may later classify an ambiguous hit, but never asserts drift
// on its own. Every finding names the line it came from so a human can dismiss a
// false positive in one second.
package knowledge

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/edwardmontoya/circle/internal/compose"
	"github.com/edwardmontoya/circle/internal/contract"
)

// Kind is the closed set of contradictions we can prove.
type Kind string

const (
	// MissingPath: the document names a repository path that does not exist.
	MissingPath Kind = "missing-path"
	// AbsentStack: the document names a technology with no trace in the repo.
	AbsentStack Kind = "absent-stack"
	// UnknownService: the document names a service the compose file does not have.
	UnknownService Kind = "unknown-service"
)

// Finding is one contradiction, with the evidence for it.
type Finding struct {
	Kind     Kind
	Document string
	Line     int
	Claim    string // what the document asserts
	Reality  string // what the repository actually contains
	Excerpt  string
}

func (f Finding) String() string {
	return fmt.Sprintf("%s:%d %s — %s", f.Document, f.Line, f.Claim, f.Reality)
}

// stackToken maps a technology named in prose to the evidence that would prove
// it is really used.
//
// Deliberately small. A vocabulary that guesses produces false positives, and a
// false positive here costs more than a missed one: it teaches people to ignore
// the report.
type stackToken struct {
	Name     string
	Pattern  *regexp.Regexp
	Evidence []string // any one of these present in the repo confirms it
	Images   []string // or a compose image containing one of these
}

var stackVocabulary = []stackToken{
	{"Go", regexp.MustCompile(`(?i)\b(golang|written in go|go service|go module)\b`),
		[]string{"go.mod"}, nil},
	{"Node", regexp.MustCompile(`(?i)\b(node\.js|nodejs|npm|typescript|javascript)\b`),
		[]string{"package.json"}, nil},
	{"Python", regexp.MustCompile(`(?i)\b(python|django|fastapi|flask)\b`),
		[]string{"pyproject.toml", "requirements.txt", "setup.py"}, nil},
	{"Rust", regexp.MustCompile(`(?i)\b(rust|cargo)\b`), []string{"Cargo.toml"}, nil},
	{"Ruby", regexp.MustCompile(`(?i)\b(ruby|rails)\b`), []string{"Gemfile"}, nil},
	{"Java", regexp.MustCompile(`(?i)\b(java|spring boot|maven|gradle)\b`),
		[]string{"pom.xml", "build.gradle"}, nil},

	{"PostgreSQL", regexp.MustCompile(`(?i)\b(postgres|postgresql)\b`), nil,
		[]string{"postgres", "timescale"}},
	{"MySQL", regexp.MustCompile(`(?i)\b(mysql|mariadb)\b`), nil, []string{"mysql", "mariadb"}},
	{"MongoDB", regexp.MustCompile(`(?i)\b(mongo|mongodb)\b`), nil, []string{"mongo"}},
	{"Redis", regexp.MustCompile(`(?i)\bredis\b`), nil, []string{"redis"}},
	{"RabbitMQ", regexp.MustCompile(`(?i)\b(rabbitmq|rabbit mq|amqp)\b`), nil, []string{"rabbitmq"}},
	{"Kafka", regexp.MustCompile(`(?i)\bkafka\b`), nil, []string{"kafka", "redpanda"}},
	{"Elasticsearch", regexp.MustCompile(`(?i)\b(elasticsearch|opensearch)\b`), nil,
		[]string{"elasticsearch", "opensearch"}},
}

// pathRef matches something that unambiguously looks like a repository path:
// at least one slash, no spaces, no scheme. Anything looser produces noise.
var pathRef = regexp.MustCompile("`([A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.*-]+)+/?)`")

// emphasis strips markdown bold and italics so a technology name written as
// **Go** still matches a word-boundary pattern.
var emphasis = regexp.MustCompile(`[*_]{1,3}`)

// Verify checks every registered definition against the repository.
//
// Only `definitions` are checked. Docs describe how the system works today and
// go stale harmlessly; definitions describe what should be built, and a wrong
// one actively misdirects the plan.
func Verify(r *contract.Repo) ([]Finding, error) {
	var findings []Finding

	services, images := repoServices(r)
	present := detectStack(r, images)

	for _, doc := range r.Contract.Knowledge.Definitions {
		matches, _ := filepath.Glob(r.Path(doc))
		if len(matches) == 0 {
			if _, err := os.Stat(r.Path(doc)); err == nil {
				matches = []string{r.Path(doc)}
			}
		}
		for _, m := range matches {
			info, err := os.Stat(m)
			if err != nil || info.IsDir() {
				continue
			}
			rel, _ := filepath.Rel(r.Root, m)
			f, err := verifyDoc(r, m, rel, services, present)
			if err != nil {
				continue
			}
			findings = append(findings, f...)
		}
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Document != findings[j].Document {
			return findings[i].Document < findings[j].Document
		}
		return findings[i].Line < findings[j].Line
	})
	return findings, nil
}

func verifyDoc(r *contract.Repo, abs, rel string, services map[string]bool, present map[string]bool) ([]Finding, error) {
	fh, err := os.Open(abs)
	if err != nil {
		return nil, err
	}
	defer fh.Close()

	var out []Finding
	claimed := map[string]int{} // technology -> first line it was claimed on
	excerptOf := map[string]string{}

	sc := bufio.NewScanner(fh)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		trimmed := strings.TrimSpace(line)
		// Documents write technology names in bold constantly — "written in
		// **Go**" — and emphasis markers break word-boundary matching. Strip them
		// for the stack scan only; path and service detection needs the
		// backticks and must see the original.
		plain := emphasis.ReplaceAllString(line, "")

		// A path in backticks that does not exist. The highest-signal check
		// there is: documents rot by referring to things that moved.
		for _, m := range pathRef.FindAllStringSubmatch(line, -1) {
			p := strings.TrimSuffix(m[1], "/")
			if strings.Contains(p, "://") || looksLikeCommand(p) {
				continue
			}
			if _, err := os.Stat(r.Path(p)); err == nil {
				continue
			}
			if hits, _ := filepath.Glob(r.Path(p)); len(hits) > 0 {
				continue
			}
			out = append(out, Finding{
				Kind: MissingPath, Document: rel, Line: n,
				Claim:   p,
				Reality: "no such path in the repository",
				Excerpt: truncate(trimmed),
			})
		}

		for _, tok := range stackVocabulary {
			if tok.Pattern.MatchString(plain) {
				if _, seen := claimed[tok.Name]; !seen {
					claimed[tok.Name] = n
					excerptOf[tok.Name] = truncate(trimmed)
				}
			}
		}

		// A service named in prose that the compose file does not define.
		for _, m := range regexp.MustCompile("`([a-z][a-z0-9_-]{2,})`").FindAllStringSubmatch(line, -1) {
			name := m[1]
			if services[name] || len(services) == 0 {
				continue
			}
			if !looksLikeService(name) {
				continue
			}
			out = append(out, Finding{
				Kind: UnknownService, Document: rel, Line: n,
				Claim:   name,
				Reality: "not a service in the compose file (" + joinKeys(services) + ")",
				Excerpt: truncate(trimmed),
			})
		}
	}

	for name, line := range claimed {
		if present[name] {
			continue
		}
		out = append(out, Finding{
			Kind: AbsentStack, Document: rel, Line: line,
			Claim:   name,
			Reality: "no trace of it in the repository",
			Excerpt: excerptOf[name],
		})
	}
	return out, sc.Err()
}

// repoServices returns the compose services and the images they use.
func repoServices(r *contract.Repo) (map[string]bool, []string) {
	services := map[string]bool{}
	var images []string
	if r.Contract.Execution.Compose == "" {
		return services, images
	}
	cf, err := compose.Parse(r.Path(r.Contract.Execution.Compose))
	if err != nil {
		return services, images
	}
	for _, s := range cf.Services {
		services[s.Name] = true
	}
	// The parser does not retain image strings, so read them directly. Cheap,
	// and it keeps the parser focused on ports and services.
	if b, err := os.ReadFile(r.Path(r.Contract.Execution.Compose)); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			t := strings.TrimSpace(line)
			if strings.HasPrefix(t, "image:") {
				images = append(images, strings.TrimSpace(strings.TrimPrefix(t, "image:")))
			}
		}
	}
	return services, images
}

// detectStack reports which technologies the repository actually shows evidence of.
func detectStack(r *contract.Repo, images []string) map[string]bool {
	present := map[string]bool{}
	for _, tok := range stackVocabulary {
		for _, ev := range tok.Evidence {
			if _, err := os.Stat(r.Path(ev)); err == nil {
				present[tok.Name] = true
			}
		}
		for _, want := range tok.Images {
			for _, img := range images {
				if strings.Contains(strings.ToLower(img), want) {
					present[tok.Name] = true
				}
			}
		}
	}
	return present
}

// looksLikeCommand filters out shell snippets, which contain slashes but are not
// repository paths.
func looksLikeCommand(s string) bool {
	for _, prefix := range []string{"http", "npm/", "go/", "-"} {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return strings.Contains(s, "--")
}

// looksLikeService keeps the service check to names that read like containers,
// rather than every lowercase word someone put in backticks.
func looksLikeService(name string) bool {
	for _, hint := range []string{"api", "web", "worker", "scheduler", "queue",
		"gateway", "service", "server", "db", "cache", "proxy"} {
		if strings.Contains(name, hint) {
			return true
		}
	}
	return false
}

func joinKeys(m map[string]bool) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

func truncate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 88 {
		return s[:85] + "..."
	}
	return s
}

// ByDocument groups findings for reporting.
func ByDocument(fs []Finding) map[string][]Finding {
	out := map[string][]Finding{}
	for _, f := range fs {
		out[f.Document] = append(out[f.Document], f)
	}
	return out
}
