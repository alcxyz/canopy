# Contributing to canopy

## Development setup

Prerequisites: Go 1.26+, [`az` CLI](https://learn.microsoft.com/en-us/cli/azure/) (canopy uses it for Azure Boards authentication)

```bash
git clone https://github.com/alcxyz/canopy.git
cd canopy
go build ./cmd/canopy
```

## Running tests

```bash
go test ./...
```

## Vetting

CI runs `go vet`. Run it locally to catch issues before pushing:

```bash
go vet ./...
```

## Project structure

- `cmd/canopy/` -- entry point, config example
- `cmd/smoke/` -- headless backend smoke test
- `internal/app/` -- core TUI application (commands, filtering, update, view, form)
- `internal/backend/` -- multi-backend abstraction (Azure Boards, GitHub Issues, Jira, Linear)
- `internal/cache/` -- task caching
- `internal/config/` -- YAML config loading
- `internal/model/` -- task model
- `internal/platform/` -- browser and clipboard integration
- `internal/buildinfo/` -- build version identity
- `internal/ui/` -- help, splash, detail overlays, tabs, styles, helpers

## Making changes

1. Fork the repo and create a branch from `dev`
2. Make your changes
3. Add or update tests as needed
4. Run `go test ./...` and `go vet ./...`
5. Open a pull request against `dev`

CI runs build, vet, and tests. All checks must pass before merging.

## Commit messages

Use conventional-ish prefixes to keep history scannable:

- `feat:` new feature
- `fix:` bug fix
- `docs:` documentation only
- `chore:` maintenance, CI, dependencies
- `refactor:` code changes that don't add features or fix bugs

## Releasing

Releases are automated via [GoReleaser](https://goreleaser.com/) and GitHub Actions. The `VERSION` file is the single source of truth.

Development builds identify their source revision. Ordinary `go build` output
uses `dev-<commit>[-dirty]`; branch-based Nix packages use
`X.Y.Z-dev.<commit>[.dirty]`. GoReleaser artifacts retain the plain `X.Y.Z`
release version; an intentional Nix release build uses `.#release` from clean,
identified release source.

To cut a release:

1. Bump the `VERSION` file on `dev`
2. Open a same-repository pull request from `dev` to `main`
3. Once its checks pass, merge it by fast-forward: push the pull request's
   head to `main` (`git push origin <head sha>:main`), which GitHub records as
   merging it. Never use GitHub's merge, squash or rebase buttons: they write
   new commits, and `main` must only change through `dev`
4. CI automatically creates the git tag and runs GoReleaser

This builds binaries for linux/darwin x amd64/arm64, creates a GitHub release with changelog, updates the cask in the [Homebrew tap](https://github.com/alcxyz/homebrew-tap), and publishes to the [AUR](https://aur.archlinux.org/packages/canopy-bin) (`canopy-bin`).

### Version numbering

Follow [semver](https://semver.org/):

- **Patch** (`v0.5.x`): bug fixes, minor tweaks
- **Minor** (`v0.x.0`): new features, non-breaking changes
- **Major** (`vx.0.0`): breaking changes to config format, CLI flags, or behavior

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
