# ADR-006: Extend CreateTaskParams with delivery plan fields

**Status:** Accepted
**Date:** 2026-04-28
**Applies to:** `internal/backend/backend.go`, `internal/backend/azure.go`, `internal/app/form.go`

## Context

The `TaskCreator` interface (ADR-005) supports creating work items with type, title, description, parent, iteration, and assignee. During delivery plan creation for the Bullet team, we discovered this is insufficient for Azure Boards milestones which require tags (`Milestone`, `CapEx`, `OpEx`, `Goal`), start/end dates, and acceptance criteria. The iteration and assignee fields are also read-only in the form, preventing overrides.

These gaps forced all delivery plan work to be done via the `az boards` CLI (see `bn-bootstrap/bin/bullet-boards`), bypassing Canopy entirely.

## Decision

Extend `CreateTaskParams` with new fields rather than creating a separate interface:

```go
type CreateTaskParams struct {
    // ... existing fields ...
    DescriptionHTML    string   // pre-formatted HTML; takes precedence when set
    Tags               []string // backend-agnostic labels
    StartDate          string   // YYYY-MM-DD
    TargetDate         string   // YYYY-MM-DD
    AcceptanceCriteria string   // plain text; backends convert to native format
}
```

The TUI form gains new editable fields: Tags, Start Date, End Date, Acceptance Criteria. Sprint and Assignee become editable (they were read-only). The form uses section grouping to keep the layout scannable.

The Azure backend maps these to:
- `System.Tags` (semicolon-separated)
- `Microsoft.VSTS.Scheduling.StartDate`
- `Microsoft.VSTS.Scheduling.TargetDate`
- `Microsoft.VSTS.Common.AcceptanceCriteria`

## Alternatives Considered

- **New `DeliveryPlanCreator` interface.** Rejected because these fields are broadly useful, not delivery-plan-specific. Tags, dates, and acceptance criteria are standard Azure Boards features.
- **Backend-specific config for default tags.** Rejected because it would hide important classification choices from the user. Tags like CapEx/OpEx are decisions, not defaults.
- **Keep creation in `bullet-boards` only.** Rejected because Canopy's value is the unified dashboard. If creation requires switching to CLI, the TUI loses its purpose for write operations.

## Consequences

- Creating delivery plan milestones can be done entirely within Canopy.
- The form grows from 3 to 9 editable fields. Section grouping and sensible defaults keep it usable.
- `DescriptionHTML` enables future template support without changing the default plain-text flow.
- Other backends (GitHub, Jira, Linear) can ignore fields they don't support since `CreateTaskParams` is a struct, not an interface.
- Existing create operations (just type + title) continue to work with all new fields empty.
