# ADR-002: Azure authentication via az CLI

**Status:** Accepted
**Date:** 2026-04-22
**Applies to:** `internal/backend/azure.go`

## Context

The Azure Boards backend currently authenticates using a Personal Access Token (PAT) sourced from either the `AZURE_DEVOPS_PAT` environment variable or a file path specified in the profile config (`token_file`). PATs are long-lived secrets that must be manually rotated, stored somewhere on disk, and re-issued when they expire. In organisations that enforce Conditional Access or SSO/MFA, PATs can also be outright blocked or scoped in ways that require re-approval over time.

Users interacting with Azure DevOps from a terminal already have `az` (azure-cli) installed for other Azure work. When logged in via `az login`, the CLI holds a short-lived OAuth token that is automatically refreshed. Canopy can delegate auth entirely to that existing session.

## Decision

Acquire Azure DevOps access tokens by shelling out to `az account get-access-token --resource 499b84ac-1321-427f-aa17-267ca6975798` (the Azure DevOps resource ID). The returned JSON contains an `accessToken` field which is used as a Bearer token on all requests instead of Basic auth with a PAT.

This becomes the primary auth path for the Azure Boards backend. PAT-based auth (`AZURE_DEVOPS_PAT` / `token_file`) is removed; users who need non-interactive auth (CI, service accounts) should configure a credential via `az login --service-principal`.

## Alternatives Considered

**Keep PAT in env var or file (current approach).** Rejected because long-lived secrets in files or shell history are an unnecessary risk, and PATs don't work in orgs with Conditional Access without extra admin steps.

**Built-in OAuth device-flow.** Rejected as significantly more complex to implement (OAuth client registration, token storage, refresh logic) with no advantage over the az CLI path for users who already have it installed.

**Service principal client credentials in config.** Rejected for the same reasons as PATs — long-lived secrets in config files. The az CLI already handles service principal login cleanly via `az login --service-principal`.

**Support both az CLI and PAT as fallback.** Rejected to keep the auth surface simple and auditable. A fallback chain obscures which credential is actually in use and re-introduces the secrets-in-files problem for anyone who configures a PAT "just in case".

## Consequences

- `az` must be installed and the user must be logged in (`az login`) for the Azure Boards backend to work. A clear, actionable error is returned if the command fails or produces no token.
- `token_file` and `AZURE_DEVOPS_PAT` config fields are removed from the profile schema.
- HTTP auth changes from Basic (`Authorization: Basic base64(:PAT)`) to Bearer (`Authorization: Bearer <access_token>`).
- Tokens are short-lived (~1 hour); az CLI handles refresh transparently. No token caching is needed in Canopy.
- Non-interactive environments (CI, cron) work via `az login --service-principal` — no Canopy-specific secret handling required.
