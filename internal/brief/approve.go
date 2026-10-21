package brief

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/edwardmontoya/circle/internal/contract"
	"github.com/edwardmontoya/circle/internal/domain"
)

// Approval is the recorded gate event.
//
// Not a vibe and not a nod: a durable record naming who approved what. It is
// bound to a plan hash, so a changed plan invalidates it automatically rather
// than carrying a stale approval forward (D-15).
type Approval struct {
	Item         string    `json:"item"`
	Approver     string    `json:"approver"`
	Author       string    `json:"author"`
	PlanHash     string    `json:"plan_hash"`
	Radius       []string  `json:"radius"`
	At           time.Time `json:"at"`
	Note         string    `json:"note,omitempty"`
	SelfApproved bool      `json:"self_approved"`

	// Valid is computed at load time against the current plan; it is never
	// persisted as true and then trusted.
	Valid  bool   `json:"-"`
	Reason string `json:"-"`
}

// Rejection records a refusal, so a rejected plan is visible rather than merely
// absent.
type Rejection struct {
	Item     string    `json:"item"`
	By       string    `json:"by"`
	Reason   string    `json:"reason"`
	PlanHash string    `json:"plan_hash"`
	At       time.Time `json:"at"`
}

func itemDir(r *contract.Repo, item string) string {
	return r.StateDir("items", item)
}

func approvalPath(r *contract.Repo, item string) string {
	return filepath.Join(itemDir(r, item), "approval.json")
}

// ErrNoApproval means nothing has been approved for this item.
var ErrNoApproval = errors.New("no approval recorded")

// LoadApproval reads the approval and checks it against the live plan.
//
// The validity check happens here rather than at write time on purpose: an
// approval that was true yesterday says nothing about a plan that changed
// since, and the gate must ask the question every time it is consulted.
func LoadApproval(r *contract.Repo, item string, currentHash string) (*Approval, error) {
	b, err := os.ReadFile(approvalPath(r, item))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoApproval
		}
		return nil, err
	}
	var a Approval
	if err := json.Unmarshal(b, &a); err != nil {
		return nil, err
	}
	switch {
	case currentHash == "":
		a.Valid = true // caller does not care about drift
	case a.PlanHash != currentHash:
		a.Valid = false
		a.Reason = fmt.Sprintf(
			"the plan changed since approval (approved %s, current %s)", a.PlanHash, currentHash)
	default:
		a.Valid = true
	}
	return &a, nil
}

// Approve records the gate event.
//
// requireSecond enforces [gate] require_second_approver. Self-approval is
// otherwise permitted but always recorded as such: the person who prompted the
// agent approving their own plan is a formality, not comprehension (D-29).
func Approve(r *contract.Repo, b domain.Brief, approver, note string, allowSelf, requireSecond bool) (*Approval, error) {
	if strings.TrimSpace(approver) == "" {
		return nil, errors.New("no approver identity: pass --approver or set git config user.email")
	}
	self := b.Author != "" && approver == b.Author
	if self {
		switch {
		case requireSecond:
			return nil, fmt.Errorf(
				"%s wrote this plan and [gate] require_second_approver is set; another reviewer must approve it", approver)
		case !allowSelf:
			return nil, fmt.Errorf(
				"%s wrote this plan. Self-approval is permitted but must be explicit: pass --allow-self", approver)
		}
	}

	a := Approval{
		Item: b.Item, Approver: approver, Author: b.Author,
		PlanHash: b.PlanHash, Radius: b.Radius,
		At: time.Now().UTC(), Note: note, SelfApproved: self,
	}
	if err := os.MkdirAll(itemDir(r, b.Item), 0o755); err != nil {
		return nil, err
	}
	raw, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(approvalPath(r, b.Item), append(raw, '\n'), 0o644); err != nil {
		return nil, err
	}
	// The radius file is what the PreToolUse gate reads on every write. Kept
	// separate and tiny so the hot path never parses the whole approval.
	if err := writeRadius(r, b.Item, b.Radius); err != nil {
		return nil, err
	}
	a.Valid = true
	return &a, nil
}

// Reject records a refusal and clears any radius, closing the gate again.
func Reject(r *contract.Repo, b domain.Brief, by, reason string) error {
	if strings.TrimSpace(reason) == "" {
		return errors.New("--reason is required: a rejection without one cannot be acted on")
	}
	rj := Rejection{Item: b.Item, By: by, Reason: reason, PlanHash: b.PlanHash, At: time.Now().UTC()}
	if err := os.MkdirAll(itemDir(r, b.Item), 0o755); err != nil {
		return err
	}
	raw, _ := json.MarshalIndent(rj, "", "  ")
	if err := os.WriteFile(filepath.Join(itemDir(r, b.Item), "rejection.json"),
		append(raw, '\n'), 0o644); err != nil {
		return err
	}
	_ = os.Remove(approvalPath(r, b.Item))
	return writeRadius(r, b.Item, nil)
}

// RadiusPath is the approved radius of one item.
//
// One file per item and no repository-wide copy: a shared "current radius"
// file was rewritten by every approval, so two items approved on two branches
// always conflicted on it at merge.
func RadiusPath(r *contract.Repo, item string) string {
	return filepath.Join(itemDir(r, item), "approved-radius")
}

func writeRadius(r *contract.Repo, item string, radius []string) error {
	p := RadiusPath(r, item)
	_ = os.Remove(r.StateDir("approved-radius")) // the old repository-wide copy
	if len(radius) == 0 {
		_ = os.Remove(p)
		return nil
	}
	return os.WriteFile(p, []byte(strings.Join(radius, "\n")+"\n"), 0o644)
}

// Pending lists items whose brief is generated but not approved.
func Pending(r *contract.Repo) ([]string, error) {
	entries, err := os.ReadDir(r.StateDir("items"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(itemDir(r, e.Name()), "brief.md")); err != nil {
			continue
		}
		if _, err := os.Stat(approvalPath(r, e.Name())); os.IsNotExist(err) {
			out = append(out, e.Name())
		}
	}
	return out, nil
}
