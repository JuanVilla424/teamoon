# teamoon — Architecture

## Overview

teamoon is a terminal UI dashboard built with the **Elm Architecture** pattern via [Bubbletea](https://github.com/charmbracelet/bubbletea). It follows a strict `Model → Update → View` cycle for all UI state management, combined with [Cobra](https://github.com/spf13/cobra) for CLI command routing.

## Package Structure

```
teamoon/
├── cmd/teamoon/
│   └── main.go              # Entry point, Cobra CLI commands
├── internal/
│   ├── config/
│   │   └── config.go        # JSON config load/save (~/.config/teamoon/)
│   ├── dashboard/
│   │   ├── model.go         # Bubbletea Model, Init, data types
│   │   ├── update.go        # Bubbletea Update (key handling, messages)
│   │   ├── view.go          # Bubbletea View (panel rendering)
│   │   └── styles.go        # Lipgloss styles, progress bars
│   ├── metrics/
│   │   ├── tokens.go        # JSONL session parsing, token aggregation
│   │   └── cost.go          # Usage/cost summary calculation
│   ├── projects/
│   │   └── projects.go      # Directory scanning, git ops, GitHub PRs
│   └── queue/
│       └── tasks.go         # JSON-based task persistence
├── Makefile
├── go.mod
└── go.sum
```

## Package Responsibilities

### `cmd/teamoon`

Entry point. Defines two command trees via Cobra:

- **Root command** — Launches the TUI dashboard (`tea.NewProgram`)
- **`task` subcommand** — CLI task management (`add`, `done`, `list`)

### `internal/config`

Manages `~/.config/teamoon/config.json`. Creates default config on first run. Fields control: projects directory, Claude data directory, refresh interval, budget, and context limit.

### `internal/dashboard`

Implements the Bubbletea Elm Architecture:

- **Model** (`model.go`) — Holds all UI state: metrics, projects, tasks, cursor positions, menu state, terminal dimensions. Parses `CLAUDE_CODE_MODEL` and `CLAUDE_CODE_EFFORT_LEVEL` env vars for header display.
- **Update** (`update.go`) — Handles all messages: key events, window resize, data refresh, PR operations, task mutations. Separates main view keys, menu keys, and input mode keys into distinct handlers.
- **View** (`view.go`) — Renders four panels (TOKENS, USAGE, QUEUE, PROJECTS) plus a project actions menu overlay. Uses Lipgloss for layout (`JoinHorizontal`) and styling.
- **Styles** (`styles.go`) — Centralized Lipgloss style definitions. Includes color-coded progress bars (green/yellow/red) for context window visualization.

### `internal/metrics`

Reads Claude Code session data:

- **Token scanning** — Parses `.jsonl` files from `~/.claude/projects/*/` directories. Aggregates input, output, cache read, and cache creation tokens by day/week/month. Tracks session counts per period.
- **Active session** — Finds the most recently modified `.jsonl` file and calculates context window usage percentage.
- **Cost** — Wraps token summaries with session counts and budget info.

### `internal/projects`

Scans the configured projects directory:

- **Directory scan** — Lists all subdirectories. For git repos: extracts branch, last commit, modified file count, and GitHub remote URL. Non-git directories are included with minimal info.
- **GitHub integration** — Uses `gh` CLI to fetch open PRs, filter dependabot PRs, and merge PRs.
- **Git operations** — Executes `git pull` on selected projects.

### `internal/queue`

JSON-based task persistence at `~/.config/teamoon/tasks.json`:

- Auto-incrementing IDs via `TaskStore.NextID`
- Priority levels: `high`, `med`, `low`
- Operations: `Add`, `MarkDone`, `ListPending`, `ListAll`

## Data Flow

```
Startup
  │
  ├─ config.Load() ──► ~/.config/teamoon/config.json
  │
  ├─ dashboard.NewModel(cfg)
  │     ├─ Parse CLAUDE_CODE_MODEL env
  │     └─ Parse CLAUDE_CODE_EFFORT_LEVEL env
  │
  └─ tea.NewProgram(model).Run()
        │
        ├─ Init() ──► refreshData + tickEvery(30s)
        │
        └─ [Event Loop]
              │
              ├─ refreshMsg / tickMsg
              │     └─ fetchData(cfg)
              │           ├─ metrics.ScanTokens()     ◄── ~/.claude/projects/**/*.jsonl
              │           ├─ metrics.ScanActiveSession()
              │           ├─ projects.Scan()           ◄── ~/Projects/*/
              │           └─ queue.ListPending()       ◄── ~/.config/teamoon/tasks.json
              │
              ├─ KeyMsg ──► handleMainKey / handleMenuKey / handleInputKey
              │
              └─ dataMsg ──► Update model state ──► View() renders panels
```

## Design Decisions

| Decision                         | Rationale                                                       |
| -------------------------------- | --------------------------------------------------------------- |
| **JSON config** (not YAML)       | Go stdlib `encoding/json` — zero external deps for config       |
| **`internal/` packages**         | Enforces encapsulation; prevents external imports               |
| **`gh` CLI** (not GitHub API)    | Reuses user's existing auth; no token management needed         |
| **JSON task store** (not SQLite) | Simple, human-readable, no CGo dependency                       |
| **Cobra CLI**                    | Standard Go CLI framework; subcommand support for `task` tree   |
| **Lipgloss styling**             | Same Charm ecosystem as Bubbletea; consistent API               |
| **Periodic tick refresh**        | Configurable interval; avoids file watchers complexity          |
| **Non-git projects included**    | All directories in projects_dir are shown; git info is optional |
