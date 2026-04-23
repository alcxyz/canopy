# ADR-004: Task drill-down navigation

**Status:** Accepted
**Date:** 2026-04-23
**Applies to:** `internal/app/update.go`, `internal/app/view.go`, `internal/app/model.go`

## Context

ADR-003 established the TUI navigation model with tabs, cycle filters, and vim-style cursor movement. However, tasks with parent-child relationships (epics > user stories > subtasks) could only be viewed in a flat list with a PARENT column. There was no way to navigate into a task to see its subtasks or move between sibling tasks without scrolling through the full list.

The sibling project grove has a proven drill-down navigation pattern: Enter navigates into an item showing its children, Esc goes back, and `[`/`]` move between siblings. Users are already familiar with this pattern.

## Decision

Add a navigation stack (`navStack []model.Task`) that tracks the chain of parent tasks the user has drilled into:

**Enter** pushes the selected task onto the stack. The view then shows only tasks whose `ParentID` matches that task's `ID`, searching across all loaded task lists (myTasks, teamTasks, doneTasks) with deduplication. A breadcrumb trail renders above the task list showing the full path (e.g. `My Tasks > Epic Title > Story Title`).

**Esc** pops the stack (after clearing any active filters first). **Backspace** always pops immediately without the filter-clearing precedence.

**`[` / `]`** replace the top of the stack with the previous/next sibling task. Siblings are computed from the grandparent level (or the tab's root list for depth-1 navigation). This works in both the main view and the detail overlay.

**`i`** opens the detail overlay (previously bound to Enter). Inside the overlay, `[`/`]` move between tasks and `enter` drills into the displayed task.

Tab switching (`h`/`l`/`1-4`) clears the navigation stack entirely.

## Alternatives Considered

**Filter-based approach (filter by ParentID).** Would reuse existing filter machinery but conflates navigation context with search filters. Pressing Esc would be ambiguous: does it clear the parent filter or the text search? A separate stack keeps navigation orthogonal to filtering.

**Tree view with expand/collapse.** More traditional but harder to implement with the current flat-list rendering and would require significant view refactoring. The drill-down approach reuses the existing list renderer and matches grove's proven pattern.

## Consequences

- Users can explore task hierarchies by drilling into parent tasks and navigating between siblings.
- The navigation stack is independent of filters — users can filter within a navigated level.
- Child lookup is O(n) over all loaded tasks per render. This is acceptable for typical task counts (< 1000) but may need indexing if datasets grow significantly.
- The detail overlay trigger moved from Enter to `i`, which is a keybinding change users need to learn.
