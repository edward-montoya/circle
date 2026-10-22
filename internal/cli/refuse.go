package cli

import (
	"fmt"
	"os"
	"strings"
)

// refuseSkill stops a skill whose precondition does not hold.
//
// Interactive, a non-zero exit from an injected command aborts the skill and
// Claude Code shows the user the failure — the hard stop is right. Headless
// (`claude -p`, the SDK), the same abort is silent: zero turns, an empty result,
// and an orchestrating caller learns nothing. There the refusal is injected
// instead, as text the model is told to report verbatim before stopping. Nothing
// becomes writable either way: the write hook enforces the gate on every write,
// whatever the model does with the text.
func refuseSkill(e Env, reason string) int {
	if headless() {
		fmt.Fprintf(e.Stdout, "CIRCLE REFUSED — %s\n\n", reason)
		fmt.Fprintln(e.Stdout, "Report the line above to the user verbatim and stop. Do not carry out any step of this skill.")
		return ExitOK
	}
	fmt.Fprintln(e.Stderr, "circle: "+reason)
	return ExitGate
}

// headless reports whether refusals should be injected as text: Claude Code's
// SDK entrypoints (`claude -p` sets sdk-cli), or an orchestrator that asked with
// CIRCLE_REFUSE=text. The entrypoint alone cannot be trusted — a child process
// inherits its parent's value, and a headless run started from an interactive
// session reports the parent's surface — so orchestrators should set the
// variable explicitly.
func headless() bool {
	if v := os.Getenv("CIRCLE_REFUSE"); v != "" {
		return v == "text"
	}
	return strings.HasPrefix(os.Getenv("CLAUDE_CODE_ENTRYPOINT"), "sdk")
}
