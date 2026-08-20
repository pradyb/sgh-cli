#!/bin/sh
#
# Runs the test suite with coverage instrumentation and fails if total
# coverage across the tracked packages drops below the required floor.
# Used identically by CI (.github/workflows/ci.yml) and by contributors
# locally before opening a pull request — see CONTRIBUTING.md.
#
# Usage:
#   ./scripts/check-coverage.sh

set -e

THRESHOLD=85
PROFILE=coverage.out

# Only colourise when stdout is a terminal, so redirecting to a file or
# piping into another tool does not embed escape codes in the output.
if [ -t 1 ]; then
	RED=$(printf '\033[31m')
	GREEN=$(printf '\033[32m')
	RESET=$(printf '\033[0m')
else
	RED=""
	GREEN=""
	RESET=""
fi

# cmd/tui is the one excluded package: an interactive Bubble Tea terminal
# app driven by a real TTY event loop, which can't be meaningfully tested
# without one. Everything else is tracked. See CONTRIBUTING.md.
packages=$(go list ./... | grep -v '/cmd/tui$')

# shellcheck disable=SC2086
go test -race -coverprofile="$PROFILE" -covermode=atomic $packages

pct=$(go tool cover -func="$PROFILE" | tail -1 | grep -oE '[0-9]+\.[0-9]+')

echo ""
echo "Total coverage: ${pct}% (floor: ${THRESHOLD}%)"

below=$(awk -v p="$pct" -v t="$THRESHOLD" 'BEGIN { print (p < t) }')
if [ "$below" = "1" ]; then
	printf '%sFAIL%s coverage %s%% is below the required %s%% floor\n' "$RED" "$RESET" "$pct" "$THRESHOLD"
	printf '  see CONTRIBUTING.md#coverage-expectations\n'
	printf '  for a line-by-line view: go tool cover -html=%s -o coverage.html\n' "$PROFILE"
	exit 1
fi

printf '%sOK%s coverage %s%% meets the %s%% floor\n' "$GREEN" "$RESET" "$pct" "$THRESHOLD"
