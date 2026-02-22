## 🔨 Installation

### Prerequisites

- Ubuntu/Debian-based system
- sudo access

---

## ⚡ Quick Install (recommended)

Single command that installs all prerequisites and teamoon:

```bash
curl -sSL https://raw.githubusercontent.com/JuanVilla424/teamoon/main/install.sh | bash
```

This handles everything automatically:

| Step | What it installs                                                    |
| ---- | ------------------------------------------------------------------- |
| 1    | System packages (build-essential, libssl-dev, jq, shellcheck, etc.) |
| 2    | GitHub CLI (gh)                                                     |
| 3    | Go 1.24                                                             |
| 4    | nvm + Node.js LTS                                                   |
| 5    | Python 3.11 (via pyenv)                                             |
| 6    | Rust (via rustup)                                                   |
| 7    | Claude Code CLI                                                     |
| 8    | Clones, builds, and installs teamoon with systemd service           |

Each step checks if already installed and skips if so — safe to re-run for updates.

---

## 🛠️ Manual Install

If you prefer to install components individually.

### 1. System Packages

```bash
sudo apt update && sudo apt install -y \
  git curl wget zip unzip \
  build-essential gcc g++ make cmake \
  libssl-dev zlib1g-dev libbz2-dev libreadline-dev \
  libsqlite3-dev llvm libncursesw5-dev xz-utils \
  tk-dev libxml2-dev libxmlsec1-dev libffi-dev liblzma-dev \
  libgdbm-dev libnss3-dev libgdbm-compat-dev uuid-dev \
  jq htop tree tmux \
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
| `jq`                                             | JSON processing in scripts and pipelines                               |
| `htop`, `tree`                                   | System monitoring and directory visualization                          |
| `tmux`                                           | Terminal multiplexer for persistent sessions                           |
| `shellcheck`                                     | Shell script linting (used by pre-commit hooks)                        |
| `yamllint`                                       | YAML validation (CloudFormation, configs)                              |
| `pre-commit`                                     | Git hook framework                                                     |
| `ca-certificates`, `gnupg`, `lsb-release`        | Repository signing and HTTPS transport                                 |
| `pkg-config`                                     | Library discovery for native builds                                    |

### 2. Go 1.24+

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

### 3. nvm + Node.js LTS

```bash
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.1/install.sh | bash
source ~/.bashrc
nvm install --lts
nvm use --lts
```

Verifies: `node --version` (18+), `npm --version`

### 4. GitHub CLI (optional)

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

### 5. Build and Install teamoon

```bash
git clone https://github.com/JuanVilla424/teamoon.git
cd teamoon
make build
make install
```

`make install` compiles the binary, copies it to `/usr/local/bin/teamoon`, sets up the systemd service, and starts it.

---

## 🌐 Web Onboarding

After installing teamoon, the web UI provides a guided setup wizard:

```bash
teamoon serve
# Open http://localhost:7777
```

On first visit, the web UI shows the **Setup** view — an Ubuntu installer-style wizard that:

1. **Prerequisites** — checks all required tools with version validation, installs missing ones directly from the browser
2. **Configuration** — projects directory, web port, password, concurrency
3. **Skills** — 21 Claude Code agent skills (superpowers + anthropic + vercel)
4. **BMAD** — project management framework commands
5. **Hooks** — global security hooks for Claude Code
6. **MCP Servers** — Context7, Memory, Sequential Thinking

Each step streams real-time progress. Already-installed components are automatically skipped.

Alternatively, the CLI wizard provides the same setup:

```bash
teamoon init
```

---

## 🔄 Updating

Re-run the installer:

```bash
curl -sSL https://raw.githubusercontent.com/JuanVilla424/teamoon/main/install.sh | bash
```

Or manually:

```bash
cd ~/.local/src/teamoon  # or wherever you cloned it
git pull
make install
```

---

## 📜 License

2026 - This project is licensed under the [GNU General Public License v3.0](https://www.gnu.org/licenses/gpl-3.0.en.html). You are free to use, modify, and distribute this software under the terms of the GPL-3.0 license. For more details, please refer to the [LICENSE](LICENSE) file included in this repository.
