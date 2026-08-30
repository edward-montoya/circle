#!/usr/bin/env bash
# Shim. All logic lives in circle_preflight.py.
#
# Why: macOS ships bash 3.2, which mis-parses heredocs inside command
# substitution when they contain backticks or apostrophes. Phase 0 targets a
# stranger's machine, so the shell layer stays trivial on purpose.
#
# Exit codes are the API: 0 valid, 2 invalid (blocking), 3 precondition missing.
exec python3 "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/circle_preflight.py" "$@"
