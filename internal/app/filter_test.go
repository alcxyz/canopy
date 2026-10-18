package app

import (
	"slices"
	"testing"

	"github.com/alcxyz/canopy/internal/config"
	"github.com/alcxyz/canopy/internal/model"
)

func TestCollectCycleValues_PresetTagsStayFirst(t *testing.T) {
	m := Model{
		cfg:       config.Config{Tags: []string{"preset-b", "preset-a"}},
		activeTab: tabTeam,
		teamTasks: []model.Task{
			{Title: "Task one", Labels: []string{"zulu"}},
			{Title: "Task two", Labels: []string{"alpha"}},
		},
	}

	got := m.collectCycleValues("tag")
	want := []string{"preset-b", "preset-a", "alpha", "zulu"}
	if !slices.Equal(got, want) {
		t.Fatalf("collectCycleValues(tag) = %#v, want %#v", got, want)
	}
}

func TestCollectCycleValues_UsesFullTextMatch(t *testing.T) {
	m := Model{
		activeTab:   tabTeam,
		filterQuery: "ABC-123",
		teamTasks:   []model.Task{{ID: "ABC-123", Assignee: "Alice"}},
	}

	got := m.collectCycleValues("assignee")
	want := []string{"Alice"}
	if !slices.Equal(got, want) {
		t.Fatalf("collectCycleValues(assignee) = %#v, want %#v", got, want)
	}
}
