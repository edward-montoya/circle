package cli

import (
	"path/filepath"
	"testing"
)

// `brief verify` reported .circle/tasks.jsonl, .circle/timelines/ and the event
// log as changes outside the approved radius. Circle wrote all three, during the
// commands the user had just run. Warning somebody about files the tool itself
// created is how a warning stops being read — and this is the one that must be.
func TestIsCircleState(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{filepath.Join(".circle", "tasks.jsonl"), true},
		{filepath.Join(".circle", "timelines", "default.json"), true},
		{filepath.Join(".circle", "runtime", "events", "events.jsonl"), true},
		{filepath.Join(".circle", "items", "default", "approval.json"), true},
		{filepath.Join(".circle", "project.toml"), true},
		{".circle", true},

		{"internal/httpx/server.go", false},
		{"README.md", false},
		// Not Circle's directory, merely a name that starts the same way.
		{".circleci/config.yml", false},
		{"docs/.circle-notes.md", false},
	}
	for _, c := range cases {
		if got := isCircleState(c.path); got != c.want {
			t.Errorf("isCircleState(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

// The two predicates answer different questions and must not be collapsed:
// writableState asks whether the agent's editing tools may change a file, and
// the answer for approval.json is no. isCircleState asks whether Circle wrote
// it, and there the answer is yes.
func TestWritableStateIsNarrowerThanCircleState(t *testing.T) {
	approval := filepath.Join(".circle", "items", "default", "approval.json")
	if writableState(approval) {
		t.Error("the approval record must not be writable through the gate")
	}
	if !isCircleState(approval) {
		t.Error("the approval record is still Circle's own state for reporting")
	}

	runtime := filepath.Join(".circle", "runtime", "events", "events.jsonl")
	if !writableState(runtime) {
		t.Error("volatile bookkeeping must stay writable")
	}
	if !isCircleState(runtime) {
		t.Error("volatile bookkeeping is Circle's own state")
	}
}
