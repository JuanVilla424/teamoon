## 🔨 Installation

### Prerequisites

- **Linux**: Ubuntu/Debian **or** RHEL / Rocky Linux 8+ (with sudo access)
- **macOS**: macOS 12+ with Apple Silicon (M1/M2/M3/M4) or Intel

---

## ⚡ Quick Install (recommended)

Single command that installs all prerequisites and teamoon:

```bash
curl -sSL https://raw.githubusercontent.com/JuanVilla424/teamoon/main/install.sh | bash
```

The installer auto-detects your OS (Ubuntu/Debian, RHEL/Rocky, macOS) and handles everything:

| Step | What it installs                                      |
| ---- | ----------------------------------------------------- |
| 1    | System packages (Homebrew on macOS, apt/dnf on Linux) |
| 2    | Go 1.24                                               |
| 3    | nvm + Node.js LTS                                     |
| 4    | GitHub CLI (gh)                                       |
| 5    | Rust (via rustup)                                     |
| 6    | Python 3.11 (via pyenv)                               |
| 7    | Claude Code CLI                                       |
| 8    | Clones, builds, and installs teamoon as a service     |

Service management: **systemd** on Linux, **launchd** on macOS.

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

## 🐧 RHEL / Rocky Linux 8+ Install

### Prerequisites

- RHEL 8 or 9, Rocky Linux 8 or 9, AlmaLinux 8 or 9
- sudo access (non-root user recommended)
- `dnf` package manager

### Quick Install (recommended)

Same single command — the installer auto-detects the distro:

```bash
curl -sSL https://raw.githubusercontent.com/JuanVilla424/teamoon/main/install.sh | bash
```

### Manual — System Packages

```bash
sudo dnf install -y \
  git curl wget zip unzip \
  gcc gcc-c++ make cmake \
  openssl-devel zlib-devel bzip2-devel readline-devel \
  sqlite-devel llvm ncurses-devel xz-devel \
  tk-devel libxml2-devel libffi-devel \
  gdbm-devel nss-devel libuuid-devel \
  jq htop tree tmux \
  ShellCheck yamllint \
  ca-certificates gnupg2 redhat-lsb-core \
  dnf-plugins-core pkgconf-pkg-config
```

### Manual — GitHub CLI on RHEL

```bash
sudo dnf install -y 'dnf-command(config-manager)'
sudo dnf config-manager --add-repo https://cli.github.com/packages/rpm/gh-cli.repo
sudo dnf install -y gh
gh auth login
```

### SELinux Notes

teamoon writes logs to `/var/log/teamoon.log`. On RHEL with SELinux enforcing, restore the file context after creation:

```bash
sudo restorecon -v /var/log/teamoon.log
sudo restorecon -v /usr/local/bin/teamoon
```

The `make install` target and `install.sh` do this automatically.

### firewalld

If firewalld is running, open the web dashboard port (default 7777):

```bash
sudo firewall-cmd --permanent --add-port=7777/tcp
sudo firewall-cmd --reload
```

The installer does this automatically when firewalld is active.

### Environment File

On RHEL, the systemd service reads `/etc/sysconfig/teamoon` for environment variables (the RHEL convention). The installer creates this file automatically.

---

## 🍎 macOS Install

### Prerequisites

- macOS 12 Monterey or later
- Apple Silicon (M1/M2/M3/M4) or Intel
- Xcode Command Line Tools (`xcode-select --install`)

### Quick Install (recommended)

Same single command — the installer auto-detects macOS:

```bash
curl -sSL https://raw.githubusercontent.com/JuanVilla424/teamoon/main/install.sh | bash
```

The installer uses **Homebrew** for packages and **launchd** instead of systemd.

### Manual — Homebrew Packages

```bash
# Install Homebrew (if not already installed)
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# Install prerequisites
brew install go jq htop tree tmux shellcheck openssl readline
```

### Manual — Build and Install

```bash
git clone https://github.com/JuanVilla424/teamoon.git
cd teamoon
make build
make install
```

`make install` on macOS:

- Copies the binary to `/usr/local/bin/teamoon`
- Generates a launchd user agent (`~/Library/LaunchAgents/com.teamoon.plist`)
- Loads and starts the agent via `launchctl`

### Service Management

```bash
# Check status
launchctl list | grep teamoon

# Stop
launchctl bootout gui/$(id -u)/com.teamoon

# Start
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.teamoon.plist

# View logs
tail -f ~/Library/Logs/teamoon.log
```

### Apple Silicon vs Intel

The installer and Makefile auto-detect your architecture. Homebrew installs to:

- **Apple Silicon**: `/opt/homebrew/`
- **Intel**: `/usr/local/`

Both paths are included in the launchd environment.

### Pre-Built Binaries

Download pre-compiled binaries from [GitHub Releases](https://github.com/JuanVilla424/teamoon/releases):

| Platform            | File                                 |
| ------------------- | ------------------------------------ |
| macOS Apple Silicon | `teamoon-vX.Y.Z-darwin-arm64.tar.gz` |
| macOS Intel         | `teamoon-vX.Y.Z-darwin-amd64.tar.gz` |
| Linux amd64         | `teamoon-vX.Y.Z-linux-amd64.tar.gz`  |
| Linux arm64         | `teamoon-vX.Y.Z-linux-arm64.tar.gz`  |

```bash
tar xzf teamoon-*.tar.gz
sudo mv teamoon /usr/local/bin/
teamoon serve
```

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
