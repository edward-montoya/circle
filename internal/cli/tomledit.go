package cli

import (
	"fmt"
	"os"
	"strings"
)

// setTOMLKey sets one key in one section of project.toml, in place.
//
// The same rule as writeContract: never re-encode the whole file, because that
// deletes every comment the author and `circle init` wrote. A key that exists is
// rewritten on its own line, keeping the author's alignment; a missing key goes
// at the end of its section; a missing section goes at the end of the file.
func setTOMLKey(path, section, key, literal string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	header := "[" + section + "]"
	in, sectionAt, endAt := false, -1, -1
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "[") {
			if in && endAt < 0 {
				endAt = i
			}
			in = t == header
			if in {
				sectionAt = i
			}
			continue
		}
		if !in {
			continue
		}
		if k := strings.TrimSpace(strings.SplitN(t, "=", 2)[0]); strings.Contains(t, "=") && unquote(k) == key {
			lines[i] = line[:strings.Index(line, "=")+1] + " " + literal
			return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
		}
	}
	entry := fmt.Sprintf("%s = %s", tomlKey(key), literal)
	switch {
	case sectionAt < 0:
		lines = append(lines, "", header, entry)
	case endAt < 0:
		lines = append(lines, entry)
	default:
		// Insert before the blank lines that separate this section from the next.
		at := endAt
		for at > sectionAt+1 && strings.TrimSpace(lines[at-1]) == "" {
			at--
		}
		lines = append(lines[:at], append([]string{entry}, lines[at:]...)...)
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
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
