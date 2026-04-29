package backend

import (
	"strings"
	"testing"

	"github.com/alcxyz/canopy/internal/model"
)

func TestBuildCreateOps_TitleAlwaysPresent(t *testing.T) {
	params := CreateTaskParams{
		Type:  model.TypeTask,
		Title: "My Task",
	}
	ops := buildCreateOps(params, "myorg")

	if len(ops) == 0 {
		t.Fatal("expected at least one op")
	}
	if ops[0].Path != "/fields/System.Title" {
		t.Errorf("expected first op path to be System.Title, got %q", ops[0].Path)
	}
	if ops[0].Value != "My Task" {
		t.Errorf("expected title value %q, got %v", "My Task", ops[0].Value)
	}
}

func TestBuildCreateOps_TagsProduceSemicolonSeparated(t *testing.T) {
	params := CreateTaskParams{
		Type:  model.TypeFeature,
		Title: "Feature",
		Tags:  []string{"Milestone", "CapEx"},
	}
	ops := buildCreateOps(params, "myorg")

	var tagsOp *jsonPatchOp
	for i := range ops {
		if ops[i].Path == "/fields/System.Tags" {
			tagsOp = &ops[i]
			break
		}
	}
	if tagsOp == nil {
		t.Fatal("expected System.Tags op but none found")
	}
	want := "Milestone; CapEx"
	if tagsOp.Value != want {
		t.Errorf("expected tags value %q, got %v", want, tagsOp.Value)
	}
}

func TestBuildCreateOps_EmptyTagsProduceNoOp(t *testing.T) {
	params := CreateTaskParams{
		Type:  model.TypeTask,
		Title: "Task",
		Tags:  []string{},
	}
	ops := buildCreateOps(params, "myorg")

	for _, op := range ops {
		if op.Path == "/fields/System.Tags" {
			t.Errorf("expected no System.Tags op for empty tags, but found one")
		}
	}
}

func TestBuildCreateOps_NilTagsProduceNoOp(t *testing.T) {
	params := CreateTaskParams{
		Type:  model.TypeTask,
		Title: "Task",
	}
	ops := buildCreateOps(params, "myorg")

	for _, op := range ops {
		if op.Path == "/fields/System.Tags" {
			t.Errorf("expected no System.Tags op for nil tags, but found one")
		}
	}
}

func TestBuildCreateOps_DatesProduceCorrectOps(t *testing.T) {
	params := CreateTaskParams{
		Type:       model.TypeFeature,
		Title:      "Feature",
		StartDate:  "2026-01-01",
		TargetDate: "2026-03-31",
	}
	ops := buildCreateOps(params, "myorg")

	paths := make(map[string]interface{})
	for _, op := range ops {
		paths[op.Path] = op.Value
	}

	if v, ok := paths["/fields/Microsoft.VSTS.Scheduling.StartDate"]; !ok {
		t.Error("expected StartDate op but none found")
	} else if v != "2026-01-01" {
		t.Errorf("expected StartDate value %q, got %v", "2026-01-01", v)
	}

	if v, ok := paths["/fields/Microsoft.VSTS.Scheduling.TargetDate"]; !ok {
		t.Error("expected TargetDate op but none found")
	} else if v != "2026-03-31" {
		t.Errorf("expected TargetDate value %q, got %v", "2026-03-31", v)
	}
}

func TestBuildCreateOps_EmptyDatesProduceNoOps(t *testing.T) {
	params := CreateTaskParams{
		Type:  model.TypeTask,
		Title: "Task",
	}
	ops := buildCreateOps(params, "myorg")

	for _, op := range ops {
		if op.Path == "/fields/Microsoft.VSTS.Scheduling.StartDate" {
			t.Errorf("expected no StartDate op for empty start date")
		}
		if op.Path == "/fields/Microsoft.VSTS.Scheduling.TargetDate" {
			t.Errorf("expected no TargetDate op for empty target date")
		}
	}
}

func TestBuildCreateOps_DescriptionHTMLTakesPrecedence(t *testing.T) {
	params := CreateTaskParams{
		Type:            model.TypeTask,
		Title:           "Task",
		Description:     "plain text",
		DescriptionHTML: "<p>HTML content</p>",
	}
	ops := buildCreateOps(params, "myorg")

	var descOp *jsonPatchOp
	for i := range ops {
		if ops[i].Path == "/fields/System.Description" {
			descOp = &ops[i]
			break
		}
	}
	if descOp == nil {
		t.Fatal("expected System.Description op but none found")
	}
	if descOp.Value != "<p>HTML content</p>" {
		t.Errorf("expected DescriptionHTML to take precedence, got %v", descOp.Value)
	}
}

func TestBuildCreateOps_PlainDescriptionConverted(t *testing.T) {
	params := CreateTaskParams{
		Type:        model.TypeTask,
		Title:       "Task",
		Description: "line1\nline2",
	}
	ops := buildCreateOps(params, "myorg")

	var descOp *jsonPatchOp
	for i := range ops {
		if ops[i].Path == "/fields/System.Description" {
			descOp = &ops[i]
			break
		}
	}
	if descOp == nil {
		t.Fatal("expected System.Description op but none found")
	}
	val, ok := descOp.Value.(string)
	if !ok {
		t.Fatalf("expected string value, got %T", descOp.Value)
	}
	if !strings.Contains(val, "<br>") {
		t.Errorf("expected plain text to be converted to HTML with <br>, got %q", val)
	}
}

func TestBuildCreateOps_AcceptanceCriteriaConverted(t *testing.T) {
	params := CreateTaskParams{
		Type:               model.TypeUserStory,
		Title:              "Story",
		AcceptanceCriteria: "Given X\nWhen Y\nThen Z",
	}
	ops := buildCreateOps(params, "myorg")

	var acOp *jsonPatchOp
	for i := range ops {
		if ops[i].Path == "/fields/Microsoft.VSTS.Common.AcceptanceCriteria" {
			acOp = &ops[i]
			break
		}
	}
	if acOp == nil {
		t.Fatal("expected AcceptanceCriteria op but none found")
	}
	val, ok := acOp.Value.(string)
	if !ok {
		t.Fatalf("expected string value, got %T", acOp.Value)
	}
	if !strings.Contains(val, "<br>") {
		t.Errorf("expected acceptance criteria to be HTML-converted, got %q", val)
	}
}

func TestBuildCreateOps_EmptyAcceptanceCriteriaProducesNoOp(t *testing.T) {
	params := CreateTaskParams{
		Type:  model.TypeTask,
		Title: "Task",
	}
	ops := buildCreateOps(params, "myorg")

	for _, op := range ops {
		if op.Path == "/fields/Microsoft.VSTS.Common.AcceptanceCriteria" {
			t.Errorf("expected no AcceptanceCriteria op for empty criteria")
		}
	}
}

func TestBuildCreateOps_ParentIDProducesRelationOp(t *testing.T) {
	params := CreateTaskParams{
		Type:     model.TypeTask,
		Title:    "Task",
		ParentID: "42",
	}
	ops := buildCreateOps(params, "myorg")

	var relOp *jsonPatchOp
	for i := range ops {
		if ops[i].Path == "/relations/-" {
			relOp = &ops[i]
			break
		}
	}
	if relOp == nil {
		t.Fatal("expected relations op but none found")
	}
	rv, ok := relOp.Value.(relationValue)
	if !ok {
		t.Fatalf("expected relationValue, got %T", relOp.Value)
	}
	if rv.Rel != "System.LinkTypes.Hierarchy-Reverse" {
		t.Errorf("unexpected rel type: %q", rv.Rel)
	}
	wantURL := "https://dev.azure.com/myorg/_apis/wit/workItems/42"
	if rv.URL != wantURL {
		t.Errorf("expected URL %q, got %q", wantURL, rv.URL)
	}
}

func TestBuildCreateOps_EmptyParentIDProducesNoRelationOp(t *testing.T) {
	params := CreateTaskParams{
		Type:  model.TypeTask,
		Title: "Task",
	}
	ops := buildCreateOps(params, "myorg")

	for _, op := range ops {
		if op.Path == "/relations/-" {
			t.Errorf("expected no relation op for empty ParentID")
		}
	}
}
