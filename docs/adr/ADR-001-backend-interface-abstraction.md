# ADR-001: Backend interface abstraction

**Status:** Accepted
**Date:** 2026-04-22
**Applies to:** `internal/backend/backend.go`

## Context

Canopy needs to pull tasks from multiple task-tracking systems (Azure Boards, Jira, GitHub Issues, Linear). Each system has different APIs, authentication, data models, and terminology (sprints vs cycles vs milestones, stories vs issues, etc.).

The tool should work with whichever system the user's team uses, and support multiple backends simultaneously (e.g. work Jira + personal GitHub Issues).

## Decision

Define a `Backend` interface with three methods: `ListTasks`, `ListSprints`, `ListTeam`. Each provider implements this interface, normalising its data into shared model types (`model.Task`, `model.Sprint`, `model.TeamMember`).

A `backend.New(profile)` factory function creates the right implementation based on `profile.Backend`. Profile-specific fields (`org`, `project`, `url`, `team_id`, etc.) live on the `config.Profile` struct and are validated at construction time.

Filters (`config.Filter`) use normalised values ("done", "in-progress", "feature", "bug") that each backend maps to its native equivalents internally.

## Alternatives Considered

**Separate binaries per backend.** Rejected because the whole point is a unified dashboard — splitting by backend defeats the purpose and makes multi-profile views impossible.

**Generic API client with config-driven field mapping.** Rejected as over-engineered. Each backend's API is different enough that a generic approach would be fragile and hard to debug. Concrete implementations are clearer.

**Plugin system (Go plugins or exec-based).** Rejected for now. The four backends cover the vast majority of teams. A plugin system adds complexity without clear near-term value. Can be revisited if community demand appears.

## Consequences

- Adding a new backend means implementing the `Backend` interface and adding a case to the factory.
- Each backend owns its own API calls, auth, and data mapping — no shared HTTP client or query builder.
- The normalised model types are a lowest-common-denominator; backend-specific metadata can be added to Task as needed.
- Filter interpretation varies by backend (e.g. "sprint: current" means different things in Jira vs Azure Boards vs Linear cycles).
