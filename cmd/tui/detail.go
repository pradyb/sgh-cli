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

		if f.label == "" {
			b.WriteString(fmt.Sprintf(" %s%s", strings.Repeat(" ", labelWidth+1), f.value))
			b.WriteString("\n")
			continue
		}

		label := detailLabelStyle.Width(labelWidth).Render(f.label + ":")
		valStyle := detailValueStyle
		if f.color != "" {
			valStyle = valStyle.Foreground(f.color)
		}
		value := valStyle.Render(f.value)

		b.WriteString(fmt.Sprintf(" %s %s", label, value))
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
