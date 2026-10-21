package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/alcxyz/canopy/internal/model"
)

func TestTaskRenderingPlainText(t *testing.T) {
	dirty := "Blå\x1b]0;hidden\a\x1b[2J\b\r\x7f\u0085界\nnext"
	task := model.Task{ID: dirty, Title: dirty, State: model.TaskState(dirty), Type: model.TaskType(dirty), Assignee: dirty, ParentTitle: dirty}
	m := newTestModel()
	m.navStack = []model.Task{task}
	m.filterQuery, m.statusMsg, m.notice = dirty, dirty, dirty
	m.filtering = true
	m.form = createForm{parent: &task}
	m.form.values[formFieldTitle] = dirty
	for name, got := range map[string]string{"list": m.renderTaskList([]model.Task{task}), "header": m.renderHeader(), "footer": m.renderFooter(), "form": m.renderForm()} {
		t.Run(name, func(t *testing.T) {
			for _, bad := range []string{"hidden", "\x1b]", "\x1b[2J", "\b", "\r", "\x7f", "\u0085"} {
				if strings.Contains(got, bad) {
					t.Errorf("contains %q", bad)
				}
			}
			if !strings.Contains(got, "Blå界") {
				t.Errorf("Unicode text missing: %q", got)
			}
		})
	}
	if got := truncate("\x1b[31m世界abcd\x1b[0m", 4); got != "世…" {
		t.Fatalf("truncate = %q", got)
	}
	if task.Title != dirty {
		t.Fatal("rendering changed source task")
	}
}

func TestFilterDisplayPreservesSpaces(t *testing.T) {
	m := newTestModel()
	m.filtering = true
	m.filterQuery = "deploy  release"
	if got := m.renderHeader(); !strings.Contains(got, "deploy  release") {
		t.Fatalf("filter spacing missing: %q", got)
	}
}

func TestTaskErrorIndent(t *testing.T) {
	m := newTestModel(&fakeBackend{})
	m.err = fmt.Errorf("failed\x1b[2J")
	if got := m.renderTaskList(nil); !strings.Contains(got, "  Error: failed") || strings.Contains(got, "\x1b[2J") {
		t.Fatalf("error display = %q", got)
	}
}
