# ADR-003: TUI navigation and filtering model

**Status:** Accepted
**Date:** 2026-04-22
**Applies to:** `internal/app/`

## Context

The initial TUI had three tabs (My Tasks, Team, Views) with basic vim-style navigation (j/k/h/l/g/G) and no filtering. Task lists returned everything from the backend with no time scoping, causing the team tab to hit the 200-item WIQL cap with historical items. Closed tasks cluttered the active work views.

A sibling project, grove (a GitHub repository monitor TUI), has a mature navigation model with cycle filters, per-tab state, and context-aware keybindings that users are already familiar with.

## Decision

Adopt grove's navigation and filtering patterns:

**Tabs:** My Tasks, Team, Done, Views — with 1–4 direct-jump keys and h/l cycling. Active tabs (My Tasks, Team) exclude done/closed items by default. The Done tab shows only done/closed items. Views tab renders named filter presets from config.

**Cycle filters:** pressing a filter key cycles through unique values from the current dataset, wrapping to clear. Keys: `f` (date: this week / last week / this month / last month / this quarter / clear), `d` (assignee), `s` (work item type). `/` opens text search.

**Date scoping:** all tabs default to "this week" (`[System.ChangedDate] >= @today - 7`). The `f` cycle overrides this. This prevents unbounded queries from hitting the WIQL 200-item cap.

**Navigation:** j/k cursor, gg/G first/last, o open-in-browser, r refresh, q/ctrl+c quit — matching grove exactly.

## Alternatives Considered

**Keep the original three-tab layout with Views as the filtering mechanism.** Rejected because closed tasks dominate the list and there's no quick way to scope by time without creating many views.

**Add a filter bar with typed queries.** Rejected as over-engineered for the current use case. Cycle filters are faster for the small set of fields that matter (date, assignee, type).

## Consequences

- Users familiar with grove will be immediately productive in canopy.
- Per-tab filter state must be tracked (date range, assignee, type, text query per tab).
- The Views tab becomes less critical since cycle filters handle most ad-hoc filtering, but remains useful for meeting presets (standup, sprint review).
- Default time scoping means the team tab no longer hits the 200-item cap in normal use.
