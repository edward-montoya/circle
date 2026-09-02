// Package compose reads just enough of a Compose file to isolate it.
//
// Deliberately not a YAML dependency. Circle must run on a stranger's machine
// with nothing installed, and the binary ships inside a plugin's bin/. We need
// service names and published ports; anything more ambiguous is reported rather
// than silently mis-parsed.
package compose

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Port is one published mapping: a host port bound to a container port.
type Port struct {
	Service   string
	Host      int
	Container int
	// Raw is the original entry, kept so an unparseable one can be reported
	// verbatim instead of guessed at.
	Raw string
	// Bind is an optional host interface, e.g. "127.0.0.1".
	Bind string
}

func (p Port) String() string {
	if p.Bind != "" {
		return fmt.Sprintf("%s:%d:%d", p.Bind, p.Host, p.Container)
	}
	return fmt.Sprintf("%d:%d", p.Host, p.Container)
}

// Service is a compose service and how it is defined.
type Service struct {
	Name string
	// HasBuild drives the D-10 heuristic: build present ⇒ application candidate.
	HasBuild bool
	Ports    []Port
	// Unparseable holds port entries we could not read. They are surfaced so a
	// stack that will collide is never reported as isolated.
	Unparseable []string
}

// File is the parsed subset.
type File struct {
	Path     string
	Services []Service
}

// AppCandidates returns services carrying `build:` (D-10).
func (f File) AppCandidates() []string {
	var out []string
	for _, s := range f.Services {
		if s.HasBuild {
			out = append(out, s.Name)
		}
	}
	return out
}

// PublishedPorts flattens every host-bound port across services.
func (f File) PublishedPorts() []Port {
	var out []Port
	for _, s := range f.Services {
		out = append(out, s.Ports...)
	}
	return out
}

// Unparseable reports port entries that could not be read, by service.
func (f File) Unparseable() map[string][]string {
	out := map[string][]string{}
	for _, s := range f.Services {
		if len(s.Unparseable) > 0 {
			out[s.Name] = s.Unparseable
		}
	}
	return out
}

var (
	reService = regexp.MustCompile(`^  ([A-Za-z0-9._-]+):\s*$`)
	reKey4    = regexp.MustCompile(`^    ([a-z_]+):`)
	reItem    = regexp.MustCompile(`^      - +(.+?)\s*$`)
	reLongKey = regexp.MustCompile(`^        ([a-z_]+):\s*(.+?)\s*$`)
)

// Parse reads a compose file.
func Parse(path string) (File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}
	f := File{Path: path}

	var cur *Service
	inServices, inPorts, inLongPort := false, false, false
	long := map[string]string{}

	flushLong := func() {
		if !inLongPort || cur == nil {
			return
		}
		inLongPort = false
		host, err1 := strconv.Atoi(long["published"])
		container, err2 := strconv.Atoi(long["target"])
		raw := fmt.Sprintf("target: %s, published: %s", long["target"], long["published"])
		if err1 != nil || err2 != nil {
			cur.Unparseable = append(cur.Unparseable, raw)
		} else {
			cur.Ports = append(cur.Ports, Port{
				Service: cur.Name, Host: host, Container: container, Raw: raw,
			})
		}
		long = map[string]string{}
	}
	flushService := func() {
		flushLong()
		if cur != nil {
			f.Services = append(f.Services, *cur)
			cur = nil
		}
	}

	for _, raw := range strings.Split(string(b), "\n") {
		line := strings.TrimRight(raw, "\r")
		if i := strings.Index(line, " #"); i >= 0 {
			line = line[:i] // strip trailing comments, keep leading indentation
		}
		if strings.TrimSpace(line) == "" {
			continue
		}

		if !strings.HasPrefix(line, " ") {
			flushService()
			inServices = strings.TrimSpace(line) == "services:"
			continue
		}
		if !inServices {
			continue
		}

		if m := reService.FindStringSubmatch(line); m != nil {
			flushService()
			cur = &Service{Name: m[1]}
			inPorts = false
			continue
		}
		if cur == nil {
			continue
		}

		if m := reKey4.FindStringSubmatch(line); m != nil {
			flushLong()
			inPorts = m[1] == "ports"
			if m[1] == "build" {
				cur.HasBuild = true
			}
			continue
		}
		if !inPorts {
			continue
		}

		// Long syntax: a list item that opens a mapping.
		if m := reLongKey.FindStringSubmatch(line); m != nil {
			long[m[1]] = strings.Trim(m[2], `"'`)
			inLongPort = true
			continue
		}
		if m := reItem.FindStringSubmatch(line); m != nil {
			entry := strings.Trim(m[1], `"'`)
			if strings.HasPrefix(entry, "target:") {
				flushLong()
				inLongPort = true
				long["target"] = strings.TrimSpace(strings.TrimPrefix(entry, "target:"))
				continue
			}
			flushLong()
			if p, ok := parseShort(cur.Name, entry); ok {
				cur.Ports = append(cur.Ports, p)
			} else {
				cur.Unparseable = append(cur.Unparseable, entry)
			}
		}
	}
	flushService()
	return f, nil
}

// parseShort reads the short port syntax.
//
// Handled: "8000:8000", "127.0.0.1:4000:8000", "3000-3005:3000-3005" (rejected),
// "8000" (container-only, no host binding — nothing to remap).
func parseShort(service, entry string) (Port, bool) {
	entry = strings.TrimSuffix(entry, "/tcp")
	if strings.Contains(entry, "/udp") || strings.Contains(entry, "-") {
		// Ranges and udp are real but rare. Reporting beats guessing: a wrong
		// remap produces a stack that looks isolated and is not.
		return Port{}, false
	}
	parts := strings.Split(entry, ":")
	switch len(parts) {
	case 1:
		// Container port only — Docker assigns a random host port, so there is
		// nothing to collide over and nothing to remap.
		return Port{}, false
	case 2:
		host, err1 := strconv.Atoi(parts[0])
		container, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			return Port{}, false
		}
		return Port{Service: service, Host: host, Container: container, Raw: entry}, true
	case 3:
		host, err1 := strconv.Atoi(parts[1])
		container, err2 := strconv.Atoi(parts[2])
		if err1 != nil || err2 != nil {
			return Port{}, false
		}
		return Port{Service: service, Bind: parts[0], Host: host, Container: container, Raw: entry}, true
	}
	return Port{}, false
}
