package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
)

// setTOMLKey sets one key in one section of project.toml, in place.
//
// The same rule as writeContract: never re-encode the whole file, because that
// deletes every comment the author and `circle init` wrote. A key that exists is
// rewritten on its own line, keeping the author's alignment; a missing key goes
// at the end of its section; a missing section goes at the end of the file.
//
// The result is parsed before it replaces the original. An edit that would
// leave the contract unreadable is refused and the file is untouched — a broken
// contract is the one state the gate cannot recover from on its own.
func setTOMLKey(path, section, key, literal string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	out := editTOML(string(b), section, key, literal)
	var probe map[string]any
	if _, err := toml.Decode(out, &probe); err != nil {
		return fmt.Errorf("refusing the edit, it would not parse: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(out), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func editTOML(text, section, key, literal string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	in, sectionAt, endAt := false, -1, -1
	for i := 0; i < len(lines); i++ {
		t := stripComment(strings.TrimSpace(lines[i]))
		if strings.HasPrefix(t, "[") {
			if in && endAt < 0 {
				endAt = i
			}
			in = headerName(t) == section
			if in {
				sectionAt = i
			}
			continue
		}
		if !in || !strings.Contains(t, "=") {
			continue
		}
		if unquote(strings.TrimSpace(strings.SplitN(t, "=", 2)[0])) != key {
			continue
		}
		// Replace the value, including any lines a multi-line array or string
		// continues onto, so nothing of the old value is left behind.
		last := valueEnd(lines, i)
		repl := lines[i][:strings.Index(lines[i], "=")+1] + " " + literal
		lines = append(lines[:i], append([]string{repl}, lines[last+1:]...)...)
		return strings.Join(lines, "\n") + "\n"
	}
	entry := fmt.Sprintf("%s = %s", tomlKey(key), literal)
	switch {
	case sectionAt < 0:
		lines = append(lines, "", "["+section+"]", entry)
	case endAt < 0:
		lines = append(lines, entry)
	default:
		at := endAt
		for at > sectionAt+1 && strings.TrimSpace(lines[at-1]) == "" {
			at--
		}
		lines = append(lines[:at], append([]string{entry}, lines[at:]...)...)
	}
	return strings.Join(lines, "\n") + "\n"
}

// headerName normalises "[ quality.test ]  # gates" to "quality.test".
// Array-of-tables headers ("[[x]]") never match a plain section.
func headerName(t string) string {
	if strings.HasPrefix(t, "[[") {
		return ""
	}
	t = strings.TrimSuffix(strings.TrimPrefix(t, "["), "]")
	parts := strings.Split(t, ".")
	for i := range parts {
		parts[i] = unquote(strings.TrimSpace(parts[i]))
	}
	return strings.Join(parts, ".")
}

// stripComment drops a trailing comment that is not inside a string.
func stripComment(t string) string {
	inStr := rune(0)
	for i, r := range t {
		switch {
		case inStr != 0 && r == inStr:
			inStr = 0
		case inStr == 0 && (r == '"' || r == '\''):
			inStr = r
		case inStr == 0 && r == '#':
			return strings.TrimSpace(t[:i])
		}
	}
	return t
}

// valueEnd finds the last line of the value starting on line i: a multi-line
// array runs to its closing bracket, a multi-line string to its closing quotes.
func valueEnd(lines []string, i int) int {
	v := strings.TrimSpace(strings.SplitN(lines[i], "=", 2)[1])
	switch {
	case strings.HasPrefix(v, `"""`) || strings.HasPrefix(v, `'''`):
		delim := v[:3]
		if strings.Count(v, delim) >= 2 {
			return i
		}
		for j := i + 1; j < len(lines); j++ {
			if strings.Contains(lines[j], delim) {
				return j
			}
		}
	case strings.HasPrefix(v, "["):
		depth := strings.Count(stripComment(v), "[") - strings.Count(stripComment(v), "]")
		for j := i + 1; j < len(lines) && depth > 0; j++ {
			t := stripComment(lines[j])
			depth += strings.Count(t, "[") - strings.Count(t, "]")
			if depth <= 0 {
				return j
			}
		}
	}
	return i
}

// tomlKey quotes a key that is not a bare TOML key.
func tomlKey(k string) string {
	for _, r := range k {
		if !(r == '_' || r == '-' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return fmt.Sprintf("%q", k)
		}
	}
	return k
}

func unquote(k string) string { return strings.Trim(k, `"'`) }
