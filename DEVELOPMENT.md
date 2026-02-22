## 🛠️ Development

Development environment details, Claude Code integrations, and reference material for contributors.

---

## 📦 System Packages Reference

| Package                                          | Purpose                                                                |
| ------------------------------------------------ | ---------------------------------------------------------------------- |
| `git`, `curl`, `wget`                            | Version control and downloads                                          |
| `zip`, `unzip`                                   | Archive handling                                                       |
| `build-essential`, `gcc`, `g++`, `make`, `cmake` | C/C++ toolchain (Go CGo, Python C extensions, Rust deps)               |
| `lib*-dev`, `uuid-dev`                           | Build dependencies for pyenv, Node native modules, Python C extensions |
| `jq`                                             | JSON processing in scripts and pipelines                               |
| `htop`, `tree`                                   | System monitoring and directory visualization                          |
| `tmux`                                           | Terminal multiplexer for persistent sessions                           |
| `shellcheck`                                     | Shell script linting (used by pre-commit hooks)                        |
| `yamllint`                                       | YAML validation (CloudFormation, configs)                              |
| `pre-commit`                                     | Git hook framework                                                     |
| `ca-certificates`, `gnupg`, `lsb-release`        | Repository signing and HTTPS transport                                 |
| `pkg-config`                                     | Library discovery for native builds                                    |

---

## 🛠️ Dev Environment Setup

### Rust (optional)

```bash
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
source ~/.cargo/env
```

Verifies: `rustc --version`, `cargo --version`

### pyenv + Python 3.11 (optional)

```bash
curl https://pyenv.run | bash
```

Add to `~/.bashrc`:

```bash
export PYENV_ROOT="$HOME/.pyenv"
[[ -d $PYENV_ROOT/bin ]] && export PATH="$PYENV_ROOT/bin:$PATH"
eval "$(pyenv init -)"
```

Install Python:

```bash
source ~/.bashrc
pyenv install 3.11
pyenv global 3.11
```

Verifies: `python --version` (3.11+)

---

## 🧠 Claude Code Skills

teamoon's autopilot engine works best with a curated set of agent skills installed. These enhance Claude Code's planning, debugging, TDD, frontend, and browser automation capabilities.

Skills are **automatically installed** during onboarding (`teamoon init` or the web setup wizard).

For manual skill installation only: `go run ./cmd/install-skills`

Browse all available skills at [skills.sh](https://skills.sh/).

### Superpowers (obra/superpowers) — 14 Skills

Core development workflow skills for planning, debugging, TDD, code review, and parallel execution.

```bash
npx skills add obra/superpowers@brainstorming -g -y
npx skills add obra/superpowers@systematic-debugging -g -y
npx skills add obra/superpowers@writing-plans -g -y
npx skills add obra/superpowers@test-driven-development -g -y
npx skills add obra/superpowers@executing-plans -g -y
npx skills add obra/superpowers@requesting-code-review -g -y
npx skills add obra/superpowers@using-superpowers -g -y
npx skills add obra/superpowers@subagent-driven-development -g -y
npx skills add obra/superpowers@verification-before-completion -g -y
npx skills add obra/superpowers@receiving-code-review -g -y
npx skills add obra/superpowers@using-git-worktrees -g -y
npx skills add obra/superpowers@writing-skills -g -y
npx skills add obra/superpowers@dispatching-parallel-agents -g -y
npx skills add obra/superpowers@finishing-a-development-branch -g -y
```

### Anthropic Official (anthropics/skills) — 2 Skills

```bash
npx skills add anthropics/skills@frontend-design -g -y
npx skills add anthropics/skills@skill-creator -g -y
```

### Vercel (vercel-labs/agent-skills + agent-browser) — 5 Skills

Frontend best practices, design systems, composition patterns, and browser automation.

```bash
npx skills add vercel-labs/agent-skills@vercel-react-best-practices -g -y
npx skills add vercel-labs/agent-skills@web-design-guidelines -g -y
npx skills add vercel-labs/agent-skills@vercel-composition-patterns -g -y
npx skills add vercel-labs/agent-skills@vercel-react-native-skills -g -y
npx skills add vercel-labs/agent-browser@agent-browser -g -y
```

### Skills Summary

| Pack            | Skills | Purpose                                                                           |
| --------------- | ------ | --------------------------------------------------------------------------------- |
| **superpowers** | 14     | Planning, TDD, debugging, code review, parallel agents, worktrees                 |
| **anthropics**  | 2      | Frontend design, skill creation                                                   |
| **vercel**      | 5      | React/Next.js, web design, composition patterns, React Native, browser automation |

Skills are installed globally at `~/.agents/skills/` and are automatically available to Claude Code sessions spawned by the autopilot.

---

## 🔒 Claude Code Hooks

teamoon includes security hooks that are installed **globally** via onboarding and **per-project** when scaffolding new projects. Hooks validate Claude Code actions **before execution** — they work even with `--dangerously-skip-permissions`.

### Included Hooks

| Hook             | Trigger    | Protection                                                                                                                                                                                    |
| ---------------- | ---------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `security-check` | Bash       | Force push, reset --hard, --no-verify, blind staging, destructive rm, remote code execution, SQL drops, docker prune, kill -9, sudo escalation, credential reading, AWS/cloud destructive ops |
| `test-guard`     | Bash       | Blocks commit without running tests first                                                                                                                                                     |
| `secrets-guard`  | Write/Edit | Blocks writing .env, credentials, SSH keys, cloud configs, DB passwords, key/cert files, lock files, shell history                                                                            |
| `build-guard`    | Bash       | Blocks push without building first                                                                                                                                                            |
| `commit-format`  | Bash       | Enforces `type(core): lowercase description` format, no emojis, no uppercase                                                                                                                  |

### Where Hooks Live

Global hooks (installed by onboarding):

```
~/.config/teamoon/hooks/        # Actual files (teamoon home)
├── security-check.sh
├── test-guard.sh
├── secrets-guard.sh
├── build-guard.sh
└── commit-format.sh

~/.claude/hooks/{name}.sh       # Per-file symlinks → ~/.config/teamoon/hooks/
~/.claude/settings.json         # hooks.PreToolUse entries (merged, not symlinked)
```

Per-project hooks (installed by project init):

```
{project}/.claude/
├── settings.json
└── hooks/
    ├── security-check.sh
    ├── test-guard.sh
    ├── secrets-guard.sh
    ├── build-guard.sh
    └── commit-format.sh
```

Per-project hooks are committed to the repo — anyone who clones the project gets them automatically.

### Manual Installation (existing projects)

For projects created before hooks were added:

```bash
cd /path/to/teamoon
go run ./cmd/install-hooks /path/to/target-project
```

---

## 📜 License

2026 - This project is licensed under the [GNU General Public License v3.0](https://www.gnu.org/licenses/gpl-3.0.en.html). You are free to use, modify, and distribute this software under the terms of the GPL-3.0 license. For more details, please refer to the [LICENSE](LICENSE) file included in this repository.
