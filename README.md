# canopy

A terminal UI for tech leads to track tasks across Azure Boards, Jira, GitHub Issues, and Linear. Get a unified view of your team's work and stay prepared for standups and meetings.

```
    _..._
   /     \    ___ __ _ _ __   ___  _ __  _   _
  | () () |  / __/ _` | '_ \ / _ \| '_ \| | | |
   \  ^  /  | (_| (_| | | | | (_) | |_) | |_| |
    |||||    \___\__,_|_| |_|\___/| .__/ \__, |
    |||||                         |_|    |___/
  the view from above
```

## Features

- **Multi-backend**: connect to Azure Boards, Jira, GitHub Issues, or Linear from a single dashboard
- **Multi-profile**: define profiles per project/org and switch between them
- **Views**: config-driven filter presets for meetings — weekly standup, sprint review, or custom views
- **My Tasks / Team**: see your own work and your team's work at a glance
- **Caching**: responses cached to disk for instant startup

## Installation

### Build from source

Requires Go 1.22+.

```sh
git clone git@github.com:alcxyz/canopy.git
cd canopy
go build -o canopy ./cmd/canopy
mv canopy ~/.local/bin/
```

## Configuration

On first run canopy writes an example config to `$XDG_CONFIG_HOME/canopy/config.yaml` (usually `~/.config/canopy/config.yaml`). Edit it to match your setup:

```yaml
profiles:
  - name: Work
    backend: azure-boards
    org: my-azure-org
    project: my-project
    team:
      - alice@example.com
      - bob@example.com

  - name: OSS
    backend: github
    owner: my-github-username
    repos:
      - my-repo

views:
  - name: Weekly Standup
    description: Features and fixes from the past week
    filters:
      updated_since: last_week
      types: [feature, bug, user-story]
      status: [done, in-review, in-progress]

  - name: My Tasks
    filters:
      assignee: me

refresh_secs: 300
```

### Supported backends

| Backend | `backend:` value | Required fields |
|---------|-----------------|-----------------|
| Azure Boards | `azure-boards` | `org`, `project` |
| GitHub Issues | `github` | `owner`, `repos` |
| Jira | `jira` | `url`, `project` |
| Linear | `linear` | `team_id` |

### Files

| Path | Purpose |
|------|---------|
| `$XDG_CONFIG_HOME/canopy/config.yaml` | Configuration |
| `$XDG_CACHE_HOME/canopy/` | Cached task data |
| `$XDG_STATE_HOME/canopy/canopy.log` | Runtime log |

## Key bindings

| Key | Action |
|-----|--------|
| `h` / `l` | Previous / next tab |
| `j` / `k` | Move down / up |
| `1` `2` `3` | Switch to tab directly |
| `q` / `ctrl+c` | Quit |

## Status

Canopy is in early development. Backend implementations are stubbed — Azure Boards will be the first to be fully implemented.

## License

MIT. See [LICENSE](LICENSE).
