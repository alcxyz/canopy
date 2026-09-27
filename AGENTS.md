# Canopy repository instructions

- GitHub `origin` is the development, CI, and release host. Forgejo is a secondary continuity remote; follow
  [CONTRIBUTING.md](CONTRIBUTING.md) for the GitHub workflow.
- Develop from `dev` and target GitHub pull requests at `dev`; update `VERSION` before integrating a release into
  `main`.
- Read the [ADR index](docs/adr/README.md) and relevant accepted decisions before lasting design changes,
  especially [ADR-001](docs/adr/ADR-001-backend-interface-abstraction.md) for provider work.
- Keep backends behind `internal/backend` and normalized task types in `internal/model`; the current package map is
  in [CONTRIBUTING.md](CONTRIBUTING.md).
- For Go changes, run `gofmt`, `go test -race ./...`, and `go vet ./...`; CI also builds, lints, checks module
  tidiness, and validates release archives. See [.github/workflows/ci.yml](.github/workflows/ci.yml).
- The Azure Boards backend uses Azure CLI authentication under
  [ADR-002](docs/adr/ADR-002-azure-auth-via-az-cli.md); avoid reviving PAT configuration from stale README
  examples.
