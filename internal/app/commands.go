package app

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alcxyz/canopy/internal/config"
	"github.com/alcxyz/canopy/internal/model"
)

// ── Messages ────────────────────────────────────────────────────────────

type tasksLoadedMsg struct {
	myTasks   []model.Task
	teamTasks []model.Task
	doneTasks []model.Task
	err       error
}

type tickMsg time.Time
type ggTimeoutMsg struct{}

// ── Commands ────────────────────────────────────────────────────────────

// loadAllTasks fetches my tasks, team tasks (active only), and done tasks
// using the current dateScope for time-bounding.
func (m Model) loadAllTasks() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		days := dateScopeDays(m.dateScope)
		scope := fmt.Sprintf("last_%d_days", days)

		var myTasks, teamTasks, doneTasks []model.Task

		for _, b := range m.backends {
			// My active tasks (not done/closed)
			my, err := b.ListTasks(ctx, config.Filter{
				Assignee:     "me",
				Status:       []string{"todo", "in-progress", "in-review"},
				UpdatedSince: scope,
			})
			if err != nil {
				return tasksLoadedMsg{err: err}
			}
			myTasks = append(myTasks, my...)

			// Team active tasks (not done/closed)
			team, err := b.ListTasks(ctx, config.Filter{
				Status:       []string{"todo", "in-progress", "in-review"},
				UpdatedSince: scope,
			})
			if err != nil {
				return tasksLoadedMsg{err: err}
			}
			teamTasks = append(teamTasks, team...)

			// Done/closed tasks
			done, err := b.ListTasks(ctx, config.Filter{
				Status:       []string{"done", "closed"},
				UpdatedSince: scope,
			})
			if err != nil {
				return tasksLoadedMsg{err: err}
			}
			doneTasks = append(doneTasks, done...)
		}

		return tasksLoadedMsg{
			myTasks:   myTasks,
			teamTasks: teamTasks,
			doneTasks: doneTasks,
		}
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
