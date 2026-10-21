package ui

import (
	"strings"
	"testing"

	"github.com/alcxyz/canopy/internal/model"
	"github.com/charmbracelet/lipgloss"
)

func TestDetailPlainTaskFields(t *testing.T) {
	dirty := "Blå\x1b]0;hidden\a\x1b[2J\b\r\x7f\u0085界\nnext"
	task := model.Task{ID: dirty, Title: dirty, State: model.TaskState(dirty), Type: model.TaskType(dirty), Assignee: dirty, Sprint: dirty, ParentTitle: dirty, Labels: []string{dirty}, URL: dirty}
	got := RenderDetail(task, map[model.TaskState]lipgloss.Style{}, 120, 60)
	for _, bad := range []string{"hidden", "\x1b]", "\x1b[2J", "\b", "\r", "\x7f", "\u0085"} {
		if strings.Contains(got, bad) {
			t.Errorf("detail contains %q", bad)
		}
	}
	if strings.Count(got, "Blå界") != 9 || !strings.Contains(got, "next") {
		t.Fatalf("task text missing: %q", got)
	}
}
