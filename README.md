# canopy

A terminal UI for tech leads to track tasks in Azure Boards, with GitHub Issues, Jira, and Linear support planned. Get a unified view of your team's work and stay prepared for standups and meetings.

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

- **Multi-backend design**: Azure Boards today; GitHub Issues, Jira, and Linear backends are stubbed
- **Multi-profile**: define profiles per project/org and switch between them
- **Views**: config-driven filter presets for meetings — weekly standup, sprint review, or custom views; open one to see its tasks
- **My Tasks / Team / Done**: see your own work, your team's work, and completed items
- **Task drill-down**: navigate into tasks to see subtasks, with breadcrumb trail and sibling navigation
- **Detail overlay**: inline task detail view with dates, tags, state, and URL
- **Create work items**: inline form for creating tasks, features, bugs, and user stories with full field support
- **Cycle filters**: quickly narrow by date, assignee, type, or tag; date filters load older history when needed (start/target dates filter the loaded tasks)
- **Caching**: responses cached to disk for instant startup; tab state persisted across sessions
- **Update notifications**: footer shows when a newer release is available
- Catppuccin Mocha colour palette

## Installation

### Homebrew

```sh
brew tap alcxyz/tap
brew install canopy
```

### Build from source

Requires Go 1.26+.

```sh
git clone git@github.com:alcxyz/canopy.git
cd canopy
go build -o canopy ./cmd/canopy
mv canopy ~/.local/bin/
```

## Configuration

On first run canopy writes an example config, with its example profiles commented out, to `$XDG_CONFIG_HOME/canopy/config.yaml` (usually `~/.config/canopy/config.yaml`). Edit it to match your setup:

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

**Azure Boards** authenticates through the [Azure CLI](https://learn.microsoft.com/en-us/cli/azure/): install `az` and run `az login` before starting canopy. canopy requests a short-lived Azure DevOps token from `az` and keeps it in memory only; no personal access token is stored. For non-interactive use, log in with `az login --service-principal`.

Optional: set `azure_team` on the profile if your Azure DevOps team name differs from the default `"{project} Team"`.

### View filters

Views support these filter fields:

| Filter | Values | Example |
|--------|--------|---------|
| `updated_since` | `today`, `yesterday`, `last_week`, `last_2_weeks`, `last_month`, `last_quarter` | `last_week` |
| `types` | `feature`, `bug`, `user-story`, `task`, `epic` | `[feature, bug]` |
| `status` | `todo`, `in-progress`, `in-review`, `done`, `closed` | `[done, in-progress]` |
| `sprint` | `current`, `previous`, a sprint name, or a full iteration path | `current` |
| `assignee` | `me`, or a name/email | `me` |
| `labels` | tag names | `[frontend, priority-high]` |

### Files

| Path | Purpose |
|------|---------|
| `$XDG_CONFIG_HOME/canopy/config.yaml` | Configuration |
| `$XDG_CACHE_HOME/canopy/` | Cached task data |
| `$XDG_STATE_HOME/canopy/canopy.log` | Runtime log |

## Key bindings

### Navigation

| Key | Action |
|-----|--------|
| `j` / `k` | Move down / up |
| `h` / `l` | Previous / next tab |
| `gg` / `G` | First / last item |
| `ctrl+d` / `ctrl+u` | Half page down / up (`pgdn` / `pgup`: full page) |
| `1`–`4` | Switch to tab directly |
| `enter` | Navigate into task (show subtasks) / open view |
| `backspace` / `esc` | Navigate back / leave view (`esc` clears filters first) |
| `[` / `]` | Previous / next sibling task (when navigated in) |

### Filters

| Key | Action |
|-----|--------|
| `/` | Open text filter (type to search, enter to confirm, esc to clear) |
| `f` | Cycle by date (today → yesterday → week → month → quarter → 6mo); tabs load the last 7 days by default and widen as needed |
| `F` | Cycle date field (updated → created → start → target → closed → state changed) |
| `d` | Cycle by assignee |
| `s` | Cycle by type (feature, bug, user-story, task, epic) |
| `t` | Cycle by tag / label |
| `esc` | Clear all filters |

### Actions

| Key | Action |
|-----|--------|
| `c` | Create work item |
| `i` | Task detail overlay |
| `o` | Open task in browser |
| `space` | Copy task URL to clipboard |
| `r` | Refresh tasks |
| `!` | About: version, config path, cache and log locations |
| `?` | Help overlay |
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
