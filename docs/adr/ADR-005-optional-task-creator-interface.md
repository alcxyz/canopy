# ADR-005: Optional TaskCreator interface for write operations

**Status:** Accepted
**Date:** 2026-04-23
**Applies to:** `internal/backend/backend.go`, `internal/backend/azure.go`

## Context

Canopy has been read-only since inception (ADR-001). Users now want to create work items from the TUI — starting with Azure Boards, where the typical workflow is creating a Feature with User Stories underneath it. The existing `Backend` interface defines three read methods (`ListTasks`, `ListSprints`, `ListTeam`). Adding write operations raises the question of how to extend the interface without breaking backends that don't support creation (GitHub, Jira, and Linear are currently stubs).

## Decision

Define a separate `TaskCreator` interface rather than extending the existing `Backend` interface. The TUI checks at runtime with a type assertion (`creator, ok := b.(TaskCreator)`). Backends that support creation implement both `Backend` and `TaskCreator`; those that don't need no changes.

```go
type TaskCreator interface {
    CreateTask(ctx context.Context, params CreateTaskParams) (CreateTaskResult, error)
    CurrentIteration(ctx context.Context) (string, error)
}
```

`CurrentIteration` is bundled here because iteration context is needed to pre-populate the creation form and is only relevant when write operations are supported.

The TUI form accepts plain text for descriptions; the backend is responsible for converting to its native format (HTML for Azure DevOps).

## Alternatives Considered

**Extend the `Backend` interface directly.** Rejected because it forces all stub backends to implement `CreateTask` with a `not implemented` error. This is noisy and error-prone as more backends are added. The optional interface pattern is idiomatic Go (cf. `io.Reader` vs `io.ReadCloser`).

**Separate CLI subcommand instead of TUI form.** Rejected because the value of canopy is the unified dashboard — switching to a CLI for creation breaks the flow. The form overlay follows the existing overlay pattern (`showHelp`, `showDetail`) and feels native.

**Use `az boards` CLI under the hood.** Rejected because it adds an external dependency and doesn't generalise to other backends. Direct API calls using the existing `doRequest` infrastructure are more reliable and consistent.

## Consequences

- Adding write support to a new backend means implementing `TaskCreator` — no changes to the core `Backend` interface or existing backends.
- The TUI must check for `TaskCreator` support at runtime and gracefully disable the `c` key when no backend supports it.
- Future write operations (update, delete) can follow the same optional interface pattern.
- Description formatting is a backend concern — the TUI always works with plain text.
