package app

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/alcxyz/canopy/internal/backend"
	"github.com/alcxyz/canopy/internal/model"
	"github.com/alcxyz/canopy/internal/ui"
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
	formFieldTags
	formFieldStartDate
	formFieldTargetDate
	formFieldAcceptCriteria
	formFieldIteration
	formFieldAssignee
	formFieldCount // sentinel
)

// formFieldSpecs describes how each form field is labelled and edited.
var formFieldSpecs = [formFieldCount]struct {
	label     string
	multiline bool // enter inserts a newline instead of moving on
	date      bool // must be empty or YYYY-MM-DD
}{
	formFieldType:           {label: "Type"},
	formFieldTitle:          {label: "Title"},
	formFieldDesc:           {label: "Description", multiline: true},
	formFieldTags:           {label: "Tags"},
	formFieldStartDate:      {label: "Start Date", date: true},
	formFieldTargetDate:     {label: "End Date", date: true},
	formFieldAcceptCriteria: {label: "Criteria", multiline: true},
	formFieldIteration:      {label: "Sprint"},
	formFieldAssignee:       {label: "Assignee"},
}

// createForm holds the state of the create-work-item overlay.
type createForm struct {
	field      int
	typeIdx    int // index into formTypes
	values     [formFieldCount]string
	err        string
	submitting bool

	creator backend.TaskCreator // backend the item is created in
	parent  *model.Task         // parent work item, when created from a drill-down
}

// dropLastRune removes the final rune, keeping multi-byte characters intact.
func dropLastRune(s string) string {
	_, size := utf8.DecodeLastRuneInString(s)
	return s[:len(s)-size]
}

// ── Form key handling ──────────────────────────────────────────────────

func (m Model) handleFormKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	f := &m.form
	if f.submitting {
		return m, nil // ignore input while submitting
	}

	key := msg.String()
	switch key {
	case "esc":
		m.showForm = false
		return m, nil
	case "ctrl+s":
		if err := f.validate(); err != "" {
			f.err = err
			return m, nil
		}
		f.err = ""
		f.submitting = true
		return m, createTask(*f)
	case "tab":
		f.field = (f.field + 1) % formFieldCount
		return m, nil
	case "shift+tab":
		f.field = (f.field - 1 + formFieldCount) % formFieldCount
		return m, nil
	}

	if f.field == formFieldType {
		switch key {
		case "left", "h":
			f.typeIdx = (f.typeIdx - 1 + len(formTypes)) % len(formTypes)
		case "right", "l":
			f.typeIdx = (f.typeIdx + 1) % len(formTypes)
		}
		return m, nil
	}

	value := &f.values[f.field]
	switch key {
	case "backspace":
		*value = dropLastRune(*value)
	case "enter":
		if formFieldSpecs[f.field].multiline {
			*value += "\n"
		} else if f.field < formFieldCount-1 {
			f.field++
		}
	default:
		*value += string(msg.Runes)
	}
	return m, nil
}

// validate returns a message describing the first invalid field, or "".
func (f createForm) validate() string {
	if strings.TrimSpace(f.values[formFieldTitle]) == "" {
		return "title is required"
	}
	for i, spec := range formFieldSpecs {
		if !spec.date || f.values[i] == "" {
			continue
		}
		if _, err := time.Parse(time.DateOnly, f.values[i]); err != nil {
			return strings.ToLower(spec.label) + " must be YYYY-MM-DD"
		}
	}
	return ""
}

// openCreateForm opens the create overlay. Items created while drilled into
// a task become its children and are created in that task's profile.
func (m *Model) openCreateForm() tea.Cmd {
	var parent *model.Task
	if n := len(m.navStack); n > 0 {
		p := m.navStack[n-1]
		parent = &p
	}

	creator, profile := m.creatorFor(parent)
	if creator == nil {
		if parent != nil {
			m.statusMsg = fmt.Sprintf("profile %q cannot create work items", parent.Profile)
		} else {
			m.statusMsg = "no configured backend can create work items"
		}
		return nil
	}

	m.form = createForm{
		field:   formFieldTitle,
		typeIdx: defaultFormTypeIndex(parent),
		creator: creator,
		parent:  parent,
	}
	m.form.values[formFieldAssignee] = m.defaultAssignee(profile)
	m.showForm = true
	return resolveIteration(creator)
}

// creatorFor returns the backend that should create a child of parent (or a
// top-level item when parent is nil) and its profile name.
func (m Model) creatorFor(parent *model.Task) (backend.TaskCreator, string) {
	for _, b := range m.backends {
		creator, ok := b.(backend.TaskCreator)
		if !ok || (parent != nil && b.Name() != parent.Profile) {
			continue
		}
		return creator, b.Name()
	}
	return nil, ""
}

// defaultAssignee returns the first team member of the named profile.
func (m Model) defaultAssignee(profile string) string {
	for _, p := range m.cfg.Profiles {
		if p.Name == profile && len(p.Team) > 0 {
			return p.Team[0]
		}
	}
	return ""
}

func defaultFormTypeIndex(parent *model.Task) int {
	if parent == nil {
		return 0 // Feature
	}
	child := map[model.TaskType]model.TaskType{
		model.TypeEpic:      model.TypeFeature,
		model.TypeFeature:   model.TypeUserStory,
		model.TypeUserStory: model.TypeTask,
	}[parent.Type]
	for i, t := range formTypes {
		if t == child {
			return i
		}
	}
	return 0
}

// ── Form rendering ─────────────────────────────────────────────────────

func (m Model) renderForm() string {
	f := m.form
	w := min(72, m.width-4)
	fieldW := w - 20 // label takes ~18 chars + padding

	var b strings.Builder
	b.WriteString(ui.TitleStyle.Render("  Create work item") + "\n\n")

	for i, spec := range formFieldSpecs {
		switch i {
		case formFieldTags:
			b.WriteString("\n" + ui.DimStyle.Render("  -- Delivery plan --") + "\n")
		case formFieldIteration:
			b.WriteString("\n")
		}

		var value string
		switch {
		case i == formFieldType:
			value = string(formTypes[f.typeIdx])
			if f.field == i {
				value = "< " + value + " >"
			}
		case spec.date && f.values[i] == "" && f.field != i:
			value = ui.DimStyle.Render("YYYY-MM-DD")
		default:
			value = f.textInput(i, fieldW)
		}
		b.WriteString(f.row(spec.label, value, f.field == i))
	}

	// Parent (read-only)
	parentLabel := ui.DimStyle.Render("none")
	if f.parent != nil {
		parentLabel = ui.DimStyle.Render(fmt.Sprintf("#%s %s", f.parent.ID, truncate(f.parent.Title, 40)))
	}
	b.WriteString(f.row("Parent", parentLabel, false))

	if f.err != "" {
		b.WriteString("\n" + ui.OverdueStyle.Render("  "+f.err))
	}
	if f.submitting {
		b.WriteString("\n" + ui.StatusStyle.Render("  submitting..."))
	}

	b.WriteString("\n\n")
	b.WriteString(ui.DimStyle.Render("  ctrl+s submit  esc cancel  tab next field"))

	box := ui.BorderStyle.Width(w).Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (f createForm) row(label, value string, focused bool) string {
	style := ui.DimStyle
	if focused {
		style = ui.FilterStyle
	}
	return style.Render(fmt.Sprintf("  %-14s ", label)) + value + "\n"
}

// textInput renders the last line of a field's value, scrolled to fit width.
func (f createForm) textInput(field, width int) string {
	text := f.values[field]
	if i := strings.LastIndexByte(text, '\n'); i >= 0 {
		text = text[i+1:]
	}
	if r := []rune(text); len(r) > width {
		text = string(r[len(r)-width:])
	}

	if f.field == field {
		return text + ui.FilterStyle.Render("█")
	}
	if text == "" {
		return ui.DimStyle.Render("—")
	}
	return text
}
