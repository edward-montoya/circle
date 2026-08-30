// Package cli is the command surface. Every command is one use case; no domain
// logic lives here.
package cli

// Exit codes are the API.
//
// `circle gate check` exiting 2 is simultaneously what makes a PreToolUse hook
// deny a write and what makes an injected !`command` abort a skill invocation.
// Anything richer than 0/2 exists so one binary can serve humans and hooks at
// once; the hook wrapper collapses the rest.
//
// EXIT 2, NEVER 1, for a contract failure: Claude Code treats exit 1 from
// search-like commands as a normal result.
const (
	ExitOK       = 0 // success / gate passed
	ExitError    = 1 // generic error
	ExitGate     = 2 // gate failed — deny the write, abort the skill
	ExitPrecond  = 3 // precondition missing: no .circle/, not a git repo
	ExitExternal = 4 // external tool missing or failed: git, docker, gh
	ExitQuality  = 5 // a quality gate command exited non-zero
	ExitDeclined = 6 // the user declined
	ExitDrift    = 7 // files touched outside the approved blast radius
)
