# 🌙 teamoon

[![Go](https://img.shields.io/badge/Go-1.24%2B-00ADD8.svg)](https://go.dev/)
[![Version](https://img.shields.io/badge/Version-1.0.4-blue.svg)](VERSIONING.md)
[![Build](https://img.shields.io/github/actions/workflow/status/JuanVilla424/teamoon/go.yml?branch=dev&label=Build)](https://github.com/JuanVilla424/teamoon/actions)
[![Status](https://img.shields.io/badge/Status-Active-green.svg)]()
[![License](https://img.shields.io/badge/License-GPLv3-purple.svg)](LICENSE)

Terminal UI dashboard and web interface for monitoring Claude Code usage, managing development tasks, and running autopilot across your workspace.

## 📑 Table of Contents

- [Features](#-features)
- [Getting Started](#-getting-started)
  - [Prerequisites](#prerequisites)
  - [Installation](#installation)
  - [Environment Setup](#environment-setup)
  - [Pre-Commit Hooks](#pre-commit-hooks)
- [Usage](#-usage)
  - [TUI Dashboard](#tui-dashboard)
  - [Web Dashboard](#web-dashboard)
  - [CLI Task Management](#cli-task-management)
- [Configuration](#-configuration)
- [CI/CD](#-cicd)
- [Contributing](#-contributing)
- [Contact](#-contact)
- [License](#-license)

## ✨ Features

- **TOKENS panel** — Real-time token consumption (input, output, cache read) with context window usage bar
- **USAGE panel** — Session counts and output volume aggregated by day, week, and month
- **QUEUE panel** — Local task management with priority levels (high/med/low) and time tracking
- **PROJECTS panel** — Auto-discovered projects from your workspace with git branch, modified files, and last commit info
- **Project actions menu** — Merge dependabot PRs, git pull, view open PRs, and create tasks per project
- **Web Dashboard** — Browser-based UI with SSE real-time updates, task management, chat, MCP server config, and skeleton settings
- **Autopilot engine** — Automated task execution via Claude CLI with multi-step planning, validation, retry logic, and configurable step timeouts
- **Autopilot recovery** — Tasks in progress automatically resume after service restarts
- **MCP server management** — Configure and toggle MCP servers from the web UI
- **Skeleton steps** — Configurable execution phases per project (investigate, context7, implement, build, test, pre-commit, commit, push)
- **CLI task management** — Add, complete, and list tasks directly from the command line

## 🚀 Getting Started

### Prerequisites

- [Go](https://go.dev/) 1.24+
- [Git](https://git-scm.com/)
- [Make](https://www.gnu.org/software/make/)
- [`gh` CLI](https://cli.github.com/) (optional, for GitHub PR features)

### Installation

```bash
git clone https://github.com/JuanVilla424/teamoon.git
cd teamoon
make build
make install
```

### Environment Setup

Configuration is automatically created at `~/.config/teamoon/config.json` on first run. See [Configuration](#-configuration) for available options.

### Initialize (Skills + Hooks)

```bash
teamoon init
```

This installs 21 curated Claude Code agent skills globally (superpowers + anthropic + vercel). See [INSTALL.md](INSTALL.md) for the full list and manual installation instructions.

### Make Targets

| Target    | Description                               |
| --------- | ----------------------------------------- |
| `build`   | Compile the binary                        |
| `test`    | Run all tests (`go test ./internal/...`)  |
| `install` | Build + copy binary to `/usr/local/bin`   |
| `service` | Build + install + restart systemd service |
| `release` | Cross-compile binary (used by CI/CD)      |
| `clean`   | Remove compiled binary                    |

### Pre-Commit Hooks

```bash
pip install pre-commit
pre-commit install
pre-commit install --hook-type pre-push
```

## 📖 Usage

### TUI Dashboard

```bash
teamoon
```

#### Keyboard Shortcuts

| Key         | Action                                        |
| ----------- | --------------------------------------------- |
| `esc` / `q` | Quit                                          |
| `r`         | Refresh data                                  |
| `tab`       | Switch focus between QUEUE and PROJECTS       |
| `Up/Down`   | Navigate items                                |
| `enter`     | Open project actions / view task detail       |
| `d`         | Mark task as done (in QUEUE)                  |
| `a`         | Toggle auto-pilot on selected task (in QUEUE) |
| `ctrl+a`    | Toggle auto-pilot on ALL tasks (in QUEUE)     |
| `p`         | Generate plan for task (in QUEUE)             |
| `x`         | Replan task (in QUEUE)                        |
| `e`         | Archive task (in QUEUE)                       |

#### Project Actions Menu

| Key         | Action                    |
| ----------- | ------------------------- |
| `1`         | Merge all dependabot PRs  |
| `2`         | Git pull                  |
| `3`         | View open PRs             |
| `4`         | Add task for this project |
| `esc` / `q` | Close menu                |

### Web Dashboard

```bash
teamoon serve
```

The web UI is available at `http://localhost:7777` (configurable via `web_port`). Features include:

- Real-time task queue with SSE updates
- Task creation, editing, planning, and autopilot controls
- Per-project autopilot start/stop and skeleton configuration
- Chat interface for interacting with Claude
- MCP server management (list, toggle, install from catalog)
- Skills catalog and installation
- Configuration editor
- Optional basic auth via `web_password`

### CLI Task Management

```bash
# Add a task
teamoon task add "implement auth module" -p cloud-adm -r high

# Mark task as done
teamoon task done 3

# List pending tasks
teamoon task list
```

## ⚙️ Configuration

Configuration is stored at `~/.config/teamoon/config.json`.

| Field                  | Type   | Default      | Description                              |
| ---------------------- | ------ | ------------ | ---------------------------------------- |
| `projects_dir`         | string | `~/Projects` | Directory to scan for projects           |
| `claude_dir`           | string | `~/.claude`  | Claude Code data directory               |
| `refresh_interval_sec` | int    | `30`         | Dashboard refresh interval in seconds    |
| `budget_monthly`       | float  | `0`          | Monthly budget display (0 = hidden)      |
| `context_limit`        | int    | `0`          | Context window limit for usage bar       |
| `web_enabled`          | bool   | `false`      | Enable web dashboard on startup          |
| `web_port`             | int    | `7777`       | Web dashboard port                       |
| `web_password`         | string | `""`         | Basic auth password (empty = no auth)    |
| `webhook_url`          | string | `""`         | Webhook URL for task event notifications |
| `max_concurrent`       | int    | `3`          | Max concurrent autopilot sessions        |

### Spawn Settings (`spawn`)

| Field              | Type   | Default | Description                                    |
| ------------------ | ------ | ------- | ---------------------------------------------- |
| `model`            | string | `""`    | Claude model override (empty = default)        |
| `effort`           | string | `""`    | Effort level override (empty = default)        |
| `max_turns`        | int    | `25`    | Max agentic turns per step                     |
| `step_timeout_min` | int    | `5`     | Max minutes per step before timeout (0 = none) |

### Skeleton Settings (`skeleton`)

Configurable per-project via `project_skeletons` map.

| Field             | Type | Default | Description                         |
| ----------------- | ---- | ------- | ----------------------------------- |
| `web_search`      | bool | `true`  | Enable web search step              |
| `context7_lookup` | bool | `true`  | Enable Context7 library lookup step |
| `build_verify`    | bool | `true`  | Enable build verification step      |
| `test`            | bool | `true`  | Enable test execution step          |
| `pre_commit`      | bool | `true`  | Enable pre-commit hooks step        |
| `commit`          | bool | `true`  | Enable auto-commit step             |
| `push`            | bool | `false` | Enable auto-push step               |

## 🔄 CI/CD

### Workflows

| Workflow               | File                     | Trigger                        | Description                                            |
| ---------------------- | ------------------------ | ------------------------------ | ------------------------------------------------------ |
| **Go**                 | `go.yml`                 | Push to dev/test/prod/main, PR | Go vet, tests (`make test`), build (`make build`)      |
| **CI**                 | `ci.yml`                 | Push to main, PR               | Go vet, tests, build on main branch                    |
| **Version Controller** | `version-controller.yml` | Push to dev/test/prod/main     | Auto version bump, tag creation, PR promotion          |
| **Release Controller** | `release-controller.yml` | Tag `v*.*.*` (main only)       | Cross-compiled binaries (5 platforms) + GitHub Release |

### Release Artifacts

On each release tag, binaries are built for:

| Platform | Architecture | Archive |
| -------- | ------------ | ------- |
| Linux    | amd64        | .tar.gz |
| Linux    | arm64        | .tar.gz |
| macOS    | amd64        | .tar.gz |
| macOS    | arm64        | .tar.gz |
| Windows  | amd64        | .zip    |

Each release includes a `SHA256SUMS` file for integrity verification.

### Branch Promotion

`dev` → `test` → `prod` → `main`

Version bump keywords in commit messages:

- `[major candidate]` — Major version bump
- `[minor candidate]` — Minor version bump
- `[patch candidate]` — Patch version bump

## 🤝 Contributing

We welcome contributions! Please read our [Contributing Guidelines](CONTRIBUTING.md) and [Code of Conduct](CODE_OF_CONDUCT.md) before getting started.

## 📬 Contact

For questions, suggestions, or issues:

- **GitHub Issues**: [github.com/JuanVilla424/teamoon/issues](https://github.com/JuanVilla424/teamoon/issues)
- **Email**: [r6ty5r296it6tl4eg5m.constant214@passinbox.com](mailto:r6ty5r296it6tl4eg5m.constant214@passinbox.com)

## 📜 License

2026 - This project is licensed under the [GNU General Public License v3.0](https://www.gnu.org/licenses/gpl-3.0.en.html). You are free to use, modify, and distribute this software under the terms of the GPL-3.0 license. For more details, please refer to the [LICENSE](LICENSE) file included in this repository.
