package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// RenderTabs renders a row of tab labels, wrapping to multiple rows only
// when the terminal is too narrow to fit them all on one line.
func RenderTabs(tabs []string, active, width int) string {
	// Find the longest tab label to set a uniform column width.
	colW := 0
	for _, t := range tabs {
		if len(t) > colW {
			colW = len(t)
		}
	}

	// Measure the rendered width of a single tab cell (label + padding).
	cellW := lipgloss.Width(TabStyle.Render(fmt.Sprintf("%-*s", colW, "")))

	// Determine how many tabs fit per row; fall back to all-on-one-row
	// when width is unknown (zero) or large enough.
	perRow := len(tabs)
	if width > 0 && cellW > 0 {
		perRow = width / cellW
		if perRow < 1 {
			perRow = 1
		}
		if perRow > len(tabs) {
			perRow = len(tabs)
		}
	}

	var rows []string
	for start := 0; start < len(tabs); start += perRow {
		end := min(start+perRow, len(tabs))
		var row []string
		for i := start; i < end; i++ {
			label := fmt.Sprintf("%-*s", colW, tabs[i])
			if i == active {
				row = append(row, ActiveTabStyle.Render(label))
			} else {
				row = append(row, TabStyle.Render(label))
			}
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, row...))
	}
	return strings.Join(rows, "\n")
}
