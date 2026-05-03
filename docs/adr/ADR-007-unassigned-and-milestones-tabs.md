# ADR-007: Unassigned and Milestones tabs

**Status:** Accepted
**Date:** 2026-04-29
**Applies to:** `internal/app/model.go`, `internal/app/update.go`, `internal/app/view.go`, `internal/app/commands.go`

## Context

Canopy's original four tabs (My Tasks, Team, Done, Views) left two gaps:

1. **Unassigned work** was invisible unless a team member happened to filter for it. Items with no assignee fell through the cracks between My Tasks (assigned to me) and Team (assigned to anyone on the team list).

2. **Milestone/iteration progress** required opening Azure DevOps. There was no way to see at a glance how much of a sprint or tagged milestone was complete.

Both gaps matter for a tech lead's daily workflow: triaging unassigned backlog and tracking delivery progress.

## Decision

Add two new tabs, shifting the tab bar from four to six entries:

| Position | Tab | Data source |
|----------|-----|-------------|
| 1 | My Tasks | (unchanged) |
| **2** | **Unassigned** | WIQL query with `[System.AssignedTo] = ''` |
| 3 | Team | (unchanged, was position 2) |
| 4 | Done | (unchanged, was position 3) |
| **5** | **Milestones** | Aggregated from all loaded task lists, grouped |
| 6 | Views | (unchanged, was position 4) |

### Unassigned tab

A new backend query fetches active work items with no assignee, using the same date scope and status filters as Team. In the WIQL builder, the special assignee value `"unassigned"` emits `[System.AssignedTo] = ''`.

### Milestones tab

Operates in two modes, toggled with `m`:

- **Iteration mode** (default): groups all loaded tasks (deduplicated across My Tasks, Unassigned, Team, Done) by their Sprint/iteration path. Each group shows total count, done count, and a progress bar.
- **Tag mode**: groups tasks by configured `milestone_tags` (defaults to `["Milestone"]`). Same progress display.

Press Enter to drill into a group and see its tasks. Esc returns to the group list. Standard filters and nav stack work within a drilled group.

### Configuration

A new `milestone_tags` field in `config.yaml` controls which tags identify milestones in tag mode:

```yaml
milestone_tags:
  - Milestone
```

## Alternatives Considered

**Use Views for milestones.** Views could filter by tag or iteration, but they show flat task lists without progress aggregation. The grouped view with progress bars is the key value-add.

**Derive unassigned from Team data.** We considered client-side filtering of Team tasks where `Assignee == ""`, but the Team query already filters to configured team members. A separate backend query is needed to capture truly unassigned items.

**Put Milestones before Done.** Considered ordering active work closer together (My Tasks, Unassigned, Team, Milestones, Done, Views) but Done is consulted more frequently than Milestones in daily standups, so it stays at position 4.

## Consequences

- Tab bar grows from 4 to 6 entries. Number keys 2-4 change meaning. The `RenderTabs` function already supports dynamic wrapping for narrow terminals.
- One additional API call per refresh cycle (unassigned query). Mitigated by the existing 5-concurrent-request semaphore.
- Milestones tab reuses existing loaded data (no extra API calls) but computes groups on render. This is O(n) over all tasks and negligible for typical dataset sizes.
- Milestone mode and active tab are persisted in the cache `UIState`, surviving restarts.
