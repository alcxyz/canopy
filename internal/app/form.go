package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/alcxyz/canopy/internal/backend"
	"github.com/alcxyz/canopy/internal/model"
)

// formTypes lists the work item types available for creation.
var formTypes = []model.TaskType{
	model.TypeFeature,
	model.TypeUserStory,
	model.TypeBug,
	model.TypeTask,
}

const (
	formFieldType = iota
	formFieldTitle
	formFieldDesc
	formFieldCount // sentinel
)

// ── Form key handling ──────────────────────────────────────────────────

func (m Model) handleFormKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.formSubmitting {
		return m, nil // ignore input while submitting
	}

	key := msg.String()

	switch key {
	case "esc":
		m.showForm = false
		return m, nil

	case "ctrl+s":
		// Validate and submit.
		if strings.TrimSpace(m.formTitle) == "" {
			m.formErr = "title is required"
			return m, nil
		}
		m.formErr = ""
		m.formSubmitting = true
		return m, m.createTask()

	case "tab":
		m.formField = (m.formField + 1) % formFieldCount
		return m, nil

	case "shift+tab":
		m.formField = (m.formField - 1 + formFieldCount) % formFieldCount
		return m, nil
	}

	// Field-specific handling.
	switch m.formField {
	case formFieldType:
		switch key {
		case "left", "h":
			m.formType = (m.formType - 1 + len(formTypes)) % len(formTypes)
		case "right", "l":
			m.formType = (m.formType + 1) % len(formTypes)
		}

	case formFieldTitle:
		switch key {
		case "backspace":
			if len(m.formTitle) > 0 {
				m.formTitle = m.formTitle[:len(m.formTitle)-1]
			}
		case "enter":
			m.formField = formFieldDesc
		default:
			if r := msg.Runes; len(r) > 0 {
				m.formTitle += string(r)
			}
		}

	case formFieldDesc:
		switch key {
		case "backspace":
			if len(m.formDesc) > 0 {
				m.formDesc = m.formDesc[:len(m.formDesc)-1]
			}
		case "enter":
			m.formDesc += "\n"
		default:
			if r := msg.Runes; len(r) > 0 {
				m.formDesc += string(r)
			}
		}
	}

	return m, nil
}

// ── Form rendering ─────────────────────────────────────────────────────

func (m Model) renderForm() string {
	w := min(72, m.width-4)
	fieldW := w - 20 // label takes ~18 chars + padding

	var b strings.Builder
	b.WriteString(titleStyle.Render("  Create work item") + "\n\n")

	// Type selector
	typeLabel := string(formTypes[m.formType])
	if m.formField == formFieldType {
		typeLabel = "< " + typeLabel + " >"
	}
	b.WriteString(m.formRow("Type", typeLabel, formFieldType))

	// Title
	b.WriteString(m.formRow("Title", m.formTextInput(m.formTitle, fieldW, formFieldTitle), formFieldTitle))

	// Description (show up to 3 visible lines)
	descDisplay := m.formTextInput(m.formDesc, fieldW, formFieldDesc)
	b.WriteString(m.formRow("Description", descDisplay, formFieldDesc))

	b.WriteString("\n")

	// Read-only info
	parentLabel := dimStyle.Render("none")
	if pid := m.formParentID(); pid != "" {
		pt := m.formParentTitle()
		parentLabel = dimStyle.Render(fmt.Sprintf("#%s %s", pid, pt))
	}
	b.WriteString(m.infoRow("Parent", parentLabel))

	iterLabel := dimStyle.Render("loading...")
	if m.formIteration != "" {
		iterLabel = dimStyle.Render(m.formIteration)
	}
	b.WriteString(m.infoRow("Sprint", iterLabel))

	assigneeLabel := dimStyle.Render("unassigned")
	if m.formAssignee != "" {
		assigneeLabel = dimStyle.Render(m.formAssignee)
	}
	b.WriteString(m.infoRow("Assignee", assigneeLabel))

	// Error
	if m.formErr != "" {
		b.WriteString("\n")
		b.WriteString(overdueStyle.Render("  " + m.formErr))
	}

	if m.formSubmitting {
		b.WriteString("\n")
		b.WriteString(statusStyle.Render("  submitting..."))
	}

	b.WriteString("\n\n")
	b.WriteString(dimStyle.Render("  ctrl+s submit  esc cancel  tab next field"))

	box := borderStyle.Width(w).Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m Model) formRow(label, value string, field int) string {
	l := dimStyle.Render(fmt.Sprintf("  %-14s ", label))
	if m.formField == field {
		l = filterStyle.Render(fmt.Sprintf("  %-14s ", label))
	}
	return l + value + "\n"
}

func (m Model) infoRow(label, value string) string {
	return dimStyle.Render(fmt.Sprintf("  %-14s ", label)) + value + "\n"
}

func (m Model) formTextInput(text string, width, field int) string {
	// Show last visible line only for simplicity.
	display := text
	if lines := strings.Split(text, "\n"); len(lines) > 1 {
		display = lines[len(lines)-1]
	}
	if len([]rune(display)) > width {
		display = string([]rune(display)[len([]rune(display))-width:])
	}

	if m.formField == field {
		return display + filterStyle.Render("█")
	}
	if display == "" {
		return dimStyle.Render("—")
	}
	return display
}

// ── Form helpers ───────────────────────────────────────────────────────

func (m Model) formParentID() string {
	if len(m.navStack) > 0 {
		return m.navStack[len(m.navStack)-1].ID
	}
	return ""
}

func (m Model) formParentTitle() string {
	if len(m.navStack) > 0 {
		return truncate(m.navStack[len(m.navStack)-1].Title, 40)
	}
	return ""
}

func (m Model) defaultFormTypeIndex() int {
	if len(m.navStack) > 0 {
		parent := m.navStack[len(m.navStack)-1]
		switch parent.Type {
		case model.TypeEpic:
			return indexOf(formTypes, model.TypeFeature)
		case model.TypeFeature:
			return indexOf(formTypes, model.TypeUserStory)
		case model.TypeUserStory:
			return indexOf(formTypes, model.TypeTask)
		}
	}
	return 0 // Feature
}

func (m Model) defaultAssignee() string {
	for _, p := range m.cfg.Profiles {
		if len(p.Team) > 0 {
			return p.Team[0]
		}
	}
	return ""
}

// canCreate returns true if any backend supports creating work items.
func (m Model) canCreate() bool {
	for _, b := range m.backends {
		if _, ok := b.(backend.TaskCreator); ok {
			return true
		}
	}
	return false
}

func indexOf(types []model.TaskType, t model.TaskType) int {
	for i, tt := range types {
		if tt == t {
			return i
		}
	}
	return 0
}
