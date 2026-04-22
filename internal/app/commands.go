package app

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alcxyz/canopy/internal/config"
	"github.com/alcxyz/canopy/internal/model"
)

// ── Messages ────────────────────────────────────────────────────────────

type tasksLoadedMsg struct {
	myTasks   []model.Task
	teamTasks []model.Task
	err       error
}

type tickMsg time.Time

// ── Commands ────────────────────────────────────────────────────────────

func (m Model) loadAllTasks() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		var myTasks, teamTasks []model.Task

		for _, b := range m.backends {
			// My tasks
			my, err := b.ListTasks(ctx, config.Filter{Assignee: "me"})
			if err != nil {
				return tasksLoadedMsg{err: err}
			}
			myTasks = append(myTasks, my...)

			// Team tasks (no assignee filter — returns all)
			all, err := b.ListTasks(ctx, config.Filter{})
			if err != nil {
				return tasksLoadedMsg{err: err}
			}
			teamTasks = append(teamTasks, all...)
		}

		return tasksLoadedMsg{myTasks: myTasks, teamTasks: teamTasks}
	}
}

func (m Model) loadViewTasks(filter config.Filter) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		var tasks []model.Task
		for _, b := range m.backends {
			t, err := b.ListTasks(ctx, filter)
			if err != nil {
				return tasksLoadedMsg{err: err}
			}
			tasks = append(tasks, t...)
		}

		return tasksLoadedMsg{myTasks: tasks, teamTasks: tasks}
	}
}

func tickCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
