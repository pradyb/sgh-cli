package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/prady-lab/sgh-cli/pkg/ui"
)

type detailModel struct {
	title   string
	fields  []detailField
	scroll  int
	focused bool
	width   int
	height  int
	visible bool
}

type detailField struct {
	label string
	value string
	color lipgloss.Color
}

func newDetail() detailModel {
	return detailModel{}
}

func (m *detailModel) setData(title string, fields []detailField) {
	m.title = title
	m.fields = fields
	m.scroll = 0
	m.visible = true
}

func (m *detailModel) clear() {
	m.title = ""
	m.fields = nil
	m.visible = false
	m.scroll = 0
}

func (m *detailModel) scrollUp() {
	if m.scroll > 0 {
		m.scroll--
	}
}

func (m *detailModel) scrollDown() {
	visible := m.height - 1
	if visible < 1 {
		visible = 1
	}
	maxScroll := len(m.fields) - visible
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scroll < maxScroll {
		m.scroll++
	}
}

func (m *detailModel) goTop() {
	m.scroll = 0
}

func (m *detailModel) goBottom() {
	visible := m.height - 1
	if visible < 1 {
		visible = 1
	}
	maxScroll := len(m.fields) - visible
	if maxScroll < 0 {
		maxScroll = 0
	}
	m.scroll = maxScroll
}

func (m detailModel) view() string {
	if !m.visible || m.width < 6 {
		return ""
	}

	var b strings.Builder

	visible := m.height - 1
	if visible < 1 {
		visible = 1
	}
	end := m.scroll + visible
	if end > len(m.fields) {
		end = len(m.fields)
	}

	labelWidth := 0
	for _, f := range m.fields {
		if len(f.label) > labelWidth {
			labelWidth = len(f.label)
		}
	}
	labelWidth += 2

	for i := m.scroll; i < end; i++ {
		f := m.fields[i]

		line := ""
		if f.label == "" {
			line = fmt.Sprintf(" %s%s", strings.Repeat(" ", labelWidth+1), f.value)
		} else {
			label := detailLabelStyle.Width(labelWidth).Render(f.label + ":")
			value := ""
			if f.color != "" {
				pillStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#000000")).Background(f.color).Padding(0, 1)
				value = pillStyle.Render(" " + f.value + " ")
			} else {
				value = detailValueStyle.Render(f.value)
			}
			line = fmt.Sprintf(" %s %s", label, value)
		}

		scrollChar := ""
		if len(m.fields) > visible {
			pct := float64(m.scroll) / float64(len(m.fields)-visible)
			if m.scroll == 0 {
				pct = 0
			} else if end == len(m.fields) {
				pct = 1
			}
			scrollPos := int(pct * float64(visible-1))
			if i-m.scroll == scrollPos {
				scrollChar = "█"
			} else {
				scrollChar = "│"
			}
		}

		b.WriteString(line)
		if scrollChar != "" {
			lineWidth := lipgloss.Width(line)
			pad := m.width - 2 - lineWidth
			if pad > 0 {
				b.WriteString(strings.Repeat(" ", pad))
			}
			b.WriteString(separatorStyle.Render(scrollChar))
		}
		b.WriteString("\n")
	}

	if len(m.fields) > visible {
		scrollInfo := fmt.Sprintf(" (%d-%d of %d)", m.scroll+1, end, len(m.fields))
		b.WriteString(cachedStyle.Render(scrollInfo))
	}

	return b.String()
}

func statusColor(val string) lipgloss.Color {
	return ui.StatusColor(val)
}
