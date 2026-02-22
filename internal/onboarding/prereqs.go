package onboarding

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type toolCheck struct {
	name     string
	required bool
}

// prereqGroup represents a group of tools that can be checked and installed together.
type prereqGroup struct {
	ID          string
	Name        string
	Required    bool
	Installable bool
	CheckFn     func() (version string, found bool)
	InstallFn   func(ProgressFunc) error
}

// tools is the legacy list used by the CLI wizard (step 1).
var tools = []toolCheck{
	{"node", true},
	{"npx", true},
	{"git", true},
	{"gh", false},
	{"claude", false},
}

// prereqGroups is the expanded list for the web installer.
var prereqGroups []prereqGroup

func init() {
	prereqGroups = []prereqGroup{
		{
			ID: "system-packages", Name: "System Packages",
			Required: true, Installable: true,
			CheckFn:  checkSystemPackages,
			InstallFn: installSystemPackages,
		},
		{
			ID: "go", Name: "Go",
			Required: true, Installable: true,
			CheckFn:  checkGo,
			InstallFn: installGo,
		},
		{
			ID: "node", Name: "Node.js",
			Required: true, Installable: true,
			CheckFn:  checkNode,
			InstallFn: installNode,
		},
		{
			ID: "python", Name: "Python",
			Required: false, Installable: true,
			CheckFn:  checkPython,
			InstallFn: installPython,
		},
		{
			ID: "rust", Name: "Rust",
			Required: false, Installable: true,
			CheckFn:  checkRust,
			InstallFn: installRust,
		},
		{
			ID: "gh", Name: "GitHub CLI",
			Required: false, Installable: true,
			CheckFn:  checkGh,
			InstallFn: installGh,
		},
		{
			ID: "claude", Name: "Claude Code",
			Required: false, Installable: true,
			CheckFn:  checkClaude,
			InstallFn: installClaude,
		},
	}
}

// ── CLI check (legacy) ──────────────────────────────────

func checkPrereqs() error {
	fmt.Println("\n[1/7] Checking prerequisites...")

	var missing []string
	for _, tool := range tools {
		version := getVersion(tool.name)
		if version == "" {
			tag := "optional"
			if tool.required {
				tag = "REQUIRED"
				missing = append(missing, tool.name)
			}
			fmt.Printf("  [x] %-10s not found (%s)\n", tool.name, tag)
		} else {
			fmt.Printf("  [+] %-10s %s\n", tool.name, version)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required tools: %s", strings.Join(missing, ", "))
	}
	return nil
}

func getVersion(name string) string {
	cmd := exec.Command(name, "--version")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	return line
}

// ── Web: check all groups ───────────────────────────────

// StreamPrereqs checks all prereq groups and streams results.
func StreamPrereqs(progress ProgressFunc) error {
	var missing []string
	for _, g := range prereqGroups {
		version, found := g.CheckFn()
		if !found && g.Required {
			missing = append(missing, g.Name)
		}
		progress(map[string]any{
			"type": "tool", "id": g.ID, "name": g.Name,
			"version": version, "required": g.Required,
			"found": found, "installable": g.Installable,
		})
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required: %s", strings.Join(missing, ", "))
	}
	return nil
}

// StreamPrereqsInstall installs all missing prereq groups with progress.
func StreamPrereqsInstall(progress ProgressFunc) error {
	var failed []string
	for _, g := range prereqGroups {
		_, found := g.CheckFn()
		if found {
			progress(map[string]any{
				"type": "install", "id": g.ID, "name": g.Name, "status": "skipped",
			})
			continue
		}
		if g.InstallFn == nil {
			progress(map[string]any{
				"type": "install", "id": g.ID, "name": g.Name, "status": "not-installable",
			})
			continue
		}
		progress(map[string]any{
			"type": "install", "id": g.ID, "name": g.Name, "status": "installing",
		})
		if err := g.InstallFn(progress); err != nil {
			failed = append(failed, g.Name)
			progress(map[string]any{
				"type": "install", "id": g.ID, "name": g.Name,
				"status": "error", "message": err.Error(),
			})
			continue
		}
		progress(map[string]any{
			"type": "install", "id": g.ID, "name": g.Name, "status": "done",
		})
	}
	if len(failed) > 0 {
		return fmt.Errorf("failed to install: %s", strings.Join(failed, ", "))
	}
	return nil
}

// ── Check functions ─────────────────────────────────────

// keySystemPackages are the ones we check to determine if the system packages group is installed.
var keySystemPackages = []string{"build-essential", "libssl-dev", "jq", "shellcheck", "yamllint"}

func checkSystemPackages() (string, bool) {
	missing := 0
	for _, pkg := range keySystemPackages {
		cmd := exec.Command("dpkg", "-s", pkg)
		if err := cmd.Run(); err != nil {
			missing++
		}
	}
	if missing > 0 {
		return fmt.Sprintf("%d/%d key packages missing", missing, len(keySystemPackages)), false
	}
	return fmt.Sprintf("%d key packages OK", len(keySystemPackages)), true
}

func checkGo() (string, bool) {
	out := getVersion("go")
	if out == "" {
		return "", false
	}
	// Extract version number
	parts := strings.Fields(out)
	for _, p := range parts {
		if strings.HasPrefix(p, "go") && len(p) > 2 {
			ver := strings.TrimPrefix(p, "go")
			// Check >= 1.24
			segs := strings.SplitN(ver, ".", 3)
			if len(segs) >= 2 {
				minor := 0
				fmt.Sscanf(segs[1], "%d", &minor)
				if minor >= 24 {
					return ver, true
				}
				return ver + " (need >= 1.24)", false
			}
			return ver, true
		}
	}
	return out, true
}

func checkNode() (string, bool) {
	out := getVersion("node")
	if out == "" {
		return "", false
	}
	ver := strings.TrimPrefix(strings.TrimSpace(out), "v")
	segs := strings.SplitN(ver, ".", 3)
	if len(segs) >= 1 {
		major := 0
		fmt.Sscanf(segs[0], "%d", &major)
		if major >= 18 {
			return ver, true
		}
		return ver + " (need >= 18)", false
	}
	return ver, true
}

func checkPython() (string, bool) {
	// Check python3 first, then python
	for _, cmd := range []string{"python3", "python"} {
		out := getVersion(cmd)
		if out == "" {
			continue
		}
		// Extract version
		parts := strings.Fields(out)
		for _, p := range parts {
			if len(p) > 0 && p[0] >= '0' && p[0] <= '9' {
				segs := strings.SplitN(p, ".", 3)
				if len(segs) >= 2 {
					major, minor := 0, 0
					fmt.Sscanf(segs[0], "%d", &major)
					fmt.Sscanf(segs[1], "%d", &minor)
					if major == 3 && minor >= 11 {
						return p, true
					}
					return p + " (need >= 3.11)", false
				}
				return p, true
			}
		}
		return out, true
	}
	return "", false
}

func checkRust() (string, bool) {
	out := getVersion("rustc")
	if out == "" {
		return "", false
	}
	parts := strings.Fields(out)
	if len(parts) >= 2 {
		return parts[1], true
	}
	return out, true
}

func checkGh() (string, bool) {
	out := getVersion("gh")
	if out == "" {
		return "", false
	}
	// "gh version 2.x.x ..."
	parts := strings.Fields(out)
	for i, p := range parts {
		if p == "version" && i+1 < len(parts) {
			return parts[i+1], true
		}
	}
	return out, true
}

func checkClaude() (string, bool) {
	out := getVersion("claude")
	if out == "" {
		return "", false
	}
	return strings.TrimSpace(out), true
}

// ── Install functions ───────────────────────────────────

var allSystemPackages = []string{
	"git", "curl", "wget", "zip", "unzip",
	"build-essential", "gcc", "g++", "make", "cmake",
	"libssl-dev", "zlib1g-dev", "libbz2-dev", "libreadline-dev",
	"libsqlite3-dev", "llvm", "libncursesw5-dev", "xz-utils",
	"tk-dev", "libxml2-dev", "libxmlsec1-dev", "libffi-dev", "liblzma-dev",
	"libgdbm-dev", "libnss3-dev", "libgdbm-compat-dev", "uuid-dev",
	"jq", "htop", "tree", "tmux",
	"shellcheck", "yamllint", "pre-commit",
	"ca-certificates", "gnupg", "lsb-release", "apt-transport-https",
	"software-properties-common", "pkg-config",
}

func installSystemPackages(progress ProgressFunc) error {
	// Find which are missing
	var missing []string
	for _, pkg := range allSystemPackages {
		cmd := exec.Command("dpkg", "-s", pkg)
		if err := cmd.Run(); err != nil {
			missing = append(missing, pkg)
		}
	}
	if len(missing) == 0 {
		return nil
	}

	progress(map[string]any{
		"type": "detail", "message": fmt.Sprintf("Installing %d packages...", len(missing)),
	})

	args := append([]string{"apt-get", "install", "-y", "-qq"}, missing...)
	cmd := exec.Command("sudo", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("apt install failed: %s — %s", err, trimOutput(out))
	}
	return nil
}

const goVersion = "1.24.1"

func installGo(progress ProgressFunc) error {
	arch := runtime.GOARCH
	if arch == "" {
		arch = "amd64"
	}
	tarball := fmt.Sprintf("go%s.linux-%s.tar.gz", goVersion, arch)
	url := "https://go.dev/dl/" + tarball

	progress(map[string]any{"type": "detail", "message": "Downloading Go " + goVersion})

	// Download
	cmd := exec.Command("curl", "-sLO", url)
	cmd.Dir = os.TempDir()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("download failed: %s — %s", err, trimOutput(out))
	}

	tarPath := filepath.Join(os.TempDir(), tarball)
	defer os.Remove(tarPath)

	// Remove old, extract
	progress(map[string]any{"type": "detail", "message": "Installing to /usr/local/go"})
	rmCmd := exec.Command("sudo", "rm", "-rf", "/usr/local/go")
	rmCmd.Run()

	extractCmd := exec.Command("sudo", "tar", "-C", "/usr/local", "-xzf", tarPath)
	if out, err := extractCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("extract failed: %s — %s", err, trimOutput(out))
	}

	// Ensure PATH in bashrc
	home, _ := os.UserHomeDir()
	bashrc := filepath.Join(home, ".bashrc")
	content, _ := os.ReadFile(bashrc)
	if !strings.Contains(string(content), "/usr/local/go/bin") {
		f, err := os.OpenFile(bashrc, os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			f.WriteString("\nexport PATH=$PATH:/usr/local/go/bin:$HOME/go/bin\n")
			f.Close()
		}
	}

	return nil
}

const nvmVersion = "0.40.1"

func installNode(progress ProgressFunc) error {
	home, _ := os.UserHomeDir()
	nvmDir := filepath.Join(home, ".nvm")

	// Install nvm if not present
	if _, err := os.Stat(filepath.Join(nvmDir, "nvm.sh")); err != nil {
		progress(map[string]any{"type": "detail", "message": "Installing nvm " + nvmVersion})
		url := fmt.Sprintf("https://raw.githubusercontent.com/nvm-sh/nvm/v%s/install.sh", nvmVersion)
		cmd := exec.Command("bash", "-c", fmt.Sprintf("curl -so- %s | bash", url))
		cmd.Env = append(os.Environ(), "NVM_DIR="+nvmDir)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("nvm install failed: %s — %s", err, trimOutput(out))
		}
	}

	// Install Node LTS via nvm
	progress(map[string]any{"type": "detail", "message": "Installing Node.js LTS via nvm"})
	script := fmt.Sprintf(`
		export NVM_DIR="%s"
		[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"
		nvm install --lts
		nvm use --lts
		nvm alias default node
	`, nvmDir)
	cmd := exec.Command("bash", "-c", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("node install failed: %s — %s", err, trimOutput(out))
	}

	return nil
}

func installPython(progress ProgressFunc) error {
	home, _ := os.UserHomeDir()
	pyenvRoot := filepath.Join(home, ".pyenv")

	// Install pyenv if not present
	if _, err := os.Stat(pyenvRoot); err != nil {
		progress(map[string]any{"type": "detail", "message": "Installing pyenv"})
		cmd := exec.Command("bash", "-c", "curl -s https://pyenv.run | bash")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("pyenv install failed: %s — %s", err, trimOutput(out))
		}

		// Add to bashrc if not present
		bashrc := filepath.Join(home, ".bashrc")
		content, _ := os.ReadFile(bashrc)
		if !strings.Contains(string(content), "PYENV_ROOT") {
			f, err := os.OpenFile(bashrc, os.O_APPEND|os.O_WRONLY, 0644)
			if err == nil {
				f.WriteString("\nexport PYENV_ROOT=\"$HOME/.pyenv\"\n")
				f.WriteString("[[ -d $PYENV_ROOT/bin ]] && export PATH=\"$PYENV_ROOT/bin:$PATH\"\n")
				f.WriteString("eval \"$(pyenv init -)\"\n")
				f.Close()
			}
		}
	}

	// Install Python 3.11
	progress(map[string]any{"type": "detail", "message": "Installing Python 3.11 (may take a few minutes)"})
	pyenvBin := filepath.Join(pyenvRoot, "bin", "pyenv")
	script := fmt.Sprintf(`
		export PYENV_ROOT="%s"
		export PATH="$PYENV_ROOT/bin:$PATH"
		eval "$(%s init -)"
		%s install -s 3.11
		%s global 3.11
	`, pyenvRoot, pyenvBin, pyenvBin, pyenvBin)
	cmd := exec.Command("bash", "-c", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("python install failed: %s — %s", err, trimOutput(out))
	}

	return nil
}

func installRust(progress ProgressFunc) error {
	progress(map[string]any{"type": "detail", "message": "Installing Rust via rustup"})
	cmd := exec.Command("bash", "-c", "curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("rust install failed: %s — %s", err, trimOutput(out))
	}
	return nil
}

func installGh(progress ProgressFunc) error {
	progress(map[string]any{"type": "detail", "message": "Adding GitHub CLI apt repository"})

	script := `
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
	`
	cmd := exec.Command("bash", "-c", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gh install failed: %s — %s", err, trimOutput(out))
	}
	return nil
}

func installClaude(progress ProgressFunc) error {
	progress(map[string]any{"type": "detail", "message": "Installing Claude Code via npm"})

	home, _ := os.UserHomeDir()
	nvmDir := filepath.Join(home, ".nvm")
	script := fmt.Sprintf(`
		export NVM_DIR="%s"
		[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"
		npm install -g @anthropic-ai/claude-code
	`, nvmDir)
	cmd := exec.Command("bash", "-c", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("claude install failed: %s — %s", err, trimOutput(out))
	}
	return nil
}

// trimOutput trims command output to a reasonable length for error messages.
func trimOutput(out []byte) string {
	s := strings.TrimSpace(string(out))
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}
