// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package tui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/pradyb/sgh-cli/pkg/ui"
)

var urlRegex = regexp.MustCompile(`https?://[^\s]+`)

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

// visibleLines returns the number of scrollable body lines available.
func (m *detailModel) visibleLines() int {
	v := m.height - 1
	if v < 1 {
		v = 1
	}
	return v
}

// maxScroll returns the max scroll offset given the rendered line count.
func (m *detailModel) maxScroll(totalLines int) int {
	ms := totalLines - m.visibleLines()
	if ms < 0 {
		return 0
	}
	return ms
}

func (m *detailModel) scrollUp() {
	if m.scroll > 0 {
		m.scroll--
	}
}

func (m *detailModel) scrollDown() {
	// Use a generous upper bound; view() will clamp properly.
	if m.scroll < len(m.fields)*4 {
		m.scroll++
	}
}

func (m *detailModel) goTop() {
	m.scroll = 0
}

func (m *detailModel) goBottom() {
	// Set a large value; view() clamps to actual maxScroll.
	m.scroll = len(m.fields) * 4
}

func (m *detailModel) pageUp() {
	m.scroll -= m.visibleLines()
	if m.scroll < 0 {
		m.scroll = 0
	}
}

func (m *detailModel) pageDown() {
	m.scroll += m.visibleLines()
}

func (m detailModel) view() string {
	if !m.visible || m.width < 6 {
		return ""
	}

	// Total lines the panel body can show. Reserve 1 line for the
	// scroll indicator when content overflows, so it never pushes rows out.
	totalVisible := m.visibleLines()

	labelWidth := 0
	for _, f := range m.fields {
		if lw := lipgloss.Width(f.label); lw > labelWidth {
			labelWidth = lw
		}
	}
	labelWidth += 2 // room for ": "

	// Value column width: panel inner width minus label, leading space, separator.
	maxValueWidth := m.width - labelWidth - 3
	if maxValueWidth < 8 {
		maxValueWidth = 8
	}

	// Pre-render all fields into a flat slice of display lines.
	// Each field produces exactly ONE line (values are truncated, not wrapped)
	// except for body/comment fields that contain explicit "\n" — those are
	// split into multiple lines, one per paragraph.
	allLines := make([]string, 0, len(m.fields))
	for _, f := range m.fields {
		if f.label == "" {
			if f.value == "" {
				allLines = append(allLines, "")
				continue
			}
			// Value-only lines (file list, body continuation, etc.).
			// Split on explicit newlines but truncate each segment — no extra wrapping.
			for _, seg := range strings.Split(f.value, "\n") {
				seg = truncateToWidth(seg, m.width-3)
				allLines = append(allLines, " "+strings.Repeat(" ", labelWidth+1)+
					detailValueStyle.Render(highlightURLs(seg)))
			}
		} else {
			label := detailLabelStyle.Width(labelWidth).Render(f.label + ":")
			var valuePart string
			if f.color != "" {
				pillStyle := lipgloss.NewStyle().
					Foreground(lipgloss.Color("#000000")).
					Background(f.color).
					Padding(0, 1)
				val := truncateToWidth(f.value, maxValueWidth-4)
				valuePart = pillStyle.Render(" " + val + " ")
				allLines = append(allLines, fmt.Sprintf(" %s %s", label, valuePart))
			} else {
				// Split on explicit newlines (e.g. body text); truncate each part.
				parts := strings.Split(f.value, "\n")
				for pi, part := range parts {
					part = truncateToWidth(part, maxValueWidth)
					if pi == 0 {
						allLines = append(allLines, fmt.Sprintf(" %s %s",
							label,
							detailValueStyle.Render(highlightURLs(part))))
					} else {
						// Continuation: indent under value column.
						allLines = append(allLines, fmt.Sprintf(" %s %s",
							strings.Repeat(" ", labelWidth),
							detailValueStyle.Render(highlightURLs(part))))
					}
				}
			}
		}
	}

	totalLines := len(allLines)
	ms := m.maxScroll(totalLines)

	// Clamp scroll (goBottom intentionally sets a large value).
	scroll := m.scroll
	if scroll > ms {
		scroll = ms
	}

	// When content overflows, reserve the last body line for the indicator.
	scrollable := totalVisible
	needsIndicator := totalLines > totalVisible
	if needsIndicator {
		scrollable = totalVisible - 1
		if scrollable < 1 {
			scrollable = 1
		}
	}

	end := scroll + scrollable
	if end > totalLines {
		end = totalLines
	}

	var b strings.Builder
	for li := scroll; li < end; li++ {
		line := allLines[li]

		// Scrollbar thumb/track on the right edge.
		scrollChar := ""
		if totalLines > scrollable {
			pct := 0.0
			if ms > 0 {
				pct = float64(scroll) / float64(ms)
			}
			if end >= totalLines {
				pct = 1
			}
			scrollPos := int(pct * float64(scrollable-1))
			if li-scroll == scrollPos {
				scrollChar = "█"
			} else {
				scrollChar = "│"
			}
		}

		b.WriteString(line)
		if scrollChar != "" {
			lw := lipgloss.Width(line)
			pad := m.width - 2 - lw
			if pad > 0 {
				b.WriteString(strings.Repeat(" ", pad))
			}
			b.WriteString(separatorStyle.Render(scrollChar))
		}
		b.WriteString("\n")
	}

	// Scroll position indicator — occupies the reserved last line.
	if needsIndicator {
		pct := 0
		if ms > 0 {
			pct = (scroll * 100) / ms
		}
		if end >= totalLines {
			pct = 100
		}
		indicator := separatorStyle.Render(
			fmt.Sprintf("  ─ %d–%d / %d  (%d%%)  j/k to scroll ─", scroll+1, end, totalLines, pct))
		b.WriteString(indicator)
		b.WriteString("\n")
	}

	return b.String()
}

func statusColor(val string) lipgloss.Color {
	return ui.StatusColor(val)
}

func highlightURLs(text string) string {
	urlStyle := lipgloss.NewStyle().Foreground(ui.Cyan).Underline(true)
	return urlRegex.ReplaceAllStringFunc(text, func(url string) string {
		return urlStyle.Render(url)
	})
}
