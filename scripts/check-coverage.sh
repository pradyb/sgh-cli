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

# Packages excluded from the coverage denominator, by exact import path.
#
#   cmd/tui                      an interactive Bubble Tea application driven
#                                by a real TTY event loop, which cannot be
#                                meaningfully tested without one.
#   internal/testutils           the mock GitHub server.
#   internal/service/servicetest the mock context builder.
#
# The last two exist only to support tests and are never linked into the
# binary. They are excluded because their reported 0% is a measurement
# artifact, not a fact: Go attributes coverage per test binary, and neither
# package has one of its own, so the thousands of times other packages' tests
# execute them earns no credit. Excluded is not untrusted -- a bug in the mock
# server does not hide, it fails tests across the whole repo at once. If
# either grows real branching logic it should get its own tests and come back
# into the count. See CONTRIBUTING.md.
#
# Listed individually on purpose. A pattern such as `grep -v test` would
# silently swallow future packages, and an exclusion list that grows by
# accident is how a coverage gate stops meaning anything.
EXCLUDED='github.com/pradyb/sgh-cli/cmd/tui
github.com/pradyb/sgh-cli/internal/testutils
github.com/pradyb/sgh-cli/internal/service/servicetest'

all_packages=$(go list ./...)

# A stale entry -- one naming a package that has been renamed or removed --
# would silently stop excluding anything, so fail loudly instead.
echo "$EXCLUDED" | while IFS= read -r pkg; do
	if ! echo "$all_packages" | grep -qxF "$pkg"; then
		printf 'stale exclusion in %s: %s no longer exists
' "$0" "$pkg" >&2
		exit 1
	fi
done || exit 1

packages=$(echo "$all_packages" | grep -vxF "$EXCLUDED")

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
