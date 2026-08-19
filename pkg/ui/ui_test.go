// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func TestStatusColor(t *testing.T) {
	tests := []struct {
		status string
		want   lipgloss.Color
	}{
		{"success", Green},
		{"approved", Green},
		{"clean", Green},
		{"open", Blue},
		{"merged", Purple},
		{"failure", Red},
		{"failed", Red},
		{"closed", Red},
		{"changes_requested", Red},
		{"in_progress", Yellow},
		{"queued", Yellow},
		{"pending", Yellow},
		{"waiting", Yellow},
		{"blocked", Orange},
		{"cancelled", Dimmed},
		{"skipped", Dimmed},
		{"dismissed", Dimmed},
		{"something-unknown", Subtle},
		{"", Subtle},

		// Matching is case-insensitive; the API returns mixed casing.
		{"SUCCESS", Green},
		{"Merged", Purple},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			if got := StatusColor(tt.status); got != tt.want {
				t.Errorf("StatusColor(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestStatusIcon(t *testing.T) {
	tests := []struct {
		conclusion string
		wantGlyph  string
	}{
		{"success", "✓"},
		{"failure", "✗"},
		{"cancelled", "⊘"},
		{"skipped", "○"},
		{"in_progress", "●"},
		{"queued", "◌"},
	}

	for _, tt := range tests {
		t.Run(tt.conclusion, func(t *testing.T) {
			got := StatusIcon(tt.conclusion)
			if !strings.Contains(got, tt.wantGlyph) {
				t.Errorf("StatusIcon(%q) = %q, want it to contain %q", tt.conclusion, got, tt.wantGlyph)
			}
		})
	}

	// Unrecognised values return empty so callers can skip rendering entirely.
	for _, unknown := range []string{"", "neutral", "SUCCESS", "timed_out"} {
		if got := StatusIcon(unknown); got != "" {
			t.Errorf("StatusIcon(%q) = %q, want empty string", unknown, got)
		}
	}
}

func TestShortSHA(t *testing.T) {
	tests := []struct {
		name string
		sha  string
		want string
	}{
		{"full sha truncated to 7", "1c33f19a2b3c4d5e6f7890abcdef1234567890ab", "1c33f19"},
		{"exactly 7 unchanged", "1c33f19", "1c33f19"},
		{"8 truncated", "1c33f19a", "1c33f19"},
		{"shorter than 7 unchanged", "1c33", "1c33"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShortSHA(tt.sha); got != tt.want {
				t.Errorf("ShortSHA(%q) = %q, want %q", tt.sha, got, tt.want)
			}
		})
	}
}

func TestRelativeTime(t *testing.T) {
	now := time.Now()
	rfc := func(d time.Duration) string {
		return now.Add(-d).Format(time.RFC3339)
	}

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty stays empty", "", ""},
		{"unparseable falls back to input", "not-a-timestamp", "not-a-timestamp"},
		{"future clamps to just now", now.Add(time.Hour).Format(time.RFC3339), "just now"},
		{"under a minute", rfc(30 * time.Second), "just now"},
		{"one minute singular", rfc(time.Minute + time.Second), "1 minute ago"},
		{"minutes plural", rfc(5 * time.Minute), "5 minutes ago"},
		{"one hour singular", rfc(time.Hour + time.Minute), "1 hour ago"},
		{"hours plural", rfc(5 * time.Hour), "5 hours ago"},
		{"one day singular", rfc(25 * time.Hour), "1 day ago"},
		{"days plural", rfc(5 * 24 * time.Hour), "5 days ago"},
		{"one month singular", rfc(31 * 24 * time.Hour), "1 month ago"},
		{"months plural", rfc(90 * 24 * time.Hour), "3 months ago"},
		{"one year singular", rfc(370 * 24 * time.Hour), "1 year ago"},
		{"years plural", rfc(3 * 370 * 24 * time.Hour), "3 years ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RelativeTime(tt.in); got != tt.want {
				t.Errorf("RelativeTime(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSortIndicator(t *testing.T) {
	tests := []struct {
		name   string
		header string
		sortBy string
		colKey string
		want   string
	}{
		{"active column gets a marker", "Repository", "repo", "repo", "Repository ▼"},
		{"inactive column unchanged", "Repository", "title", "repo", "Repository"},
		{"match is case-insensitive", "Repository", "REPO", "repo", "Repository ▼"},
		{"empty sortBy marks nothing", "Repository", "", "repo", "Repository"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SortIndicator(tt.header, tt.sortBy, tt.colKey); got != tt.want {
				t.Errorf("SortIndicator(%q, %q, %q) = %q, want %q",
					tt.header, tt.sortBy, tt.colKey, got, tt.want)
			}
		})
	}
}

func TestErrorMessage(t *testing.T) {
	got := ErrorMessage("could not reach %s: %d", "api.github.com", 503)

	for _, want := range []string{"Error:", "api.github.com", "503"} {
		if !strings.Contains(got, want) {
			t.Errorf("ErrorMessage() = %q, missing %q", got, want)
		}
	}
}

func TestStyledStatus(t *testing.T) {
	// Styling may be stripped when not attached to a TTY, but the text itself
	// must always survive — it is the only thing the user reads.
	for _, status := range []string{"success", "failure", "open", "merged"} {
		if got := StyledStatus(status); !strings.Contains(got, status) {
			t.Errorf("StyledStatus(%q) = %q, want it to contain the status text", status, got)
		}
	}
}

func TestTerminalWidth(t *testing.T) {
	// Under `go test` no descriptor is a TTY, so this falls through to COLUMNS
	// and then to the built-in default.
	t.Run("falls back to COLUMNS", func(t *testing.T) {
		t.Setenv("COLUMNS", "137")
		if got := TerminalWidth(); got != 137 && got <= 0 {
			t.Errorf("TerminalWidth() = %d, want 137 or a real terminal width", got)
		}
	})

	t.Run("always positive", func(t *testing.T) {
		t.Setenv("COLUMNS", "")
		if got := TerminalWidth(); got <= 0 {
			t.Errorf("TerminalWidth() = %d, want a positive width", got)
		}
	})

	t.Run("ignores garbage COLUMNS", func(t *testing.T) {
		t.Setenv("COLUMNS", "not-a-number")
		if got := TerminalWidth(); got <= 0 {
			t.Errorf("TerminalWidth() = %d, want a positive width", got)
		}
	})

	t.Run("ignores non-positive COLUMNS", func(t *testing.T) {
		t.Setenv("COLUMNS", "0")
		if got := TerminalWidth(); got <= 0 {
			t.Errorf("TerminalWidth() = %d, want a positive width", got)
		}
	})
}

func TestPrintCompactTableIsTabSeparated(t *testing.T) {
	out := captureStdout(t, func() {
		PrintCompactTable(
			[]string{"REPO", "BRANCH"},
			[][]string{{"api-gateway", "main"}, {"service-auth", "develop"}},
		)
	})

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3 (header + 2 rows):\n%s", len(lines), out)
	}
	if lines[0] != "REPO\tBRANCH" {
		t.Errorf("header = %q, want %q", lines[0], "REPO\tBRANCH")
	}
	if lines[1] != "api-gateway\tmain" {
		t.Errorf("row 1 = %q, want %q", lines[1], "api-gateway\tmain")
	}

	// The whole point of compact mode is that awk/cut can split on tabs.
	for i, line := range lines {
		if strings.Count(line, "\t") != 1 {
			t.Errorf("line %d = %q, want exactly one tab", i, line)
		}
	}
}

func TestPrintJSON(t *testing.T) {
	out := captureStdout(t, func() {
		PrintJSON(map[string]any{"name": "api-gateway", "open_prs": 3})
	})

	if !strings.Contains(out, `"name"`) || !strings.Contains(out, "api-gateway") {
		t.Errorf("PrintJSON output missing expected fields:\n%s", out)
	}
	// Indented output is the documented format for piping into jq.
	if !strings.Contains(out, "\n") {
		t.Errorf("PrintJSON output is not indented:\n%s", out)
	}
}

func TestPrintNoDataMessage(t *testing.T) {
	out := captureStdout(t, func() {
		PrintNoDataMessage("No branches found", "try --filter", "check --org")
	})

	for _, want := range []string{"No branches found", "try --filter", "check --org"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestPrintSummaryBanner(t *testing.T) {
	out := captureStdout(t, func() {
		PrintSummaryBanner(10, 7, 3, 1500*time.Millisecond)
	})

	// The banner reports successes and failures, not the total.
	for _, want := range []string{"7", "3", "1.5s"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary missing %q:\n%s", want, out)
		}
	}
}

func TestPrintDryRunBanner(t *testing.T) {
	out := captureStdout(t, func() { PrintDryRunBanner() })

	if !strings.Contains(strings.ToUpper(out), "DRY") {
		t.Errorf("dry-run banner should be unmistakable, got:\n%s", out)
	}
}

func TestPrintDryRunActions(t *testing.T) {
	out := captureStdout(t, func() {
		PrintDryRunActions(
			"delete branch",
			"my-org",
			[]string{"api-gateway", "service-auth"},
			map[string]string{"branch": "old-feature"},
		)
	})

	// A dry run must name the operation, the owner, and every affected repo —
	// this output is the only thing standing between the user and a bulk write.
	for _, want := range []string{"delete branch", "my-org", "api-gateway", "service-auth", "old-feature"} {
		if !strings.Contains(out, want) {
			t.Errorf("dry-run output missing %q:\n%s", want, out)
		}
	}
}

func TestPrintSelectedRepos(t *testing.T) {
	out := captureStderr(t, func() {
		PrintSelectedRepos("create branch", "my-org", []string{"api-gateway"})
	})

	if !strings.Contains(out, "api-gateway") || !strings.Contains(out, "my-org") {
		t.Errorf("output missing repo or org:\n%s", out)
	}
}

func TestPrintFuzzyMatchWarning(t *testing.T) {
	out := captureStderr(t, func() {
		PrintFuzzyMatchWarning("api", []string{"api-gateway", "api-legacy"}, "api-gateway")
	})

	// The user typed something ambiguous; they must see what was chosen for them.
	for _, want := range []string{"api", "api-gateway"} {
		if !strings.Contains(out, want) {
			t.Errorf("fuzzy warning missing %q:\n%s", want, out)
		}
	}
}

func TestPrintNoFuzzyMatchWarning(t *testing.T) {
	out := captureStderr(t, func() { PrintNoFuzzyMatchWarning("nonexistent") })

	if !strings.Contains(out, "nonexistent") {
		t.Errorf("output should name the unmatched term:\n%s", out)
	}
}

func TestNewProgressBar(t *testing.T) {
	if bar := NewProgressBar(10, "cloning"); bar == nil {
		t.Fatal("NewProgressBar returned nil")
	}
	if bar := NewSilentProgressBar(10); bar == nil {
		t.Fatal("NewSilentProgressBar returned nil")
	}
}

// Progress bars are constructed per bulk operation and must tolerate a zero
// total, which happens whenever a filter matches no repositories.
func TestProgressBarWithZeroTotal(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("NewProgressBar(0) panicked: %v", r)
		}
	}()

	if bar := NewProgressBar(0, "nothing to do"); bar == nil {
		t.Fatal("NewProgressBar(0) returned nil")
	}
}

func ExampleShortSHA() {
	fmt.Println(ShortSHA("1c33f19a2b3c4d5e6f7890abcdef1234567890ab"))
	// Output: 1c33f19
}
