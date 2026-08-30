package contract

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/edwardmontoya/circle/internal/domain"
)

// Severity decides whether preflight blocks.
type Severity string

const (
	OK   Severity = "ok"
	Warn Severity = "warn"
	Fail Severity = "fail"
)

// Check is one line of the preflight report.
//
// Every failing check carries the command that fixes it. The risk table
// predicts developers route around a gate that only says no, so the output is a
// ratio with a fix list rather than a binary rejection.
type Check struct {
	Severity Severity
	Section  string
	Label    string
	Detail   string
	Fix      string
}

// Result is the whole report.
type Result struct {
	Checks []Check
	Forced bool
}

// add records a check. fix is variadic so the common "nothing to fix" case
// reads cleanly at the call site.
func (r *Result) add(s Severity, section, label, detail string, fix ...string) {
	f := ""
	if len(fix) > 0 {
		f = fix[0]
	}
	r.Checks = append(r.Checks, Check{s, section, label, detail, f})
}

func (r Result) Count(s Severity) int {
	n := 0
	for _, c := range r.Checks {
		if c.Severity == s {
			n++
		}
	}
	return n
}

// Blocked reports whether any check failed. This is what maps to exit 2.
func (r Result) Blocked() bool { return r.Count(Fail) > 0 }

var serviceKey = regexp.MustCompile(`^  ([A-Za-z0-9._-]+):\s*$`)

// composeServices extracts service keys from a compose file.
//
// Deliberately not a YAML dependency: this must run on a stranger's machine
// with nothing installed. Two-space indentation under `services:` covers every
// compose file in the wild we have seen; anything stranger is reported rather
// than silently mis-parsed.
func composeServices(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []string
	inside := false
	for _, line := range strings.Split(string(b), "\n") {
		trimmed := strings.TrimRight(line, "\r")
		if strings.TrimSpace(trimmed) == "services:" && !strings.HasPrefix(trimmed, " ") {
			inside = true
			continue
		}
		if !inside {
			continue
		}
		if trimmed != "" && !strings.HasPrefix(trimmed, " ") {
			break // next top-level key
		}
		if m := serviceKey.FindStringSubmatch(trimmed); m != nil {
			out = append(out, m[1])
		}
	}
	return out, nil
}

// binaryOf finds the executable a gate command will actually run, stripping
// leading `cd <dir> &&` chains and environment assignments.
func binaryOf(cmd string) string {
	c := regexp.MustCompile(`^(cd\s+\S+\s*&&\s*)+`).ReplaceAllString(cmd, "")
	for _, tok := range strings.Fields(c) {
		if !strings.Contains(tok, "=") {
			return tok
		}
	}
	return ""
}

// Validate runs every contract check against the repository.
func Validate(r *Repo) Result {
	var res Result
	c := r.Contract
	validateExecution(r, c, &res)
	validateQuality(c, &res)
	validateKnowledge(r, c, &res)
	validateForce(r, &res)
	return res
}

func validateExecution(r *Repo, c domain.Contract, res *Result) {
	ex := c.Execution
	var services []string

	switch {
	case ex.Compose == "":
		res.add(Fail, "execution", "compose", "no compose file declared",
			"add execution.compose")
	default:
		p := r.Path(ex.Compose)
		if _, err := os.Stat(p); err != nil {
			res.add(Fail, "execution", "compose", ex.Compose+" does not exist",
				"point execution.compose at a real file")
		} else if s, err := composeServices(p); err != nil {
			res.add(Fail, "execution", "compose", "unreadable: "+err.Error(), "")
		} else {
			services = s
			res.add(OK, "execution", "compose",
				fmt.Sprintf("%s · %d services", ex.Compose, len(services)))
		}
	}

	switch {
	case len(ex.AppServices) == 0:
		res.add(Fail, "execution", "app_services", "none declared",
			"list the containers that hold your code")
	case len(services) > 0:
		var unknown []string
		known := map[string]bool{}
		for _, s := range services {
			known[s] = true
		}
		for _, s := range ex.AppServices {
			if !known[s] {
				unknown = append(unknown, s)
			}
		}
		if len(unknown) > 0 {
			res.add(Fail, "execution", "app_services",
				"not in compose: "+strings.Join(unknown, ", "),
				"real services: "+strings.Join(services, ", "))
		} else {
			res.add(OK, "execution", "app_services", summarise(ex.AppServices))
		}
	}

	for _, kv := range []struct{ k, v string }{{"up", ex.Up}, {"down", ex.Down}} {
		if strings.TrimSpace(kv.v) == "" {
			res.add(Fail, "execution", kv.k, "not declared", "add execution."+kv.k)
		} else {
			res.add(OK, "execution", kv.k, kv.v)
		}
	}
}

func validateQuality(c domain.Contract, res *Result) {
	q := c.Quality
	gates := q.Gates()
	if len(gates) == 0 {
		res.add(Fail, "quality", "gates", "no gates declared",
			"add at least one runnable gate")
		return
	}

	// A gate whose binary is absent is not runnable *yet* — different from one
	// that was never declared. Phase 0 finding: without a bootstrap step,
	// preflight reports a green contract a fresh clone cannot run.
	var missing []string
	for _, g := range gates {
		bin := binaryOf(g.Command)
		if bin != "" {
			if _, err := exec.LookPath(bin); err == nil {
				res.add(OK, "quality", g.Name, g.Command)
				continue
			}
		}
		missing = append(missing, g.Name)
		fix := "declare quality.bootstrap so a fresh clone can install it"
		if q.Bootstrap != "" {
			fix = "run: " + q.Bootstrap
		}
		res.add(Warn, "quality", g.Name, bin+" not runnable yet", fix)
	}

	if len(missing) > 0 && q.Bootstrap == "" {
		res.add(Fail, "quality", "bootstrap",
			fmt.Sprintf("%d gate(s) unrunnable and no bootstrap", len(missing)),
			"add quality.bootstrap with the install command")
	}

	if q.Coverage.Min > 0 {
		res.add(OK, "quality", "coverage", fmt.Sprintf("floor %d%%", q.Coverage.Min))
	} else {
		res.add(Warn, "quality", "coverage", "no floor configured",
			"the status score cannot use what is not measured")
	}
}

func validateKnowledge(r *Repo, c domain.Contract, res *Result) {
	total, hit := 0, 0
	for _, cls := range c.Knowledge.Classes() {
		if len(cls.Paths) == 0 {
			res.add(Warn, "knowledge", cls.Name, "empty — nothing registered",
				fmt.Sprintf("circle knowledge add <path> --as %s", cls.Name))
			continue
		}
		for _, p := range cls.Paths {
			total++
			matches, _ := filepath.Glob(r.Path(p))
			if len(matches) == 0 {
				if _, err := os.Stat(r.Path(p)); err == nil {
					matches = []string{p}
				}
			}
			if len(matches) > 0 {
				hit++
				res.add(OK, "knowledge", cls.Name, fmt.Sprintf("%s → %d", p, len(matches)))
			} else {
				res.add(Fail, "knowledge", cls.Name, p+" → 0 matches",
					"repoint it or remove it")
			}
		}
	}
	if total > 0 {
		s := OK
		if hit != total {
			s = Fail
		}
		res.add(s, "knowledge", "paths", fmt.Sprintf("%d of %d resolve", hit, total))
	}
}

// validateForce surfaces a recorded bypass. A --force with no consequence is a
// socially acceptable way to skip the product, so it is reported here and caps
// the status score.
func validateForce(r *Repo, res *Result) {
	b, err := os.ReadFile(r.RuntimeDir("forced"))
	if err != nil {
		return
	}
	res.Forced = true
	res.add(Warn, "preflight", "forced", strings.TrimSpace(string(b)),
		"the status score is capped and PR creation is blocked until this is cleared")
}

func summarise(xs []string) string {
	if len(xs) <= 4 {
		return strings.Join(xs, ", ")
	}
	return fmt.Sprintf("%s +%d", strings.Join(xs[:4], ", "), len(xs)-4)
}
