package timeline

import (
	"testing"

	"github.com/edwardmontoya/circle/internal/domain"
)

// The porcelain format is fixed-width: two status columns, a space, then the
// path. An off-by-one here silently truncates the first character of every
// path — which reads as a rendering glitch, not a parse bug, so it is easy to
// miss. A dotfile makes the truncation obvious, hence .circle/ throughout.
func TestParsePorcelain(t *testing.T) {
	tests := []struct {
		name string
		line string
		want domain.FileChange
	}{
		{"unstaged modify", " M .circle/timelines/default.json",
			domain.FileChange{Status: domain.Modified, Path: ".circle/timelines/default.json"}},
		{"staged modify", "M  src/app.ts",
			domain.FileChange{Status: domain.Modified, Path: "src/app.ts"}},
		{"untracked", "?? .circle/runtime/",
			domain.FileChange{Status: domain.Added, Path: ".circle/runtime/"}},
		{"staged add", "A  src/mw/rate-limit.ts",
			domain.FileChange{Status: domain.Added, Path: "src/mw/rate-limit.ts"}},
		{"deleted", " D old.txt",
			domain.FileChange{Status: domain.Deleted, Path: "old.txt"}},
		{"staged delete", "D  gone.txt",
			domain.FileChange{Status: domain.Deleted, Path: "gone.txt"}},
		{"rename keeps the new path", "R  src/middleware/index.ts -> src/mw/index.ts",
			domain.FileChange{Status: domain.Renamed, Path: "src/mw/index.ts"}},
		{"modified after staged add", "AM src/new.go",
			domain.FileChange{Status: domain.Added, Path: "src/new.go"}},
		{"quoted path with spaces", `?? "docs/my file.md"`,
			domain.FileChange{Status: domain.Added, Path: "docs/my file.md"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParsePorcelain(tc.line)
			if len(got) != 1 {
				t.Fatalf("got %d changes, want 1", len(got))
			}
			if got[0] != tc.want {
				t.Errorf("got %+v, want %+v", got[0], tc.want)
			}
		})
	}
}

func TestParsePorcelainSkipsShortLines(t *testing.T) {
	if got := ParsePorcelain("\n\n M\n"); len(got) != 0 {
		t.Fatalf("got %d changes, want 0", len(got))
	}
}

func TestParsePorcelainMultiline(t *testing.T) {
	out := " M .circle/timelines/default.json\n?? docs/probe.md\nA  src/x.go"
	got := ParsePorcelain(out)
	if len(got) != 3 {
		t.Fatalf("got %d changes, want 3", len(got))
	}
	if got[0].Path != ".circle/timelines/default.json" {
		t.Errorf("leading dot lost: %q", got[0].Path)
	}
}
