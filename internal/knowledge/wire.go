package knowledge

import (
	"sort"
	"strings"

	"github.com/edwardmontoya/circle/internal/brief"
	"github.com/edwardmontoya/circle/internal/contract"
)

// init wires drift detection into preflight.
//
// The indirection exists because contract cannot import knowledge — knowledge
// already imports contract. Registering from this side keeps the dependency
// pointing one way and lets contract's own tests run without it.
func init() {
	contract.DriftChecker = func(r *contract.Repo) (int, string) {
		fs, err := Verify(r)
		if err != nil || len(fs) == 0 {
			return 0, ""
		}
		claims := map[string]bool{}
		for _, f := range fs {
			claims[f.Claim] = true
		}
		names := make([]string, 0, len(claims))
		for c := range claims {
			names = append(names, c)
		}
		sort.Strings(names)
		if len(names) > 4 {
			names = append(names[:4], "…")
		}
		return len(fs), strings.Join(names, ", ")
	}
}

// init also lets the brief report contradicted definitions inline.
func init() {
	brief.Contradictions = func(r *contract.Repo) map[string][]string {
		fs, err := Verify(r)
		if err != nil || len(fs) == 0 {
			return nil
		}
		out := map[string][]string{}
		seen := map[string]bool{}
		for _, f := range fs {
			key := f.Document + "\x1f" + f.Claim
			if seen[key] {
				continue
			}
			seen[key] = true
			out[f.Document] = append(out[f.Document], f.Claim)
		}
		return out
	}
}
