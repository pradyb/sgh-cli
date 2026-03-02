package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/prady-lab/sgh-cli/pkg/ui"
)

var (
	sidebarWidth = 34

	panelBorder = lipgloss.RoundedBorder()

	panelFocusedStyle = lipgloss.NewStyle().
		BorderStyle(panelBorder).
		BorderForeground(ui.Cyan).
		Padding(0, 1)

	panelUnfocusedStyle = lipgloss.NewStyle().
		BorderStyle(panelBorder).
		BorderForeground(ui.Dimmed).
		Padding(0, 1)

	panelTitleFocusedStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.Cyan).
		Padding(0, 1)

	panelTitleUnfocusedStyle = lipgloss.NewStyle().
		Foreground(ui.Subtle).
		Padding(0, 1)

	repoSelectedStyle = lipgloss.NewStyle().Foreground(ui.Green)
	repoNormalStyle   = lipgloss.NewStyle().Foreground(ui.Subtle)
	repoCursorStyle   = lipgloss.NewStyle().Foreground(ui.White).Bold(true)

	commandActiveStyle = lipgloss.NewStyle().
		Foreground(ui.CrayolaGreen).
		Bold(true)
	commandNormalStyle = lipgloss.NewStyle().
		Foreground(ui.White)
	commandCursorStyle = lipgloss.NewStyle().
		Foreground(ui.Cyan).
		Bold(true)

	contentHeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.Subtle)

	contentRowStyle    = lipgloss.NewStyle().Foreground(ui.White)
	contentRowDimStyle = lipgloss.NewStyle().Foreground(ui.Dimmed)
	contentCursorStyle = lipgloss.NewStyle().
		Foreground(ui.CrayolaGreen).
		Bold(true)

	detailLabelStyle = lipgloss.NewStyle().
		Foreground(ui.Subtle).
		Bold(true)

	detailValueStyle = lipgloss.NewStyle().
		Foreground(ui.White)

	statusBarStyle = lipgloss.NewStyle().
		Foreground(ui.Dimmed).
		Italic(true).
		Padding(0, 1)

	statusBarOrgStyle = lipgloss.NewStyle().
		Foreground(ui.Cyan).
		Bold(true)

	statusBarCountStyle = lipgloss.NewStyle().
		Foreground(ui.Green)

	separatorStyle = lipgloss.NewStyle().
		Foreground(ui.Dimmed)

	filterInputStyle = lipgloss.NewStyle().
		Foreground(ui.Yellow).
		Bold(true)

	badgeStyle = lipgloss.NewStyle().
		Foreground(ui.Green).
		Bold(true)

	spinnerStyle = lipgloss.NewStyle().
		Foreground(ui.Cyan)

	errorStyle = lipgloss.NewStyle().
		Foreground(ui.Red).
		Bold(true)

	cachedStyle = lipgloss.NewStyle().
		Foreground(ui.Dimmed).
		Italic(true)

	confirmStyle = lipgloss.NewStyle().
		Foreground(ui.Yellow).
		Bold(true).
		Padding(0, 1)

	toastStyle = lipgloss.NewStyle().
		Foreground(ui.Green).
		Italic(true).
		Padding(0, 1)

	helpOverlayTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.Cyan).
		Padding(0, 0, 1, 0)

	helpOverlayKeyStyle = lipgloss.NewStyle().
		Foreground(ui.Green).
		Bold(true).
		Width(16)

	helpOverlayDescStyle = lipgloss.NewStyle().
		Foreground(ui.White)

	helpOverlaySectionStyle = lipgloss.NewStyle().
		Foreground(ui.Subtle).
		Bold(true).
		Italic(true).
		Padding(1, 0, 0, 0)
)

func panelStyle(focused bool) lipgloss.Style {
	if focused {
		return panelFocusedStyle
	}
	return panelUnfocusedStyle
}

func panelTitleStyle(focused bool) lipgloss.Style {
	if focused {
		return panelTitleFocusedStyle
	}
	return panelTitleUnfocusedStyle
}

// renderBorderedPanel renders a bordered box. w is inner content width, h is inner body height
// (excluding the title line). The title is rendered as the first line inside the border.
func renderBorderedPanel(title string, body string, w, h int, focused bool) string {
	totalH := h + 1 // +1 for the title line
	style := panelStyle(focused).
		Width(w).
		Height(totalH).
		BorderTop(true).BorderBottom(true).BorderLeft(true).BorderRight(true)
	titleStr := panelTitleStyle(focused).Render(title)
	return style.Render(lipgloss.JoinVertical(lipgloss.Left, titleStr, body))
}
