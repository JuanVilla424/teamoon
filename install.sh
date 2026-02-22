#!/usr/bin/env bash
set -eo pipefail

# teamoon installer — single command:
#   curl -sSL https://raw.githubusercontent.com/JuanVilla424/teamoon/main/install.sh | bash
#
# Interactive installer — asks before optional components.
# Installs prerequisites, builds, and sets up teamoon with systemd.

REPO="https://github.com/JuanVilla424/teamoon.git"
BRANCH="main"
INSTALL_DIR="${HOME}/.local/src/teamoon"
BINARY_DEST="/usr/local/bin/teamoon"

GO_VERSION="1.24.1"
NVM_VERSION="0.40.1"
CURRENT_USER=$(whoami)

# ── helpers ──────────────────────────────────────────────
info()  { printf '\033[1;34m→\033[0m %s\n' "$*"; }
ok()    { printf '\033[1;32m✓\033[0m %s\n' "$*"; }
warn()  { printf '\033[1;33m~\033[0m %s\n' "$*"; }
fail()  { printf '\033[1;31m✗\033[0m %s\n' "$*"; exit 1; }
step()  { printf '\n\033[1;36m── %s ──\033[0m\n' "$*"; }

has() { command -v "$1" &>/dev/null; }

# Read from terminal even when piped from curl
ask() {
    local prompt="$1" default="${2:-y}"
    if [ "$default" = "y" ]; then
        prompt="$prompt [Y/n] "
    else
        prompt="$prompt [y/N] "
    fi
    printf '\033[1;33m?\033[0m %s' "$prompt"
    local answer
    read -r answer </dev/tty 2>/dev/null || answer=""
    answer="${answer:-$default}"
    case "$answer" in
        [yY]*) return 0 ;;
        *) return 1 ;;
    esac
}

ask_input() {
    local prompt="$1" default="$2"
    printf '\033[1;33m?\033[0m %s [%s] ' "$prompt" "$default"
    local answer
    read -r answer </dev/tty 2>/dev/null || answer=""
    echo "${answer:-$default}"
}

# ── check OS ─────────────────────────────────────────────
if [ ! -f /etc/debian_version ]; then
    fail "this installer requires Ubuntu/Debian (apt). For other distros, see INSTALL.md"
fi

if ! has sudo; then
    fail "sudo is required"
fi

printf '\n'
printf '\033[1;35m  ╔═══════════════════════════════════════╗\033[0m\n'
printf '\033[1;35m  ║         teamoon installer             ║\033[0m\n'
printf '\033[1;35m  ╚═══════════════════════════════════════╝\033[0m\n'
printf '\n'

# ── 1. system packages (required) ────────────────────────
step "system packages (required)"

PKGS=(
    git curl wget zip unzip
    build-essential gcc g++ make cmake
    libssl-dev zlib1g-dev libbz2-dev libreadline-dev
    libsqlite3-dev llvm libncursesw5-dev xz-utils
    tk-dev libxml2-dev libxmlsec1-dev libffi-dev liblzma-dev
    libgdbm-dev libnss3-dev libgdbm-compat-dev uuid-dev
    jq htop tree tmux
    shellcheck yamllint pre-commit
    ca-certificates gnupg lsb-release apt-transport-https
    software-properties-common pkg-config
)

MISSING=()
for pkg in "${PKGS[@]}"; do
    if ! dpkg -s "$pkg" &>/dev/null; then
        MISSING+=("$pkg")
    fi
done

if [ ${#MISSING[@]} -eq 0 ]; then
    ok "all system packages already installed"
else
    info "installing ${#MISSING[@]} packages..."
    sudo apt-get update -qq
    sudo apt-get install -y -qq "${MISSING[@]}"
    ok "system packages installed"
fi

if ! has yq; then
    info "installing yq"
    sudo wget -qO /usr/local/bin/yq https://github.com/mikefarah/yq/releases/latest/download/yq_linux_amd64
    sudo chmod +x /usr/local/bin/yq
    ok "yq installed"
else
    ok "yq already installed"
fi

# ── 2. Go (required) ────────────────────────────────────
step "Go ${GO_VERSION} (required)"

install_go() {
    local tarball="go${GO_VERSION}.linux-amd64.tar.gz"
    info "downloading Go ${GO_VERSION}"
    curl -sLO "https://go.dev/dl/${tarball}"
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf "${tarball}"
    rm -f "${tarball}"

    if ! echo "$PATH" | grep -q "/usr/local/go/bin"; then
        export PATH="$PATH:/usr/local/go/bin:$HOME/go/bin"
    fi
    if ! grep -q '/usr/local/go/bin' "$HOME/.bashrc" 2>/dev/null; then
        echo 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin' >> "$HOME/.bashrc"
    fi
}

if has go; then
    CURRENT_GO=$(go version | grep -oP '\d+\.\d+' | head -1)
    GO_MINOR=$(echo "$CURRENT_GO" | cut -d. -f2)
    if [ "$GO_MINOR" -ge 24 ]; then
        ok "go already installed ($CURRENT_GO)"
    else
        warn "go $CURRENT_GO found, upgrading to $GO_VERSION"
        install_go
        ok "go ${GO_VERSION} installed"
    fi
else
    install_go
    ok "go ${GO_VERSION} installed"
fi

# ── 3. nvm + Node.js (required) ─────────────────────────
step "nvm + Node.js LTS (required)"

export NVM_DIR="${HOME}/.nvm"

# nvm/pyenv/rustup use unbound variables — no set -u in this script
if [ -s "$NVM_DIR/nvm.sh" ]; then
    # shellcheck disable=SC1091
    . "$NVM_DIR/nvm.sh"
    ok "nvm already installed"
else
    info "installing nvm ${NVM_VERSION}"
    curl -so- "https://raw.githubusercontent.com/nvm-sh/nvm/v${NVM_VERSION}/install.sh" | bash
    # shellcheck disable=SC1091
    . "$NVM_DIR/nvm.sh"
    ok "nvm installed"
fi

if has node; then
    NODE_VER=$(node --version)
    NODE_MAJOR=${NODE_VER#v}
    NODE_MAJOR=${NODE_MAJOR%%.*}
    if [ "$NODE_MAJOR" -ge 18 ]; then
        ok "node already installed ($NODE_VER)"
    else
        info "node $NODE_VER too old, installing LTS"
        nvm install --lts
        nvm alias default node
        ok "node $(node --version) installed"
    fi
else
    info "installing Node.js LTS"
    nvm install --lts
    nvm alias default node
    ok "node $(node --version) installed"
fi

# ── 4. GitHub CLI (optional) ────────────────────────────
step "GitHub CLI (optional)"

if has gh; then
    ok "gh already installed ($(gh --version | head -1))"
else
    if ask "Install GitHub CLI (gh)? Needed for PR features"; then
        info "installing gh"
        sudo mkdir -p -m 755 /etc/apt/keyrings
        tmpkey=$(mktemp)
        wget -nv -O "$tmpkey" https://cli.github.com/packages/githubcli-archive-keyring.gpg
        sudo cp "$tmpkey" /etc/apt/keyrings/githubcli-archive-keyring.gpg
        sudo chmod go+r /etc/apt/keyrings/githubcli-archive-keyring.gpg
        rm -f "$tmpkey"
        echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
            | sudo tee /etc/apt/sources.list.d/github-cli.list >/dev/null
        sudo apt-get update -qq
        sudo apt-get install -y -qq gh
        ok "gh installed"
        warn "run 'gh auth login' after install to authenticate"
    else
        warn "skipped"
    fi
fi

# ── 5. Rust (optional) ──────────────────────────────────
step "Rust (optional)"

if has rustc; then
    ok "rust already installed ($(rustc --version | awk '{print $2}'))"
else
    if ask "Install Rust? Useful for some dev tools" "n"; then
        info "installing rust via rustup"
        curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
        # shellcheck disable=SC1091
        source "$HOME/.cargo/env" 2>/dev/null || true
        ok "rust installed"
    else
        warn "skipped"
    fi
fi

# ── 6. pyenv + Python (optional) ────────────────────────
step "pyenv + Python 3.11 (optional)"

export PYENV_ROOT="${HOME}/.pyenv"

if has python3; then
    PY_VER=$(python3 --version 2>&1 | grep -oP '\d+\.\d+' | head -1)
    ok "python already installed ($PY_VER)"
elif [ -d "$PYENV_ROOT" ]; then
    export PATH="$PYENV_ROOT/bin:$PATH"
    eval "$(pyenv init -)" 2>/dev/null || true
    ok "pyenv already installed"
else
    if ask "Install Python 3.11 via pyenv? (takes a few minutes)" "n"; then
        info "installing pyenv"
        curl -s https://pyenv.run | bash
        export PATH="$PYENV_ROOT/bin:$PATH"
        eval "$(pyenv init -)" 2>/dev/null || true

        if ! grep -q 'PYENV_ROOT' "$HOME/.bashrc" 2>/dev/null; then
            {
                echo 'export PYENV_ROOT="$HOME/.pyenv"'
                echo '[[ -d $PYENV_ROOT/bin ]] && export PATH="$PYENV_ROOT/bin:$PATH"'
                echo 'eval "$(pyenv init -)"'
            } >> "$HOME/.bashrc"
        fi
        ok "pyenv installed"

        info "installing python 3.11 (this may take a few minutes)"
        pyenv install 3.11
        pyenv global 3.11
        ok "python 3.11 installed"
    else
        warn "skipped"
    fi
fi

# ── 7. Claude Code (optional) ───────────────────────────
step "Claude Code CLI (optional)"

if has claude; then
    ok "claude already installed ($(claude --version 2>/dev/null || echo 'unknown'))"
else
    if ask "Install Claude Code CLI? Required for autopilot"; then
        info "installing claude code"
        npm install -g @anthropic-ai/claude-code
        ok "claude installed"
        warn "run 'claude' to authenticate with your Anthropic account"
    else
        warn "skipped"
    fi
fi

# ── 8. Configuration ────────────────────────────────────
step "Configuration"

WEB_PORT=$(ask_input "Web dashboard port" "7777")
PROJECTS_DIR=$(ask_input "Projects directory" "~/Projects")

# Expand ~ for mkdir
PROJECTS_DIR_EXPANDED="${PROJECTS_DIR/#\~/$HOME}"
mkdir -p "$PROJECTS_DIR_EXPANDED"

# ── 9. clone + build + install teamoon ──────────────────
step "teamoon"

if [ -d "$INSTALL_DIR/.git" ]; then
    info "updating existing clone at $INSTALL_DIR"
    git -C "$INSTALL_DIR" fetch origin "$BRANCH" --quiet
    git -C "$INSTALL_DIR" checkout "$BRANCH" --quiet 2>/dev/null || true
    git -C "$INSTALL_DIR" reset --hard "origin/$BRANCH" --quiet
    ok "updated to latest"
else
    info "cloning teamoon to $INSTALL_DIR"
    mkdir -p "$(dirname "$INSTALL_DIR")"
    git clone --branch "$BRANCH" --depth 1 "$REPO" "$INSTALL_DIR" --quiet
    ok "cloned"
fi

info "building"
cd "$INSTALL_DIR"
make build
ok "built"

# ── 10. generate systemd service ────────────────────────
info "generating systemd service for user $CURRENT_USER"

cat > "$INSTALL_DIR/teamoon.service" <<SVCEOF
[Unit]
Description=Teamoon - AI-powered project management and autopilot task engine
After=network.target

[Service]
Type=simple
User=${CURRENT_USER}
Group=${CURRENT_USER}
ExecStart=/usr/local/bin/teamoon serve
Restart=always
RestartSec=5
WorkingDirectory=${HOME}
Environment=HOME=${HOME}

[Install]
WantedBy=multi-user.target
SVCEOF

ok "service file generated"

# ── 11. install binary + systemd ────────────────────────
info "installing binary + systemd service (requires sudo)"
make install
ok "installed to $BINARY_DEST"

# ── 12. write initial config ────────────────────────────
CONFIG_DIR="${HOME}/.config/teamoon"
CONFIG_FILE="${CONFIG_DIR}/config.json"

if [ ! -f "$CONFIG_FILE" ]; then
    info "writing initial config"
    mkdir -p "$CONFIG_DIR"
    cat > "$CONFIG_FILE" <<CFGEOF
{
  "projects_dir": "${PROJECTS_DIR_EXPANDED}",
  "web_port": ${WEB_PORT},
  "web_password": "",
  "max_concurrent": 3,
  "refresh_interval_sec": 30
}
CFGEOF
    ok "config written to $CONFIG_FILE"
else
    ok "config already exists at $CONFIG_FILE"
fi

# ── done ─────────────────────────────────────────────────
printf '\n'
printf '\033[1;32m  ╔═══════════════════════════════════════╗\033[0m\n'
printf '\033[1;32m  ║   teamoon installed successfully!     ║\033[0m\n'
printf '\033[1;32m  ╚═══════════════════════════════════════╝\033[0m\n'
printf '\n'
echo "  binary:   $BINARY_DEST"
echo "  source:   $INSTALL_DIR"
echo "  config:   $CONFIG_FILE"
echo "  service:  systemctl status teamoon"
echo "  web ui:   http://localhost:${WEB_PORT}"
echo ""
echo "  Next steps:"
echo "    1. Open http://localhost:${WEB_PORT} for guided onboarding (skills, hooks, MCP)"
echo "    2. Or run 'teamoon init' for CLI setup wizard"
echo ""
