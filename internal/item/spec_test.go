package item

import (
	"strings"
	"testing"
)

const goodSpec = `# Cancel an order

## Problem
Customers cannot cancel.

## Solution
They can, until it ships.

## Behaviours
1. As a customer, I want to cancel an unshipped order, so that I am not charged.
2. As a customer, I want a shipped order to refuse cancellation, so that nothing is lost.

## Seams
| Seam | Behaviours | Gate | Misses |
|---|---|---|---|
| ` + "`POST /orders/:id/cancel`" + ` | 1, 2 | test:integration | UI |

## Decisions
- Cancellation is a state, not a deletion.

## Out of scope
- Partial cancellation.

## Risks
- Leans on the shipping status being current.
`

var gates = map[string]bool{"test:unit": true, "test:integration": true}

func TestCheckSpecAcceptsACompleteSpec(t *testing.T) {
	seams, problems := CheckSpec(goodSpec, gates)
	if len(problems) != 0 {
		t.Fatalf("unexpected problems: %+v", problems)
	}
	if len(seams) != 1 || seams[0].Gate != "test:integration" || len(seams[0].Behaviours) != 2 {
		t.Fatalf("seams: %+v", seams)
	}
}

func TestCheckSpecRejectsAnUndeclaredGate(t *testing.T) {
	_, problems := CheckSpec(strings.Replace(goodSpec, "test:integration", "test:e2e", 1), gates)
	if len(problems) != 1 || !strings.Contains(problems[0].Reason, `"test:e2e"`) {
		t.Fatalf("problems: %+v", problems)
	}
}

func TestCheckSpecRejectsAnUnprovedBehaviourAndEmptySections(t *testing.T) {
	spec := strings.Replace(goodSpec, "| 1, 2 |", "| 1 |", 1)
	spec = strings.Replace(spec, "- Partial cancellation.\n", "<!-- nothing yet -->\n", 1)
	_, problems := CheckSpec(spec, gates)
	var reasons []string
	for _, p := range problems {
		reasons = append(reasons, p.Reason)
	}
	joined := strings.Join(reasons, "\n")
	if !strings.Contains(joined, "behaviour 2 is proved by no seam") || !strings.Contains(joined, `"Out of scope"`) {
		t.Fatalf("problems:\n%s", joined)
	}
}

func TestCheckSpecAllowsManualSeams(t *testing.T) {
	spec := strings.Replace(goodSpec, "test:integration", "manual (copy review)", 1)
	if _, problems := CheckSpec(spec, gates); len(problems) != 0 {
		t.Fatalf("manual seam rejected: %+v", problems)
	}
}

func TestCheckSpecIgnoresUnfilledTemplateLines(t *testing.T) {
	spec := strings.Replace(goodSpec, "Customers cannot cancel.", "<From the user's perspective.>", 1)
	if _, problems := CheckSpec(spec, gates); len(problems) != 1 {
		t.Fatalf("a template placeholder should leave Problem empty: %+v", problems)
	}
}
