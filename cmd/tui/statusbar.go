package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
)

type statusBarModel struct {
	orgName      string
	selectedRepo int
	totalRepo    int
	apiCalls     int
	command      string
	loading      bool
	spinner      spinner.Model
	lastErr      string
	errExpiry    time.Time
	cacheAge     time.Duration
	width        int
	focusHint    string
}

func newStatusBar(orgName string) statusBarModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle
	return statusBarModel{orgName: orgName, spinner: s}
}

func (m statusBarModel) view() string {
	var parts []string

	parts = append(parts, statusBarOrgStyle.Render(m.orgName))
	parts = append(parts, statusBarCountStyle.Render(fmt.Sprintf("repos: %d/%d", m.selectedRepo, m.totalRepo)))

	if m.command != "" {
		parts = append(parts, fmt.Sprintf("cmd: %s", m.command))
	}

	if m.apiCalls > 0 {
		parts = append(parts, fmt.Sprintf("API: %d", m.apiCalls))
	}

	if m.loading {
		parts = append(parts, m.spinner.View()+" loading...")
	} else if m.cacheAge > 0 {
		parts = append(parts, cachedStyle.Render(formatAge(m.cacheAge)))
	}

	if m.lastErr != "" && time.Now().Before(m.errExpiry) {
		parts = append(parts, errorStyle.Render("err: "+truncate(m.lastErr, 40)))
	} else if m.lastErr != "" {
		m.lastErr = ""
	}

	if m.focusHint != "" {
		parts = append(parts, m.focusHint)
	}

	line := strings.Join(parts, statusBarStyle.Render(" │ "))
	return statusBarStyle.Width(m.width).Render(line)
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
