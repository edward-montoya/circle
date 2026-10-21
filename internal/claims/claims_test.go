package claims

import (
	"errors"
	"testing"
)

func TestTakeIsIdempotentForTheHolderAndExclusiveForOthers(t *testing.T) {
	dir := t.TempDir() // not a git repo: falls back to the runtime dir
	me := Claim{Task: "a", Holder: "dev", Session: "s1"}
	if err := Take(dir, me, false); err != nil {
		t.Fatal(err)
	}
	if err := Take(dir, me, false); err != nil {
		t.Fatalf("re-claiming your own task must be a no-op: %v", err)
	}
	other := Claim{Task: "a", Holder: "dev", Session: "s2"}
	var held HeldError
	if err := Take(dir, other, false); !errors.As(err, &held) || held.Claim.Session != "s1" {
		t.Fatalf("another session took a held task: %v", err)
	}
	if err := Take(dir, other, true); err != nil {
		t.Fatalf("steal: %v", err)
	}
	if got := All(dir)["a"].Session; got != "s2" {
		t.Fatalf("after steal the holder is %q", got)
	}
	if err := Release(dir, "a"); err != nil {
		t.Fatal(err)
	}
	if _, ok := All(dir)["a"]; ok {
		t.Fatal("released task is still held")
	}
}
