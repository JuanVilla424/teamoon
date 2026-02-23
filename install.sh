#!/usr/bin/env bash
set -eo pipefail

# teamoon installer — single command:
#   curl -sSL https://raw.githubusercontent.com/JuanVilla424/teamoon/main/install.sh | bash
#
# Interactive installer — asks before optional components.
# Installs prerequisites, builds, and sets up teamoon with systemd.
# Supports Ubuntu/Debian and RHEL/Rocky Linux 8+.

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
    printf '\033[1;33m?\033[0m %s [%s] ' "$prompt" "$default" >/dev/tty 2>/dev/null || true
    local answer
    read -r answer </dev/tty 2>/dev/null || answer=""
    echo "${answer:-$default}"
}

# Append line to shell profile (bashrc + bash_profile on RHEL)
append_to_profile() {
    local line="$1"
    if ! grep -qF "$line" "$HOME/.bashrc" 2>/dev/null; then
        printf '\n%s\n' "$line" >> "$HOME/.bashrc"
    fi
    if [ "$DISTRO_FAMILY" = "rhel" ]; then
        if ! grep -qF "$line" "$HOME/.bash_profile" 2>/dev/null; then
            printf '\n%s\n' "$line" >> "$HOME/.bash_profile"
        fi
    fi
}

# ── check root ────────────────────────────────────────────
if [ "$(id -u)" -eq 0 ]; then
    warn "running as root is not recommended — install as a regular user with sudo access"
    if ! ask "Continue as root anyway?" "n"; then
        fail "aborting — re-run as a regular user"
    fi
fi

# ── detect distro family ─────────────────────────────────
if [ -f /etc/debian_version ]; then
    DISTRO_FAMILY="debian"
elif [ -f /etc/redhat-release ] || [ -f /etc/rocky-release ] || [ -f /etc/centos-release ]; then
    DISTRO_FAMILY="rhel"
    if ! has dnf; then
        fail "dnf is required (RHEL 8+). RHEL 7 is not supported."
    fi
else
    fail "unsupported OS. Supported: Ubuntu/Debian, RHEL/Rocky Linux 8+"
fi

if ! has sudo; then
    fail "sudo is required"
fi

printf '\n'
printf '\033[1;35m  ╔═══════════════════════════════════════╗\033[0m\n'
printf '\033[1;35m  ║         teamoon installer             ║\033[0m\n'
printf '\033[1;35m  ╚═══════════════════════════════════════╝\033[0m\n'
printf '\n'
info "detected distro family: $DISTRO_FAMILY"

# ── 1. system packages (required) ────────────────────────
step "system packages (required)"

if [ "$DISTRO_FAMILY" = "debian" ]; then
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
else
    PKGS=(
        git curl wget zip unzip
        gcc gcc-c++ make cmake
        openssl-devel zlib-devel bzip2-devel readline-devel
        sqlite-devel llvm ncurses-devel xz-devel
        tk-devel libxml2-devel libffi-devel
        gdbm-devel nss-devel libuuid-devel
        jq htop tree tmux
        ShellCheck yamllint
        ca-certificates gnupg2 redhat-lsb-core
        dnf-plugins-core pkgconf-pkg-config
    )

    MISSING=()
    for pkg in "${PKGS[@]}"; do
        if ! rpm -q "$pkg" &>/dev/null; then
            MISSING+=("$pkg")
        fi
    done

    if [ ${#MISSING[@]} -eq 0 ]; then
        ok "all system packages already installed"
    else
        info "installing ${#MISSING[@]} packages..."
        sudo dnf install -y -q "${MISSING[@]}"
        ok "system packages installed"
    fi
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
    append_to_profile 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin'
}

if has go; then
    CURRENT_GO=$(go version | grep -o '[0-9]\+\.[0-9]\+' | head -1)
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
        if [ "$DISTRO_FAMILY" = "debian" ]; then
            info "installing gh (Debian/Ubuntu)"
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
        else
            info "installing gh (RHEL/Rocky)"
            sudo dnf install -y 'dnf-command(config-manager)' 2>/dev/null || true
            sudo dnf config-manager --add-repo https://cli.github.com/packages/rpm/gh-cli.repo
            sudo dnf install -y gh
        fi
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
    PY_VER=$(python3 --version 2>&1 | grep -o '[0-9]\+\.[0-9]\+' | head -1)
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

        append_to_profile 'export PYENV_ROOT="$HOME/.pyenv"'
        append_to_profile '[[ -d $PYENV_ROOT/bin ]] && export PATH="$PYENV_ROOT/bin:$PATH"'
        append_to_profile 'eval "$(pyenv init -)"'
        ok "pyenv installed"

        info "installing python 3.11 (this may take a few minutes)"
        pyenv install 3.11
        pyenv global 3.11
        ok "python 3.11 installed"
    else
        warn "skipped"
    fi
fi

# RHEL: install pre-commit via pip if not available
if [ "$DISTRO_FAMILY" = "rhel" ] && ! has pre-commit; then
    if has pip3; then
        info "installing pre-commit via pip3 (not in RHEL repos)"
        pip3 install --user pre-commit 2>/dev/null || true
        ok "pre-commit installed"
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
WEB_HOST=$(ask_input "Bind address (localhost = local only, 0.0.0.0 = all interfaces)" "localhost")
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

if [ "$DISTRO_FAMILY" = "rhel" ]; then
    ENV_FILE_PATH="/etc/sysconfig/teamoon"
else
    ENV_FILE_PATH="${HOME}/.config/teamoon/.env"
fi

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
EnvironmentFile=-${ENV_FILE_PATH}

[Install]
WantedBy=multi-user.target
SVCEOF

ok "service file generated"

# ── 11. install binary + systemd ────────────────────────
info "installing binary + systemd service (requires sudo)"
make install
ok "installed to $BINARY_DEST"

# ── 12. RHEL post-install: SELinux + firewalld ──────────
if [ "$DISTRO_FAMILY" = "rhel" ]; then
    step "RHEL: SELinux + firewall configuration"

    if has restorecon; then
        sudo restorecon -v /var/log/teamoon.log 2>/dev/null || true
        sudo restorecon -v /usr/local/bin/teamoon 2>/dev/null || true
        ok "SELinux context restored"
    else
        warn "restorecon not found — skipping SELinux context restore"
    fi

    if systemctl is-active --quiet firewalld 2>/dev/null; then
        sudo firewall-cmd --permanent --add-port="${WEB_PORT}/tcp" 2>/dev/null && \
            sudo firewall-cmd --reload 2>/dev/null || true
        ok "firewalld: port ${WEB_PORT}/tcp opened"
    else
        warn "firewalld not active — skipping port configuration"
    fi

    if [ ! -f /etc/sysconfig/teamoon ]; then
        printf '# teamoon environment variables\n' | sudo tee /etc/sysconfig/teamoon >/dev/null
        sudo chmod 640 /etc/sysconfig/teamoon
        ok "/etc/sysconfig/teamoon created"
    fi
fi

# ── 13. write initial config ────────────────────────────
CONFIG_DIR="${HOME}/.config/teamoon"
CONFIG_FILE="${CONFIG_DIR}/config.json"

if [ ! -f "$CONFIG_FILE" ]; then
    info "writing initial config"
    mkdir -p "$CONFIG_DIR"
    cat > "$CONFIG_FILE" <<CFGEOF
{
  "projects_dir": "${PROJECTS_DIR_EXPANDED}",
  "web_port": ${WEB_PORT},
  "web_host": "${WEB_HOST}",
  "web_password": "",
  "max_concurrent": 3,
  "refresh_interval_sec": 30
}
CFGEOF
    ok "config written to $CONFIG_FILE"
else
    ok "config already exists at $CONFIG_FILE"
    # Ensure source_dir points to install location
    if command -v jq &>/dev/null; then
        CURRENT_SRC=$(jq -r '.source_dir // ""' "$CONFIG_FILE")
        if [ -n "$CURRENT_SRC" ] && [ ! -d "$CURRENT_SRC/.git" ]; then
            info "updating source_dir to $INSTALL_DIR"
            jq --arg sd "$INSTALL_DIR" '.source_dir = $sd' "$CONFIG_FILE" > "${CONFIG_FILE}.tmp" && mv "${CONFIG_FILE}.tmp" "$CONFIG_FILE"
            ok "source_dir updated"
        fi
    fi
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
echo "  web ui:   http://${WEB_HOST}:${WEB_PORT}"
echo ""
echo "  Next steps:"
echo "    1. Open http://${WEB_HOST}:${WEB_PORT} for guided onboarding (skills, hooks, MCP)"
echo "    2. Or run 'teamoon init' for CLI setup wizard"
echo ""
