# 🌙 teamoon

[![Go](https://img.shields.io/badge/Go-1.24%2B-00ADD8.svg)](https://go.dev/)
[![Version](https://img.shields.io/badge/Version-1.0.2-blue.svg)](VERSIONING.md)
[![Build](https://img.shields.io/github/actions/workflow/status/JuanVilla424/teamoon/ci.yml?branch=main&label=Build)](https://github.com/JuanVilla424/teamoon/actions)
[![Status](https://img.shields.io/badge/Status-Active-green.svg)]()
[![License](https://img.shields.io/badge/License-GPLv3-purple.svg)](LICENSE)

Terminal UI dashboard for monitoring Claude Code usage, managing development tasks, and tracking project status across your workspace.

## 📑 Table of Contents

- [Features](#-features)
- [Getting Started](#-getting-started)
  - [Prerequisites](#prerequisites)
  - [Installation](#installation)
  - [Environment Setup](#environment-setup)
  - [Pre-Commit Hooks](#pre-commit-hooks)
- [Usage](#-usage)
- [Configuration](#-configuration)
- [Contributing](#-contributing)
- [License](#-license)
- [Contact](#-contact)

## ✨ Features

- **TOKENS panel** — Real-time token consumption (input, output, cache read) with context window usage bar
- **USAGE panel** — Session counts and output volume aggregated by day, week, and month
- **QUEUE panel** — Local task management with priority levels (high/med/low) and time tracking
- **PROJECTS panel** — Auto-discovered projects from your workspace with git branch, modified files, and last commit info
- **Project actions menu** — Merge dependabot PRs, git pull, view open PRs, and create tasks per project
- **Autopilot engine** — Automated task execution via Claude CLI with planning, validation, and retry logic
- **CLI task management** — Add, complete, and list tasks directly from the command line

## 🚀 Getting Started

### Prerequisites

- [Go](https://go.dev/) 1.24+
- [Git](https://git-scm.com/)
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

### CLI Task Management

```bash
# Add a task
teamoon task add "implement auth module" -p cloud-adm -r high

# Mark task as done
teamoon task done 3

# List pending tasks
teamoon task list
```

### Keyboard Shortcuts

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

### Project Actions Menu

| Key         | Action                    |
| ----------- | ------------------------- |
| `1`         | Merge all dependabot PRs  |
| `2`         | Git pull                  |
| `3`         | View open PRs             |
| `4`         | Add task for this project |
| `esc` / `q` | Close menu                |

## ⚙️ Configuration

Configuration is stored at `~/.config/teamoon/config.json`.

| Field                  | Type   | Default      | Description                           |
| ---------------------- | ------ | ------------ | ------------------------------------- |
| `projects_dir`         | string | `~/Projects` | Directory to scan for projects        |
| `claude_dir`           | string | `~/.claude`  | Claude Code data directory            |
| `refresh_interval_sec` | int    | `30`         | Dashboard refresh interval in seconds |
| `budget_monthly`       | float  | `0`          | Monthly budget display (0 = hidden)   |
| `context_limit`        | int    | `0`          | Context window limit for usage bar    |

## 🤝 Contributing

We welcome contributions! Please read our [Contributing Guidelines](CONTRIBUTING.md) and [Code of Conduct](CODE_OF_CONDUCT.md) before getting started.

## 📜 License

2026 - This project is licensed under the [GNU General Public License v3.0](https://www.gnu.org/licenses/gpl-3.0.en.html). You are free to use, modify, and distribute this software under the terms of the GPL-3.0 license. For more details, please refer to the [LICENSE](LICENSE) file included in this repository.

## 📬 Contact

For questions, suggestions, or issues:

- **GitHub Issues**: [github.com/JuanVilla424/teamoon/issues](https://github.com/JuanVilla424/teamoon/issues)
- **Email**: [r6ty5r296it6tl4eg5m.constant214@passinbox.com](mailto:r6ty5r296it6tl4eg5m.constant214@passinbox.com)
