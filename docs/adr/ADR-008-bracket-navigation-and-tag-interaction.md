# ADR-008: Bracket navigation and tag interaction

**Status:** Accepted
**Date:** 2026-04-29
**Applies to:** `internal/app/update.go`, `internal/app/filter.go`, `internal/app/view.go`

## Context

Grove uses three bracket pairs for structural navigation at different granularities:

| Keys | Grove behaviour |
|------|----------------|
| `[]` | Jump between repo blocks |
| `{}` | Jump between config-defined groups |
| `()` | Jump between subject/status blocks |

Canopy currently has `[]` for sibling navigation when drilled into a task's subtasks, but at the top level it does nothing. There is no equivalent of `{}` or `()`.

Separately, users track work with classification tags beyond milestones: **CapEx**, **OpEx**, and potentially others. The `t` cycle filter lets you filter to one tag at a time, but there's no way to _navigate between_ tag blocks or treat tags as a structural axis for movement.

These two needs — structural bracket navigation and first-class tag interaction — are complementary and should be designed together.

## Decision

Adopt grove's three-tier bracket navigation, adapted to canopy's domain model:

### `[]` — Iteration/sprint block jumping

On any task-list tab (My Tasks, Unassigned, Team, Done, or Milestones when drilled in):

- `]` jumps the cursor forward to the first task with a **different Sprint/iteration path** than the current task.
- `[` jumps backward to the previous iteration boundary.

When inside nav stack (drilled into subtasks), `[]` retains existing sibling navigation behaviour.

On the Milestones tab at group level, `[]` jumps between groups.

### `{}` — Type block jumping

- `}` jumps forward to the first task with a **different work-item type** (feature, bug, user-story, task, epic).
- `{` jumps backward.

This lets a tech lead quickly skip between feature blocks, bug blocks, etc. in a sorted list.

### `()` — Tag block jumping

- `)` jumps forward to the first task with a **different primary tag** than the current task.
- `(` jumps backward.

"Primary tag" is the first tag from the task's label list that appears in the configured `tags` list. Tasks with no configured tag are treated as a single "(untagged)" block.

This is the main tag interaction mechanism: combined with the existing `t` cycle filter (to focus on a single tag) and the Milestones tab (to see tag-grouped progress), `()` navigation provides fast movement _within_ a tag-aware view.

### Configuration

The existing `tags` config field determines the tag ordering for `()` navigation. Tags not in this list are sorted lexicographically after the configured ones.

```yaml
tags:
  - CapEx
  - OpEx
  - Milestone
  - blocked
  - tech-debt
```

### Visual feedback

When bracket navigation is active, the jumped-to field is briefly highlighted in the task list (matching grove's `highlightField` pattern):

- `[]` highlights the Sprint column (or iteration in group view)
- `{}` highlights the Type column
- `()` highlights the first matching tag

### Interaction with Milestones tab

On the Milestones tab at group level:

| Keys | Behaviour |
|------|-----------|
| `[]` | Jump between groups |
| `{}` | No-op (groups are already typed) |
| `()` | No-op (groups are already tag-based) |

When drilled into a milestone group, all three pairs work on the task list as normal.

## Alternatives Considered

**Use `{}` for iteration and `()` for type.** We considered matching grove's group→subject ordering, where `{}` maps to the coarser grouping (iterations) and `()` to finer (types). However, iteration paths tend to be fewer and more uniform than types, making type-jumping at the `{}` level and tag-jumping at `()` more useful. The chosen mapping also puts the most-requested feature (tag navigation) on the most ergonomic keys.

**Dedicated tag tab.** A separate tab for tag-based views was considered but rejected — the Milestones tab's tag mode already provides grouped progress, and `()` navigation provides fast movement on any tab. A dedicated tab would duplicate data without adding value.

**Tag column in task list.** Adding a visible tag column was considered. The task list already has 9 columns and adding another would crowd narrow terminals. Tags remain visible in the detail overlay (`i`) and searchable with `/`. If demand grows, a configurable column toggle could be added later.

## Consequences

- All three bracket pairs become active on task-list tabs. Users who don't need them can ignore them — they're non-destructive movement commands.
- The `tags` config gains a secondary purpose: ordering for `()` navigation. This incentivises users to list their most-used tags, improving both cycle-filter and bracket-nav experience.
- Implementation requires a `jumpBlock` kernel function (modelled on grove's `jumpTo`) that takes a field-extraction function and finds the next cursor position where the extracted value changes.
- No new API calls or data model changes. All bracket navigation operates on already-loaded task lists.
