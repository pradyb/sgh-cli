package ui

import "github.com/charmbracelet/lipgloss"

const (
	HyperLinkFormat           = "\x1b]8;;%s\x07%s\x1b]8;;\x07\u001b[0m"
	repositoryNameDisplayName = "Repository"
	errorMessageDisplayName   = "Error Message"

	White        = lipgloss.Color("#FFFFFF")
	Gray         = lipgloss.Color("#CCC9C9")
	LightGray    = lipgloss.Color("#959393")
	Turquoise    = lipgloss.Color("#5DE2E7")
	Red          = lipgloss.Color("#FF0000")
	Green        = lipgloss.Color("#00B500")
	CrayolaGreen = lipgloss.Color("#25A065")
)
