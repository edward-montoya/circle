package contract

import (
	"fmt"
	"io/fs"
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

	// Incubating means the repository has nothing to run and nothing to prove
	// *yet* — a project on day zero rather than one with a broken contract.
	//
	// The distinction matters because the two look identical to a naive check
	// and deserve opposite treatment. A half-declared contract is a lie and must
	// block; an undeclared one on an empty repository is just the truth, and
	// blocking it would mean a new project cannot write its first file. That
	// dead end is worse than no gate at all, because the user's only way out is
	// --force from minute one, which teaches them the gate is noise.
	Incubating bool
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
//
// Incubation is deliberately not consulted here. It already expresses itself by
// downgrading the execution and quality checks to warnings, so a day-zero
// repository has no failures to begin with. Consulting it a second time as a
// veto — which the callers used to do — let an incubating repository fail a
// knowledge check and still report success, because "nothing to run yet"
// silenced a section that has nothing to do with running.
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

// composeOnDisk reports whether the repository has a compose file at all.
//
// This is what separates "you forgot to declare it" from "there is nothing to
// declare". The first is an error; the second is a new project.
func composeOnDisk(r *Repo) string {
	for _, name := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"} {
		if _, err := os.Stat(r.Path(name)); err == nil {
			return name
		}
	}
	return ""
}

// Validate runs every contract check against the repository.
func Validate(r *Repo) Result {
	var res Result
	c := r.Contract

	// Nothing declared, and nothing on disk to declare: a project that has not
	// started rather than one that is broken.
	//
	// This flag softens the severity of the execution and quality checks below;
	// it is NOT a veto over the result. An incubating repository reports those
	// gaps as warnings and therefore does not block on its own — see Blocked.
	res.Incubating = c.Execution.Compose == "" &&
		composeOnDisk(r) == "" &&
		len(c.Quality.Gates()) == 0

	validateExecution(r, c, &res)
	validateQuality(c, &res)
	validateKnowledge(r, c, &res)
	validateDefinitionDrift(r, &res)
	validateForce(r, &res)
	return res
}

func validateExecution(r *Repo, c domain.Contract, res *Result) {
	ex := c.Execution
	var services []string

	switch {
	case ex.Compose == "":
		if found := composeOnDisk(r); found != "" {
			// The repo has one and the contract does not mention it. That is a
			// forgotten declaration, and it blocks.
			res.add(Fail, "execution", "compose", found+" exists but is not declared",
				fmt.Sprintf("set execution.compose = %q", found))
			break
		}
		if res.Incubating {
			res.add(Warn, "execution", "compose", "nothing to run yet",
				"declare it when the project has something that starts")
			break
		}
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
		if res.Incubating {
			break // no services because there is no code yet
		}
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
		switch {
		case strings.TrimSpace(kv.v) != "":
			res.add(OK, "execution", kv.k, kv.v)
		case res.Incubating:
			// Nothing to start, so nothing to declare.
		default:
			res.add(Fail, "execution", kv.k, "not declared", "add execution."+kv.k)
		}
	}
}

func validateQuality(c domain.Contract, res *Result) {
	q := c.Quality
	gates := q.Gates()
	if len(gates) == 0 {
		if res.Incubating {
			res.add(Warn, "quality", "gates", "nothing to prove yet",
				"declare a gate as soon as there is a first test")
			return
		}
		// This blocks even on a young project, and the message has to earn that.
		// Without a declared gate no task can name a verification, so no task can
		// be created, so the brief gate never engages — the framework would be
		// installed and inert. Saying "add a gate" without saying why reads as
		// bureaucracy.
		res.add(Fail, "quality", "gates",
			"nothing can prove this project works",
			"tasks cannot declare a verification without a gate. Add one under "+
				"[quality] — even `test:unit = \"<your test command>\"` is enough to start")
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

	// A floor with nothing measuring it is the report this framework exists to
	// refuse. It used to tick green on `min` alone, so a project could declare
	// 95% and sit at zero forever.
	// Labelled "coverage floor", not "coverage": a declared command is itself a
	// gate, and the loop above already prints a "coverage" line for it. Two
	// checks sharing a label read as the same thing reported twice.
	switch {
	case q.Coverage.Min > 0 && q.Coverage.Measured():
		res.add(OK, "quality", "coverage floor",
			fmt.Sprintf("%d%% · proven by the coverage gate", q.Coverage.Min))
	case q.Coverage.Min > 0:
		res.add(Warn, "quality", "coverage floor",
			fmt.Sprintf("%d%% declared, but nothing measures it", q.Coverage.Min),
			"add command to [quality.coverage] — one that exits non-zero below the floor")
	case q.Coverage.Measured():
		res.add(Warn, "quality", "coverage floor", "measured, but no floor to compare against",
			"add min to [quality.coverage]")
	case !res.Incubating:
		res.add(Warn, "quality", "coverage floor", "no floor configured",
			"add [quality.coverage] with min = <percent> and command = <how to prove it>")
	}
}

// resolveKnowledge counts the documents a registered path actually resolves to,
// and reports whether anything of that name exists at all.
//
// The count is of files, never of directories. `filepath.Glob("docs/")` returns
// the directory itself, so the previous implementation reported an empty docs
// folder as "→ 1" — a registry with nothing in it wearing a checkmark, which is
// the exact failure the drift checker exists to prevent one level up.
func resolveKnowledge(r *Repo, pattern string) (files int, exists bool) {
	matches, _ := filepath.Glob(r.Path(pattern))
	if len(matches) == 0 {
		if _, err := os.Stat(r.Path(pattern)); err != nil {
			return 0, false
		}
		matches = []string{r.Path(pattern)}
	}
	for _, m := range matches {
		st, err := os.Stat(m)
		if err != nil {
			continue
		}
		if !st.IsDir() {
			files++
			continue
		}
		// A registered directory means the documents beneath it, at any depth.
		_ = filepath.WalkDir(m, func(_ string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				files++
			}
			return nil
		})
	}
	return files, true
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
			n, exists := resolveKnowledge(r, p)
			switch {
			case !exists:
				// Nothing of that name. The registration is a lie and blocks.
				res.add(Fail, "knowledge", cls.Name, p+" → 0 matches",
					"repoint it or remove it")
			case n == 0:
				// The path is there and holds nothing. An empty registry is
				// honest — it is the checkmark on one that made this worth
				// fixing, because `init` writes docs = ["docs/"] before any
				// document exists, and a bare Stat on the directory reported it
				// satisfied. Advisory, not blocking: a young project should not
				// be unable to write its first file over an empty docs folder.
				hit++
				res.add(Warn, "knowledge", cls.Name, p+" → 0 files",
					"add a document, or drop the registration")
			default:
				hit++
				res.add(OK, "knowledge", cls.Name, fmt.Sprintf("%s → %d", p, n))
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

// DriftChecker is injected so contract does not import knowledge, which imports
// contract. It stays nil in tests that do not care about drift.
var DriftChecker func(*Repo) (int, string)

// validateDefinitionDrift reports definitions the code contradicts.
//
// Advisory, never blocking. Documents drift constantly and a false positive that
// stopped work would teach people to ignore the whole report — but a green tick
// on a definition that says Go over a Node repository is the framework actively
// misdirecting the agent, so silence is not an option either.
func validateDefinitionDrift(r *Repo, res *Result) {
	if DriftChecker == nil {
		return
	}
	n, summary := DriftChecker(r)
	if n == 0 {
		return
	}
	res.add(Warn, "knowledge", "definitions drift",
		fmt.Sprintf("%d contradiction(s): %s", n, summary),
		"circle knowledge verify — a stale definition misdirects the plan")
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
		"the status score is capped until `circle preflight --clear-force`")
}

func summarise(xs []string) string {
	if len(xs) <= 4 {
		return strings.Join(xs, ", ")
	}
	return fmt.Sprintf("%s +%d", strings.Join(xs[:4], ", "), len(xs)-4)
}
