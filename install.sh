#!/usr/bin/env bash
set -eo pipefail

# teamoon installer:
#   curl -sSL https://raw.githubusercontent.com/JuanVilla424/teamoon/<branch>/install.sh | bash
#
# Interactive installer — asks before optional components.
# Installs prerequisites, builds, and sets up teamoon as a service.
# Supports Ubuntu/Debian, RHEL/Rocky Linux 8+, and macOS (Apple Silicon + Intel).

REPO="https://github.com/JuanVilla424/teamoon.git"
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

# Silent input (for passwords — does not echo)
ask_input_silent() {
    local prompt="$1" default="$2"
    printf '\033[1;33m?\033[0m %s ' "$prompt" >/dev/tty 2>/dev/null || true
    local answer
    read -rs answer </dev/tty 2>/dev/null || answer=""
    printf '\n'
    echo "${answer:-$default}"
}

# Append line to shell profile (bashrc/zshrc depending on OS)
append_to_profile() {
    local line="$1"
    if [ "$DISTRO_FAMILY" = "darwin" ]; then
        local target="$HOME/.zshrc"
        if ! grep -qF "$line" "$target" 2>/dev/null; then
            printf '\n%s\n' "$line" >> "$target"
        fi
    else
        if ! grep -qF "$line" "$HOME/.bashrc" 2>/dev/null; then
            printf '\n%s\n' "$line" >> "$HOME/.bashrc"
        fi
        if [ "$DISTRO_FAMILY" = "rhel" ]; then
            if ! grep -qF "$line" "$HOME/.bash_profile" 2>/dev/null; then
                printf '\n%s\n' "$line" >> "$HOME/.bash_profile"
            fi
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

# ── detect OS / distro family ────────────────────────────
OS_TYPE=$(uname -s)
if [ "$OS_TYPE" = "Darwin" ]; then
    DISTRO_FAMILY="darwin"
elif [ -f /etc/debian_version ]; then
    DISTRO_FAMILY="debian"
elif [ -f /etc/redhat-release ] || [ -f /etc/rocky-release ] || [ -f /etc/centos-release ]; then
    DISTRO_FAMILY="rhel"
    if ! has dnf; then
        fail "dnf is required (RHEL 8+). RHEL 7 is not supported."
    fi
else
    fail "unsupported OS. Supported: Ubuntu/Debian, RHEL/Rocky Linux 8+, macOS"
fi

if [ "$DISTRO_FAMILY" != "darwin" ] && ! has sudo; then
    fail "sudo is required"
fi

printf '\n'
printf '\033[1;35m  ╔═══════════════════════════════════════╗\033[0m\n'
printf '\033[1;35m  ║         teamoon installer             ║\033[0m\n'
printf '\033[1;35m  ╚═══════════════════════════════════════╝\033[0m\n'
printf '\n'
info "detected OS: $DISTRO_FAMILY"

# ── 1. system packages (required) ────────────────────────
step "system packages (required)"

if [ "$DISTRO_FAMILY" = "darwin" ]; then
    # Ensure Homebrew is installed
    if ! has brew; then
        info "installing Homebrew"
        /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)" </dev/tty
        # Add brew to PATH for this session
        if [ -f /opt/homebrew/bin/brew ]; then
            eval "$(/opt/homebrew/bin/brew shellenv)"
        elif [ -f /usr/local/bin/brew ]; then
            eval "$(/usr/local/bin/brew shellenv)"
        fi
        ok "homebrew installed"
    else
        ok "homebrew already installed"
    fi

    PKGS=(jq htop tree tmux shellcheck openssl readline)
    MISSING=()
    for pkg in "${PKGS[@]}"; do
        if ! brew list --formula "$pkg" &>/dev/null; then
            MISSING+=("$pkg")
        fi
    done

    if [ ${#MISSING[@]} -eq 0 ]; then
        ok "all system packages already installed"
    else
        info "installing ${#MISSING[@]} packages via Homebrew..."
        brew install "${MISSING[@]}"
        ok "system packages installed"
    fi

    # Xcode Command Line Tools (provides git, make, clang)
    if ! xcode-select -p &>/dev/null; then
        info "installing Xcode Command Line Tools"
        xcode-select --install 2>/dev/null || true
        warn "accept the Xcode CLT dialog, then re-run this installer"
    else
        ok "xcode command line tools installed"
    fi

elif [ "$DISTRO_FAMILY" = "debian" ]; then
    PKGS=(
        git curl wget zip unzip
        build-essential gcc g++ make cmake
        libssl-dev zlib1g-dev libbz2-dev libreadline-dev
        libsqlite3-dev llvm libncursesw5-dev xz-utils
        tk-dev libxml2-dev libxmlsec1-dev libffi-dev liblzma-dev
        libgdbm-dev libnss3-dev libgdbm-compat-dev uuid-dev
        jq htop tree tmux
        shellcheck
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
    # RHEL
    info "enabling EPEL + CRB repositories"
    sudo dnf install -y -q epel-release 2>/dev/null || \
        sudo dnf install -y -q "https://dl.fedoraproject.org/pub/epel/epel-release-latest-$(rpm -E %rhel).noarch.rpm" 2>/dev/null || true
    sudo dnf config-manager --set-enabled crb 2>/dev/null || \
        sudo dnf config-manager --set-enabled powertools 2>/dev/null || true

    RHEL_VER=$(rpm -E %rhel 2>/dev/null || echo 8)
    if [ "$RHEL_VER" -ge 9 ] 2>/dev/null; then
        _CURL_PKG="curl-minimal"
    else
        _CURL_PKG="curl"
    fi

    PKGS=(
        git "$_CURL_PKG" wget zip unzip
        gcc gcc-c++ make cmake
        openssl-devel zlib-devel bzip2-devel readline-devel
        sqlite-devel llvm ncurses-devel xz-devel
        tk-devel libxml2-devel libffi-devel
        gdbm-devel nss-devel libuuid-devel
        jq htop tree tmux
        ShellCheck
        ca-certificates gnupg2
        dnf-plugins-core pkgconf-pkg-config
        python3-pip
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
        sudo dnf install -y "${MISSING[@]}"
        ok "system packages installed"
    fi
fi

# yq
if ! has yq; then
    info "installing yq"
    YQ_OS="linux"
    YQ_ARCH="amd64"
    if [ "$DISTRO_FAMILY" = "darwin" ]; then
        YQ_OS="darwin"
        if [ "$(uname -m)" = "arm64" ]; then
            YQ_ARCH="arm64"
        fi
    elif [ "$(uname -m)" = "aarch64" ]; then
        YQ_ARCH="arm64"
    fi
    YQ_URL="https://github.com/mikefarah/yq/releases/latest/download/yq_${YQ_OS}_${YQ_ARCH}"
    if [ "$DISTRO_FAMILY" = "darwin" ]; then
        curl -sL "$YQ_URL" -o /usr/local/bin/yq
        chmod +x /usr/local/bin/yq
    else
        sudo wget -qO /usr/local/bin/yq "$YQ_URL"
        sudo chmod +x /usr/local/bin/yq
    fi
    ok "yq installed"
else
    ok "yq already installed"
fi

# ── 2. Go (required) ────────────────────────────────────
step "Go ${GO_VERSION} (required)"

install_go() {
    if [ "$DISTRO_FAMILY" = "darwin" ]; then
        # Use Homebrew on macOS
        info "installing Go via Homebrew"
        brew install go
        if ! echo "$PATH" | grep -q "go/bin"; then
            local gopath
            gopath="$(go env GOPATH)"
            export PATH="$PATH:${gopath}/bin"
        fi
        append_to_profile 'export PATH=$PATH:$(go env GOPATH)/bin'
    else
        local go_arch="amd64"
        if [ "$(uname -m)" = "aarch64" ]; then
            go_arch="arm64"
        fi
        local tarball="go${GO_VERSION}.linux-${go_arch}.tar.gz"
        info "downloading Go ${GO_VERSION}"
        curl -sLO "https://go.dev/dl/${tarball}"
        sudo rm -rf /usr/local/go
        sudo tar -C /usr/local -xzf "${tarball}"
        rm -f "${tarball}"

        if ! echo "$PATH" | grep -q "/usr/local/go/bin"; then
            export PATH="$PATH:/usr/local/go/bin:$HOME/go/bin"
        fi
        append_to_profile 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin'
    fi
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
        if [ "$DISTRO_FAMILY" = "darwin" ]; then
            info "installing gh via Homebrew"
            brew install gh
        elif [ "$DISTRO_FAMILY" = "debian" ]; then
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

BRANCH=$(ask_input "Branch to install [main, prod, test, dev]" "main")
WEB_PORT=$(ask_input "Web dashboard port" "7777")
WEB_HOST=$(ask_input "Bind address (localhost = local only, 0.0.0.0 = all interfaces)" "localhost")
PROJECTS_DIR=$(ask_input "Projects directory" "~/Projects")
WEB_PASS=$(ask_input_silent "Web dashboard password (enter = no auth)")

# Expand ~ for mkdir
PROJECTS_DIR_EXPANDED="${PROJECTS_DIR/#\~/$HOME}"
mkdir -p "$PROJECTS_DIR_EXPANDED"

# ── 9. clone + build + install teamoon ──────────────────
step "teamoon"

if [ -d "$INSTALL_DIR/.git" ]; then
    info "updating existing clone at $INSTALL_DIR"
    git -C "$INSTALL_DIR" remote set-branches origin '*'
    git -C "$INSTALL_DIR" fetch origin --quiet
    git -C "$INSTALL_DIR" checkout "$BRANCH" --quiet 2>/dev/null || git -C "$INSTALL_DIR" checkout -b "$BRANCH" "origin/$BRANCH" --quiet
    git -C "$INSTALL_DIR" reset --hard "origin/$BRANCH" --quiet
    ok "updated to latest ($BRANCH)"
else
    info "cloning teamoon to $INSTALL_DIR"
    mkdir -p "$(dirname "$INSTALL_DIR")"
    git clone --branch "$BRANCH" --depth 1 "$REPO" "$INSTALL_DIR" --quiet
    ok "cloned ($BRANCH)"
fi

info "building"
cd "$INSTALL_DIR"
make build
ok "built"

# ── 10. install binary + service ─────────────────────────
if [ "$DISTRO_FAMILY" = "darwin" ]; then
    # macOS: copy binary + launchd user agent
    info "installing binary to $BINARY_DEST"
    sudo cp "$INSTALL_DIR/teamoon" "$BINARY_DEST"
    sudo chmod 755 "$BINARY_DEST"
    ok "binary installed"

    PLIST_LABEL="com.teamoon"
    PLIST_DIR="${HOME}/Library/LaunchAgents"
    PLIST_FILE="${PLIST_DIR}/${PLIST_LABEL}.plist"
    LOG_FILE="${HOME}/Library/Logs/teamoon.log"

    mkdir -p "$PLIST_DIR"

    # Unload existing agent if running
    launchctl bootout "gui/$(id -u)/${PLIST_LABEL}" 2>/dev/null || true

    info "generating launchd user agent"
    cat > "$PLIST_FILE" <<PLISTEOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>${PLIST_LABEL}</string>
  <key>ProgramArguments</key>
  <array>
    <string>${BINARY_DEST}</string>
    <string>serve</string>
  </array>
  <key>KeepAlive</key>
  <true/>
  <key>RunAtLoad</key>
  <true/>
  <key>StandardOutPath</key>
  <string>${LOG_FILE}</string>
  <key>StandardErrorPath</key>
  <string>${LOG_FILE}</string>
  <key>EnvironmentVariables</key>
  <dict>
    <key>HOME</key>
    <string>${HOME}</string>
    <key>PATH</key>
    <string>/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin:${HOME}/.nvm/versions/node/$(node --version 2>/dev/null || echo v22.0.0)/bin:${HOME}/go/bin</string>
  </dict>
  <key>WorkingDirectory</key>
  <string>${HOME}</string>
</dict>
</plist>
PLISTEOF

    launchctl bootstrap "gui/$(id -u)" "$PLIST_FILE" 2>/dev/null || \
        launchctl load "$PLIST_FILE" 2>/dev/null || true
    ok "launchd agent installed and started"
else
    # Linux: systemd service
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

    info "installing binary + systemd service (requires sudo)"
    make install
    ok "installed to $BINARY_DEST"
fi

# ── 11. RHEL post-install: SELinux + firewalld ──────────
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

# ── 13. set password (if provided) ──────────────────────
if [ -n "$WEB_PASS" ]; then
    info "setting web dashboard password (bcrypt hash)"
    "$BINARY_DEST" set-password "$WEB_PASS"
    ok "password set"
fi

# ── 14. generate service environment file ────────────────
if [ "$DISTRO_FAMILY" != "darwin" ]; then
    ENV_VARS=""
    while IFS= read -r var; do
        [ -n "$var" ] && ENV_VARS="${ENV_VARS}${var}"$'\n'
    done < <(env | grep "^CLAUDE_CODE_" | sort)

    if [ -n "$ENV_VARS" ]; then
        if [ -f "$ENV_FILE_PATH" ]; then
            cp "$ENV_FILE_PATH" "${ENV_FILE_PATH}.bak"
            ok "backed up existing .env to ${ENV_FILE_PATH}.bak"
        fi
        printf '%s' "$ENV_VARS" > "$ENV_FILE_PATH"
        chmod 600 "$ENV_FILE_PATH"
        ok "service env file written to $ENV_FILE_PATH"
    else
        warn "no CLAUDE_CODE_* env vars found — .env not created (set them in ~/.bashrc and re-run)"
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
if [ "$DISTRO_FAMILY" = "darwin" ]; then
    echo "  service:  launchctl list | grep teamoon"
    echo "  logs:     ${HOME}/Library/Logs/teamoon.log"
else
    echo "  service:  systemctl status teamoon"
fi
echo "  web ui:   http://${WEB_HOST}:${WEB_PORT}"
echo ""
echo "  Next steps:"
echo "    1. Open http://${WEB_HOST}:${WEB_PORT} for guided onboarding (skills, hooks, MCP)"
echo "    2. Or run 'teamoon init' for CLI setup wizard"
echo ""
