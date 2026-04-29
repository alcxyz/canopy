package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/alcxyz/canopy/internal/model"
	"github.com/charmbracelet/lipgloss"
)

// RenderDetail renders the full-screen task detail overlay.
func RenderDetail(t model.Task, stateColors map[model.TaskState]lipgloss.Style, width, height int) string {
	w := min(80, width-4)

	row := func(label, value string) string {
		if value == "" {
			value = DimStyle.Render("—")
		}
		return fmt.Sprintf("  %-16s %s\n", DimStyle.Render(label), value)
	}

	ss := stateColors[t.State]

	var b strings.Builder
	b.WriteString(TitleStyle.Render("  "+t.Title) + "\n\n")
	b.WriteString(row("ID", t.ID))
	b.WriteString(row("State", ss.Render(string(t.State))))
	b.WriteString(row("Type", string(t.Type)))
	b.WriteString(row("Assignee", t.Assignee))
	b.WriteString(row("Sprint", t.Sprint))
	b.WriteString(row("Parent", t.ParentTitle))
	if len(t.Labels) > 0 {
		b.WriteString(row("Tags", strings.Join(t.Labels, ", ")))
	} else {
		b.WriteString(row("Tags", ""))
	}
	b.WriteString("\n")
	b.WriteString(row("Created", FmtTime(t.CreatedAt)))
	b.WriteString(row("Updated", FmtTime(t.UpdatedAt)))
	b.WriteString(row("Start date", FmtTime(t.StartDate)))
	b.WriteString(row("Target date", FmtTime(t.TargetDate)))
	b.WriteString(row("Closed", FmtTime(t.ClosedAt)))
	b.WriteString(row("State changed", FmtTime(t.StateChangedAt)))
	if t.URL != "" {
		b.WriteString("\n")
		b.WriteString(row("URL", DimStyle.Render(t.URL)))
	}
	b.WriteString("\n")
	b.WriteString(DimStyle.Render("  [/] prev/next · enter navigate in · o open · space copy URL · esc close"))

	box := BorderStyle.Width(w).Render(b.String())
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

// FmtTime formats a time value as "YYYY-MM-DD HH:MM (Xd ago)".
func FmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04") + DimStyle.Render(" ("+TimeAgo(t)+")")
}
