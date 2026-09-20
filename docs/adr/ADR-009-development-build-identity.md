# ADR-009: Identify development builds by source revision

**Status:** Accepted
**Date:** 2026-09-19
**Applies to:** `VERSION`, Go builds, Nix packaging, version output

## Context

`VERSION` is the release source of truth, but branch-based Nix packages used it
unchanged. A development build could therefore present itself as an official
release. Ordinary `go build` binaries only reported `dev`, which distinguished
them from releases but did not identify the source used for QA.

## Decision

- Keep `VERSION` and the GoReleaser release flow unchanged.
- Nix branch builds use `X.Y.Z-dev.<12-character-commit>`, adding `.dirty` for
  modified source. Source without Git metadata reports `X.Y.Z-dev.unknown`.
  Nix flake metadata does not reliably retain the requested tag ref, so the
  deliberate `release` package is exposed only for clean, identified source
  and keeps stable `X.Y.Z`. It is intended to be selected from the matching
  tag.
- Ordinary `go build` binaries derive `dev-<12-character-commit>[-dirty]` from
  Go's embedded VCS metadata and fall back to `dev` when it is unavailable.
- An explicitly injected version always wins, preserving GoReleaser releases
  and ensuring the CLI, footer, and About view report the package identity.

## Alternatives and consequences

Writing development hashes into `VERSION` would disturb release automation and
create source changes for every build. Showing only the release base version
would keep QA builds ambiguous. The selected formats preserve release tagging,
make development artifacts traceable, and remain excluded from release update
checks. An explicit Nix release output avoids guessing provenance from a commit
shared by branches and tags.
