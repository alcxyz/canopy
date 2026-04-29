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

| Backend | `backend:` value | Required fields | Status |
|---------|-----------------|-----------------|--------|
| Azure Boards | `azure-boards` | `org`, `project` | Implemented |
| GitHub Issues | `github` | `owner`, `repos` | Stubbed |
| Jira | `jira` | `url`, `project` | Stubbed |
| Linear | `linear` | `team_id` | Stubbed |

### Authentication

**Azure Boards** requires a Personal Access Token (PAT) with Work Items read scope. Provide it via:

1. Environment variable: `export AZURE_DEVOPS_PAT=your-token`
2. Token file: add `token_file: /path/to/pat` to the profile (works with sops-nix)

Optional: set `azure_team` on the profile if your Azure DevOps team name differs from the default `"{project} Team"`.

### View filters

Views support these filter fields:

| Filter | Values | Example |
|--------|--------|---------|
| `updated_since` | `today`, `yesterday`, `last_week`, `last_2_weeks`, `last_month`, `last_quarter` | `last_week` |
| `types` | `feature`, `bug`, `user-story`, `task`, `epic` | `[feature, bug]` |
| `status` | `todo`, `in-progress`, `in-review`, `done`, `closed` | `[done, in-progress]` |
| `sprint` | `current`, or a sprint/iteration name | `current` |
| `assignee` | `me`, or a name/email | `me` |
| `labels` | tag names | `[frontend, priority-high]` |

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
| `g` / `G` | First / last item |
| `1` `2` `3` | Switch to tab directly |
| `c` | Create work item |
| `r` | Refresh tasks |
| `enter` | Select view (Views tab) |
| `o` | Open task in browser |
| `q` / `ctrl+c` | Quit |

## Status

Azure Boards is fully implemented. GitHub Issues, Jira, and Linear backends are stubbed and ready for implementation.

## License

MIT. See [LICENSE](LICENSE).

<details>
<summary>Support</summary>

- **BTC:** `bc1pzdt3rjhnme90ev577n0cnxvlwvclf4ys84t2kfeu9rd3rqpaaafsgmxrfa`
- **ETH / ERC-20:** `0x2122c7817381B74762318b506c19600fF8B8372c`
</details>
