package compose

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "docker-compose.yml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// The two Phase 0 targets, reduced to their shapes: bb-control publishes
// literal host ports, surebets mixes one image-only service with many built ones.
const realistic = `# comment above services
services:
  api:
    build: .
    container_name: bbcontrol-api
    environment:
      FOO: bar
    ports:
      # Exposed so the API is reachable directly.
      - "8000:8000"
    restart: unless-stopped

  web:
    build: ./frontend
    depends_on:
      - api
    ports:
      - "8080:80"

  redis:
    image: redis:7
    ports:
      - "6379:6379"

volumes:
  data:
`

func TestParseServicesAndBuildHeuristic(t *testing.T) {
	f, err := Parse(write(t, realistic))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Services) != 3 {
		t.Fatalf("got %d services, want 3", len(f.Services))
	}
	// D-10: build present ⇒ application candidate. redis is image-only.
	app := f.AppCandidates()
	if len(app) != 2 || app[0] != "api" || app[1] != "web" {
		t.Errorf("AppCandidates() = %v, want [api web]", app)
	}
}

func TestParseStopsAtTheNextTopLevelKey(t *testing.T) {
	// `volumes:` must not be read as a service.
	f, _ := Parse(write(t, realistic))
	for _, s := range f.Services {
		if s.Name == "data" || s.Name == "volumes" {
			t.Fatalf("parsed %q as a service", s.Name)
		}
	}
}

func TestParsePublishedPorts(t *testing.T) {
	f, _ := Parse(write(t, realistic))
	ports := f.PublishedPorts()
	if len(ports) != 3 {
		t.Fatalf("got %d ports, want 3: %+v", len(ports), ports)
	}
	want := map[string][2]int{
		"api":   {8000, 8000},
		"web":   {8080, 80},
		"redis": {6379, 6379},
	}
	for _, p := range ports {
		w, ok := want[p.Service]
		if !ok {
			t.Fatalf("unexpected service %q", p.Service)
		}
		if p.Host != w[0] || p.Container != w[1] {
			t.Errorf("%s = %d:%d, want %d:%d", p.Service, p.Host, p.Container, w[0], w[1])
		}
	}
}

func TestParseShortForms(t *testing.T) {
	tests := []struct {
		name       string
		entry      string
		ok         bool
		host, cont int
		bind       string
	}{
		{"host:container", "8000:8000", true, 8000, 8000, ""},
		{"bind:host:container", "127.0.0.1:4000:8000", true, 4000, 8000, "127.0.0.1"},
		{"tcp suffix", "4000:8000/tcp", true, 4000, 8000, ""},
		// Container-only means Docker picks a random host port, so there is
		// nothing to collide over and nothing to remap.
		{"container only", "8000", false, 0, 0, ""},
		// Ranges and udp are real but rare. Reporting beats guessing: a wrong
		// remap yields a stack that looks isolated and is not.
		{"range rejected", "3000-3005:3000-3005", false, 0, 0, ""},
		{"udp rejected", "5000:5000/udp", false, 0, 0, ""},
		{"garbage rejected", "not:a:port", false, 0, 0, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, ok := parseShort("svc", tc.entry)
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v", ok, tc.ok)
			}
			if !ok {
				return
			}
			if p.Host != tc.host || p.Container != tc.cont || p.Bind != tc.bind {
				t.Errorf("got %+v, want host=%d cont=%d bind=%q", p, tc.host, tc.cont, tc.bind)
			}
		})
	}
}

// An entry we cannot read must be reported, never dropped. Silently skipping it
// would produce a stack advertised as isolated with one port still colliding.
func TestUnparseablePortsAreSurfaced(t *testing.T) {
	f, _ := Parse(write(t, `services:
  api:
    build: .
    ports:
      - "3000-3005:3000-3005"
      - "8000:8000"
`))
	if got := len(f.PublishedPorts()); got != 1 {
		t.Fatalf("got %d parseable ports, want 1", got)
	}
	un := f.Unparseable()
	if len(un["api"]) != 1 || un["api"][0] != "3000-3005:3000-3005" {
		t.Fatalf("Unparseable() = %v, want the range entry reported", un)
	}
}

func TestParseLongSyntax(t *testing.T) {
	f, _ := Parse(write(t, `services:
  api:
    build: .
    ports:
      - target: 8000
        published: 4000
`))
	ports := f.PublishedPorts()
	if len(ports) != 1 {
		t.Fatalf("got %d ports, want 1: %+v", len(ports), ports)
	}
	if ports[0].Host != 4000 || ports[0].Container != 8000 {
		t.Errorf("got %d:%d, want 4000:8000", ports[0].Host, ports[0].Container)
	}
}

func TestParseNoPorts(t *testing.T) {
	f, _ := Parse(write(t, "services:\n  worker:\n    build: .\n"))
	if len(f.PublishedPorts()) != 0 {
		t.Error("a service with no ports must publish none")
	}
	if len(f.AppCandidates()) != 1 {
		t.Error("a service with build: is still an app candidate without ports")
	}
}
