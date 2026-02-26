package ui

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

const (
	HyperLinkFormat           = "\x1b]8;;%s\x07%s\x1b]8;;\x07\u001b[0m"
	repositoryNameDisplayName = "Repository"
	errorMessageDisplayName   = "Error Message"

	White        = lipgloss.Color("#E6E6E6")
	Subtle       = lipgloss.Color("#A0A0A0")
	Dimmed       = lipgloss.Color("#6C6C6C")
	Cyan         = lipgloss.Color("#56C8D8")
	Red          = lipgloss.Color("#E05561")
	Yellow       = lipgloss.Color("#E5C07B")
	Orange       = lipgloss.Color("#D19A66")
	Green        = lipgloss.Color("#58B573")
	Blue         = lipgloss.Color("#61AFEF")
	Purple       = lipgloss.Color("#C678DD")
	CrayolaGreen = lipgloss.Color("#2EA77A")

	// Aliases kept for backward compatibility
	Gray      = Subtle
	LightGray = Dimmed
)

// StatusColor returns the appropriate lipgloss color for a given status string,
// following GitHub's conventional color language.
func StatusColor(status string) lipgloss.Color {
	switch strings.ToLower(status) {
	case "success", "approved", "clean":
		return Green
	case "open":
		return Blue
	case "merged":
		return Purple
	case "failure", "failed", "closed", "changes_requested":
		return Red
	case "in_progress", "queued", "pending", "waiting":
		return Yellow
	case "blocked":
		return Orange
	case "cancelled", "skipped", "dismissed":
		return Dimmed
	default:
		return Subtle
	}
}

// StyledStatus returns the status string rendered with its appropriate color.
func StyledStatus(status string) string {
	return lipgloss.NewStyle().Foreground(StatusColor(status)).Render(status)
}

// StatusIcon returns a small glyph representing a conclusion/status.
func StatusIcon(conclusion string) string {
	switch conclusion {
	case "success":
		return lipgloss.NewStyle().Foreground(Green).Render("✓")
	case "failure":
		return lipgloss.NewStyle().Foreground(Red).Render("✗")
	case "cancelled":
		return lipgloss.NewStyle().Foreground(Dimmed).Render("⊘")
	case "skipped":
		return lipgloss.NewStyle().Foreground(Dimmed).Render("○")
	case "in_progress":
		return lipgloss.NewStyle().Foreground(Yellow).Render("●")
	case "queued":
		return lipgloss.NewStyle().Foreground(Yellow).Render("◌")
	default:
		return lipgloss.NewStyle().Foreground(Subtle).Render("·")
	}
}

// RelativeTime converts an RFC 3339 timestamp to a human-friendly string
// like "3 minutes ago". Returns the original string on parse failure.
func RelativeTime(timestamp string) string {
	if timestamp == "" {
		return ""
	}

	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return timestamp
	}

	duration := time.Since(t)
	if duration < 0 {
		return "just now"
	}

	seconds := int(math.Floor(duration.Seconds()))
	if seconds < 60 {
		return "just now"
	}

	minutes := seconds / 60
	if minutes == 1 {
		return "1 minute ago"
	}
	if minutes < 60 {
		return fmt.Sprintf("%d minutes ago", minutes)
	}

	hours := minutes / 60
	if hours == 1 {
		return "1 hour ago"
	}
	if hours < 24 {
		return fmt.Sprintf("%d hours ago", hours)
	}

	days := hours / 24
	if days == 1 {
		return "1 day ago"
	}
	if days < 30 {
		return fmt.Sprintf("%d days ago", days)
	}

	months := days / 30
	if months == 1 {
		return "1 month ago"
	}
	if months < 12 {
		return fmt.Sprintf("%d months ago", months)
	}

	years := months / 12
	if years == 1 {
		return "1 year ago"
	}
	return fmt.Sprintf("%d years ago", years)
}

// PrintSummaryBanner prints a colored one-line summary after bulk operations.
func PrintSummaryBanner(total, successCount, errorCount int, duration time.Duration) {
	successStyle := lipgloss.NewStyle().Bold(true).Foreground(Green)
	errorStyle := lipgloss.NewStyle().Bold(true).Foreground(Red)
	dimStyle := lipgloss.NewStyle().Foreground(Gray)
	wrapStyle := lipgloss.NewStyle().PaddingLeft(1).PaddingBottom(1)

	parts := []string{
		successStyle.Render(fmt.Sprintf("✓ %d succeeded", successCount)),
	}
	if errorCount > 0 {
		parts = append(parts, errorStyle.Render(fmt.Sprintf("✗ %d failed", errorCount)))
	}
	parts = append(parts, dimStyle.Render(fmt.Sprintf("⏱ %.1fs", duration.Seconds())))

	fmt.Println(wrapStyle.Render(strings.Join(parts, "  ")))
}

// SortIndicator returns the header name with a sort arrow appended.
func SortIndicator(header, sortBy string, colKey string) string {
	if strings.EqualFold(sortBy, colKey) {
		return header + " ▼"
	}
	return header
}

// PrintCompactTable prints tab-separated rows with a header line.
// Useful for piping output to grep/awk/cut.
func PrintCompactTable(headers []string, rows [][]string) {
	fmt.Println(strings.Join(headers, "\t"))
	for _, row := range rows {
		fmt.Println(strings.Join(row, "\t"))
	}
}

// PrintJSON marshals data as indented JSON to stdout.
func PrintJSON(data any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(data)
}

const defaultTermWidth = 200

// TerminalWidth returns the current terminal width by trying multiple
// detection strategies: stdout/stderr/stdin fd, then the COLUMNS env var,
// and finally a generous default.
func TerminalWidth() int {
	for _, f := range []*os.File{os.Stdout, os.Stderr, os.Stdin} {
		if w, _, err := term.GetSize(int(f.Fd())); err == nil && w > 0 {
			return w
		}
	}
	if cols := os.Getenv("COLUMNS"); cols != "" {
		if w, err := strconv.Atoi(cols); err == nil && w > 0 {
			return w
		}
	}
	return defaultTermWidth
}

