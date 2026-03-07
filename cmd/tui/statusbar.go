package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
)

type statusBarModel struct {
	orgName       string
	selectedRepo  int
	totalRepo     int
	apiCalls      int
	command       string
	loading       bool
	spinner       spinner.Model
	lastErr       string
	errExpiry     time.Time
	cacheAge      time.Duration
	width         int
	focusHint     string
	filteredCount int
	totalCount    int
	loadingMsg    string
}

func newStatusBar(orgName string) statusBarModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle
	return statusBarModel{orgName: orgName, spinner: s}
}

func (m statusBarModel) view() string {
	var leftParts []string
	var rightParts []string

	leftParts = append(leftParts, statusBarOrgStyle.Render(m.orgName))
	leftParts = append(leftParts, statusBarCountStyle.Render(fmt.Sprintf("repos: %d/%d", m.selectedRepo, m.totalRepo)))

	if m.command != "" {
		cmdText := fmt.Sprintf("cmd: %s", m.command)
		if m.filteredCount > 0 && m.filteredCount < m.totalCount {
			cmdText += fmt.Sprintf(" (%s%d/%d%s)",
				statusBarCountStyle.Render(""),
				m.filteredCount,
				m.totalCount,
				statusBarCountStyle.Render(""))
		}
		leftParts = append(leftParts, cmdText)
	}

	if m.apiCalls > 0 {
		rightParts = append(rightParts, fmt.Sprintf("API: %d", m.apiCalls))
	}

	if m.loading {
		msg := "loading..."
		if m.loadingMsg != "" {
			msg = m.loadingMsg
		}
		rightParts = append(rightParts, m.spinner.View()+" "+msg)
	} else if m.cacheAge > 0 {
		rightParts = append(rightParts, cachedStyle.Render(formatAge(m.cacheAge)))
	}

	if m.lastErr != "" && time.Now().Before(m.errExpiry) {
		rightParts = append(rightParts, errorStyle.Render("err: "+truncate(m.lastErr, 40)))
	} else if m.lastErr != "" {
		m.lastErr = ""
	}

	separator := separatorStyle.Render(" │ ")
	right := strings.Join(rightParts, separator)

	usableWidth := m.width - 2 // account for statusBarStyle Padding(0, 1)

	// Only add the focus hint if it fits in the available space
	leftWithoutHint := strings.Join(leftParts, separator)
	if m.focusHint != "" {
		hintStr := formatKeyHints(m.focusHint)
		if lipgloss.Width(leftWithoutHint)+lipgloss.Width(separator)+lipgloss.Width(hintStr)+lipgloss.Width(right)+1 <= usableWidth {
			leftParts = append(leftParts, hintStr)
		}
	}

	left := strings.Join(leftParts, separator)

	pad := usableWidth - lipgloss.Width(left) - lipgloss.Width(right)
	var line string
	if pad > 0 {
		line = left + strings.Repeat(" ", pad) + right
	} else {
		line = left + " " + right
	}

	return statusBarStyle.Width(m.width).MaxWidth(m.width).Render(line)
}

func formatKeyHints(hints string) string {
	var formatted []string
	for _, part := range strings.Split(hints, " ") {
		if part == "" {
			continue
		}
		if strings.HasPrefix(part, "(") && strings.HasSuffix(part, ")") {
			formatted = append(formatted, statusHintDescStyle.Render(part))
			continue
		}
		idx := strings.Index(part, ":")
		if idx > 0 {
			key := part[:idx]
			desc := part[idx+1:]
			formatted = append(formatted, statusHintKeyStyle.Render(key)+statusHintDescStyle.Render(":"+desc))
		} else {
			formatted = append(formatted, statusHintDescStyle.Render(part))
		}
	}
	return strings.Join(formatted, " ")
}

func formatAge(d time.Duration) string {
	if d < 5*time.Second {
		return "fetched just now"
	}
	if d < time.Minute {
		return fmt.Sprintf("cached %ds ago", int(d.Seconds()))
	}
	return fmt.Sprintf("cached %dm ago", int(d.Minutes()))
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
