## 🔨 Installation

### Prerequisites

- Ubuntu/Debian-based system
- sudo access

---

## 📦 System Packages

```bash
sudo apt update && sudo apt install -y \
  git curl wget zip unzip \
  build-essential gcc g++ make cmake \
  libssl-dev zlib1g-dev libbz2-dev libreadline-dev \
  libsqlite3-dev llvm libncursesw5-dev xz-utils \
  tk-dev libxml2-dev libxmlsec1-dev libffi-dev liblzma-dev \
  libgdbm-dev libnss3-dev libgdbm-compat-dev uuid-dev \
  jq yq htop tree tmux \
  shellcheck yamllint pre-commit \
  ca-certificates gnupg lsb-release apt-transport-https \
  software-properties-common pkg-config
```

| Package                                          | Purpose                                                                |
| ------------------------------------------------ | ---------------------------------------------------------------------- |
| `git`, `curl`, `wget`                            | Version control and downloads                                          |
| `zip`, `unzip`                                   | Archive handling                                                       |
| `build-essential`, `gcc`, `g++`, `make`, `cmake` | C/C++ toolchain (Go CGo, Python C extensions, Rust deps)               |
| `lib*-dev`, `uuid-dev`                           | Build dependencies for pyenv, Node native modules, Python C extensions |
| `jq`, `yq`                                       | JSON/YAML processing in scripts and pipelines                          |
| `htop`, `tree`                                   | System monitoring and directory visualization                          |
| `tmux`                                           | Terminal multiplexer for persistent sessions                           |
| `shellcheck`                                     | Shell script linting (used by pre-commit hooks)                        |
| `yamllint`                                       | YAML validation (CloudFormation, configs)                              |
| `pre-commit`                                     | Git hook framework                                                     |
| `ca-certificates`, `gnupg`, `lsb-release`        | Repository signing and HTTPS transport                                 |
| `pkg-config`                                     | Library discovery for native builds                                    |

### GitHub CLI (gh)

```bash
(type -p wget >/dev/null || sudo apt install -y wget) \
  && sudo mkdir -p -m 755 /etc/apt/keyrings \
  && out=$(mktemp) && wget -nv -O$out https://cli.github.com/packages/githubcli-archive-keyring.gpg \
  && cat $out | sudo tee /etc/apt/keyrings/githubcli-archive-keyring.gpg > /dev/null \
  && sudo chmod go+r /etc/apt/keyrings/githubcli-archive-keyring.gpg \
  && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list > /dev/null \
  && sudo apt update && sudo apt install -y gh
gh auth login
```

---

## 🛠️ Development Environment Setup

### 1. nvm (Node Version Manager)

```bash
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.1/install.sh | bash
source ~/.bashrc
nvm install --lts
nvm use --lts
```

Verifies: `node --version` (18+), `npm --version`

### 2. Go

```bash
curl -OL https://go.dev/dl/go1.24.1.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.24.1.linux-amd64.tar.gz
rm go1.24.1.linux-amd64.tar.gz
```

Add to `~/.bashrc`:

```bash
export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin
```

Verifies: `go version` (1.24+)

### 3. Rust

```bash
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
source ~/.cargo/env
```

Verifies: `rustc --version`, `cargo --version`

### 4. pyenv (Python Version Manager)

```bash
curl https://pyenv.run | bash
```

> Build dependencies are already covered in the [System Packages](#-system-packages) section.

Add to `~/.bashrc`:

```bash
export PYENV_ROOT="$HOME/.pyenv"
[[ -d $PYENV_ROOT/bin ]] && export PATH="$PYENV_ROOT/bin:$PATH"
eval "$(pyenv init -)"
```

Install Python:

```bash
source ~/.bashrc
pyenv install 3.12
pyenv global 3.12
```

Verifies: `python --version` (3.12+)

---

## 🌙 teamoon Installation

1. **Clone the Repository**

   ```bash
   git clone https://github.com/JuanVilla424/teamoon.git
   ```

2. **Navigate to the Project Directory**

   ```bash
   cd teamoon
   ```

3. **Build**

   ```bash
   make build
   ```

4. **Install** (copies binary to `/usr/local/bin`)

   ```bash
   make install
   ```

---

## 🧠 Claude Code Skills

teamoon's autopilot engine works best with a curated set of agent skills installed. These enhance Claude Code's planning, debugging, TDD, frontend, and browser automation capabilities.

Skills are **automatically installed** as part of the `teamoon init` onboarding wizard, which also sets up config, BMAD commands, global hooks, and MCP servers.

```bash
teamoon init
```

For manual skill installation only: `go run ./cmd/install-skills`

Browse all available skills at [skills.sh](https://skills.sh/).

### Superpowers (obra/superpowers) — All 14 Skills

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

### Anthropic Official (anthropics/skills)

```bash
npx skills add anthropics/skills@frontend-design -g -y
npx skills add anthropics/skills@skill-creator -g -y
```

### Vercel (vercel-labs/agent-skills + agent-browser)

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

teamoon includes security hooks that are installed **globally** via `teamoon init` and **per-project** when scaffolding new projects. Hooks validate Claude Code actions **before execution** — they work even with `--dangerously-skip-permissions`.

### Included Hooks

| Hook             | Trigger    | Protection                                                                                                                                                                                    |
| ---------------- | ---------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `security-check` | Bash       | Force push, reset --hard, --no-verify, blind staging, destructive rm, remote code execution, SQL drops, docker prune, kill -9, sudo escalation, credential reading, AWS/cloud destructive ops |
| `test-guard`     | Bash       | Blocks commit without running tests first                                                                                                                                                     |
| `secrets-guard`  | Write/Edit | Blocks writing .env, credentials, SSH keys, cloud configs, DB passwords, key/cert files, lock files, shell history                                                                            |
| `build-guard`    | Bash       | Blocks push without building first                                                                                                                                                            |
| `commit-format`  | Bash       | Enforces `type(core): lowercase description` format, no emojis, no uppercase                                                                                                                  |

### Where Hooks Live

```
{project}/.claude/
├── settings.json          # Hook configuration
└── hooks/
    ├── security-check.sh  # 40+ blocked patterns
    ├── test-guard.sh      # Test-before-commit gate
    ├── secrets-guard.sh   # 30+ protected file patterns
    ├── build-guard.sh     # Build-before-push gate
    └── commit-format.sh   # Conventional commit enforcement
```

Hooks are committed to the repo — anyone who clones the project gets them automatically.

### Manual Installation (existing projects)

For projects created before hooks were added, run from the teamoon source:

```bash
cd /path/to/teamoon
go run ./cmd/install-hooks /path/to/target-project
```

---

## 📜 License

2025 - This project is licensed under the [GNU General Public License v3.0](https://www.gnu.org/licenses/gpl-3.0.en.html). You are free to use, modify, and distribute this software under the terms of the GPL-3.0 license. For more details, please refer to the [LICENSE](LICENSE) file included in this repository.
