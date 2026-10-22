package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/edwardmontoya/circle/internal/skill"
)

// Every skill the plugin ships conforms to the contract — above all, every
// circle command a skill names exists in this binary. A skill that injects a
// missing command aborts the moment someone invokes it.
func TestShippedSkillsConform(t *testing.T) {
	paths := skill.Find(filepath.Join("..", "..", "plugin", "skills"))
	if len(paths) < 10 {
		t.Fatalf("expected the plugin's skills, found %d", len(paths))
	}
	o := skillOptions()
	for _, p := range paths {
		s, err := skill.Parse(p)
		if err != nil {
			t.Error(err)
			continue
		}
		for _, pr := range skill.Validate(s, o) {
			if !pr.Warn {
				t.Errorf("%s", pr)
			}
		}
	}
}

func TestSkillValidateCatchesBrokenSkills(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		p := filepath.Join(dir, name, "SKILL.md")
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte(body), 0o644)
	}
	write("ghost", "---\ndescription: d\ndisable-model-invocation: true\nmetadata:\n  circle:\n    kind: orchestrator\n---\n\n```!\ncircle teleport now\n```\n")
	write("ungated", "---\ndescription: d\ndisable-model-invocation: true\nmetadata:\n  circle:\n    kind: orchestrator\n    requires: approved\n---\n\n```!\ncircle status\n```\n")
	write("linker", "---\ndescription: d\nmetadata:\n  circle:\n    kind: practice\n---\nSee [the rules](${CLAUDE_SKILL_DIR}/MISSING.md).\n")
		write("chatty", "---\ndescription: d\nmetadata:\n  circle:\n    kind: practice\n    produces: done\n---\nbody\n")
	code, out := run(t, dir, "skill", "validate", dir)
	if code != ExitGate {
		t.Fatalf("broken skills must fail validation:\n%s", out)
	}
	for _, want := range []string{"`circle teleport now` is not a circle command", "first injected block is not `circle item require`", "a practice never changes item state", "MISSING.md, which does not exist"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
}

// A scaffolded skill passes validation as written: extensions start inside the
// contract.
func TestSkillNewScaffoldsAConformingSkill(t *testing.T) {
	root := newGitRepo(t)
	mustRun(t, root, "skill", "new", "deploy-check", "--description", "Checks the deploy. Use before release.",
		"--requires", "reviewing", "--produces", "done")
	out := mustRun(t, root, "skill", "validate", filepath.Join(root, ".claude", "skills"))
	if !strings.Contains(out, "1 skill(s) conform") {
		t.Fatalf("scaffold should validate:\n%s", out)
	}
}
