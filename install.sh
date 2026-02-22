#!/usr/bin/env bash
set -euo pipefail

# teamoon installer — single command:
#   curl -sSL https://raw.githubusercontent.com/JuanVilla424/teamoon/main/install.sh | bash
#
# Installs all prerequisites (system packages, Go, Node, Python, Rust, gh)
# then builds and installs teamoon with systemd service.

REPO="https://github.com/JuanVilla424/teamoon.git"
BRANCH="main"
INSTALL_DIR="${HOME}/.local/src/teamoon"
BINARY_DEST="/usr/local/bin/teamoon"

GO_VERSION="1.24.1"
NVM_VERSION="0.40.1"

# ── helpers ──────────────────────────────────────────────
info()  { printf '\033[1;34m→\033[0m %s\n' "$*"; }
ok()    { printf '\033[1;32m✓\033[0m %s\n' "$*"; }
warn()  { printf '\033[1;33m~\033[0m %s\n' "$*"; }
fail()  { printf '\033[1;31m✗\033[0m %s\n' "$*"; exit 1; }
step()  { printf '\n\033[1;36m── %s ──\033[0m\n' "$*"; }

has() { command -v "$1" &>/dev/null; }

# ── check OS ─────────────────────────────────────────────
if [ ! -f /etc/debian_version ]; then
    fail "this installer requires Ubuntu/Debian (apt). For other distros, see INSTALL.md"
fi

if ! has sudo; then
    fail "sudo is required"
fi

# ── 1. system packages ───────────────────────────────────
step "system packages"

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

# check which are missing
MISSING=()
for pkg in "${PKGS[@]}"; do
    if ! dpkg -s "$pkg" &>/dev/null; then
        MISSING+=("$pkg")
    fi
done

if [ ${#MISSING[@]} -eq 0 ]; then
    ok "all system packages already installed"
else
    info "installing ${#MISSING[@]} packages: ${MISSING[*]:0:5}..."
    sudo apt-get update -qq
    sudo apt-get install -y -qq "${MISSING[@]}"
    ok "system packages installed"
fi

# yq (snap or binary — not in apt on older Ubuntu)
if ! has yq; then
    info "installing yq"
    sudo wget -qO /usr/local/bin/yq https://github.com/mikefarah/yq/releases/latest/download/yq_linux_amd64
    sudo chmod +x /usr/local/bin/yq
    ok "yq installed"
else
    ok "yq already installed"
fi

# ── 2. GitHub CLI ────────────────────────────────────────
step "GitHub CLI (gh)"

if has gh; then
    ok "gh already installed ($(gh --version | head -1))"
else
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
fi

# ── 3. Go ────────────────────────────────────────────────
step "Go ${GO_VERSION}"

install_go() {
    local tarball="go${GO_VERSION}.linux-amd64.tar.gz"
    info "downloading Go ${GO_VERSION}"
    curl -sLO "https://go.dev/dl/${tarball}"
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf "${tarball}"
    rm -f "${tarball}"

    # ensure PATH
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

# ── 4. nvm + Node.js ────────────────────────────────────
step "nvm + Node.js LTS"

export NVM_DIR="${HOME}/.nvm"

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
        nvm use --lts
        ok "node $(node --version) installed"
    fi
else
    info "installing Node.js LTS"
    nvm install --lts
    nvm use --lts
    ok "node $(node --version) installed"
fi

# ── 5. Rust ──────────────────────────────────────────────
step "Rust"

if has rustc; then
    ok "rust already installed ($(rustc --version | awk '{print $2}'))"
else
    info "installing rust via rustup"
    curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
    # shellcheck disable=SC1091
    source "$HOME/.cargo/env"
    ok "rust installed ($(rustc --version | awk '{print $2}'))"
fi

# ── 6. pyenv + Python ───────────────────────────────────
step "pyenv + Python 3.11"

export PYENV_ROOT="${HOME}/.pyenv"

if [ -d "$PYENV_ROOT" ]; then
    export PATH="$PYENV_ROOT/bin:$PATH"
    eval "$(pyenv init -)"
    ok "pyenv already installed"
else
    info "installing pyenv"
    curl -s https://pyenv.run | bash
    export PATH="$PYENV_ROOT/bin:$PATH"
    eval "$(pyenv init -)"

    # add to bashrc if not already there
    if ! grep -q 'PYENV_ROOT' "$HOME/.bashrc" 2>/dev/null; then
        {
            echo 'export PYENV_ROOT="$HOME/.pyenv"'
            echo '[[ -d $PYENV_ROOT/bin ]] && export PATH="$PYENV_ROOT/bin:$PATH"'
            echo 'eval "$(pyenv init -)"'
        } >> "$HOME/.bashrc"
    fi
    ok "pyenv installed"
fi

if pyenv versions --bare 2>/dev/null | grep -q '^3\.12'; then
    ok "python 3.11 already installed via pyenv"
else
    info "installing python 3.11 (this may take a few minutes)"
    pyenv install 3.11
    ok "python 3.11 installed"
fi
pyenv global 3.11

# ── 7. Claude Code ──────────────────────────────────────
step "Claude Code CLI"

if has claude; then
    ok "claude already installed ($(claude --version 2>/dev/null || echo 'unknown'))"
else
    info "installing claude code"
    npm install -g @anthropic-ai/claude-code
    ok "claude installed"
    warn "run 'claude' to authenticate with your Anthropic account"
fi

# ── 8. clone + build + install teamoon ──────────────────
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

info "installing binary + systemd service (requires sudo)"
make install
ok "installed to $BINARY_DEST"

# ── done ─────────────────────────────────────────────────
printf '\n'
printf '\033[1;32m  ╔═══════════════════════════════════════╗\033[0m\n'
printf '\033[1;32m  ║   teamoon installed successfully!     ║\033[0m\n'
printf '\033[1;32m  ╚═══════════════════════════════════════╝\033[0m\n'
printf '\n'
echo "  binary:   $BINARY_DEST"
echo "  source:   $INSTALL_DIR"
echo "  service:  systemctl status teamoon"
echo "  web ui:   http://localhost:3111"
echo ""
echo "  Next steps:"
echo "    1. gh auth login          (if not already authenticated)"
echo "    2. teamoon init           (CLI setup wizard)"
echo "    3. Open web UI            (guided onboarding at http://localhost:3111)"
echo ""
