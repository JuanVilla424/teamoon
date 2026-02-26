package pathutil

import (
	"os"
	"path/filepath"
	"strings"
)

// AugmentPath probes for well-known tool directories and adds them to the
// process PATH. Call once at startup so exec.Command finds go, node, claude,
// rustc, python etc. regardless of how the process was launched (systemd,
// terminal, cron).
func AugmentPath() {
	home := os.Getenv("HOME")
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	if home == "" {
		return
	}

	extra := []string{
		filepath.Join(home, "bin"),
		filepath.Join(home, ".local", "bin"),
		filepath.Join(home, "go", "bin"),
		filepath.Join(home, ".cargo", "bin"),
		filepath.Join(home, ".bun", "bin"),
		filepath.Join(home, ".pyenv", "shims"),
		filepath.Join(home, ".pyenv", "bin"),
		"/usr/local/go/bin",
		"/usr/local/bin",
		"/usr/bin",
		"/usr/sbin",
		"/usr/local/sbin",
		"/opt/homebrew/bin",
		"/opt/homebrew/sbin",
		"/snap/bin",
	}

	// Find nvm's active node version — pick the latest installed version.
	nvmDir := filepath.Join(home, ".nvm", "versions", "node")
	if entries, err := os.ReadDir(nvmDir); err == nil {
		for i := len(entries) - 1; i >= 0; i-- {
			if !entries[i].IsDir() {
				continue
			}
			binDir := filepath.Join(nvmDir, entries[i].Name(), "bin")
			if _, err := os.Stat(binDir); err == nil {
				extra = append(extra, binDir)
				break
			}
		}
	}

	current := os.Getenv("PATH")
	existing := make(map[string]bool)
	for _, p := range strings.Split(current, ":") {
		existing[p] = true
	}

	var toAdd []string
	for _, p := range extra {
		if existing[p] {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			toAdd = append(toAdd, p)
		}
	}

	if len(toAdd) > 0 {
		os.Setenv("PATH", current+":"+strings.Join(toAdd, ":"))
	}

	// Set tool-specific env vars that may be missing in service context
	cargoHome := filepath.Join(home, ".cargo")
	if _, err := os.Stat(cargoHome); err == nil {
		if os.Getenv("CARGO_HOME") == "" {
			os.Setenv("CARGO_HOME", cargoHome)
		}
		rustupHome := filepath.Join(home, ".rustup")
		if _, err := os.Stat(rustupHome); err == nil {
			if os.Getenv("RUSTUP_HOME") == "" {
				os.Setenv("RUSTUP_HOME", rustupHome)
			}
			// rustc needs its toolchain lib dir in LD_LIBRARY_PATH
			toolchains := filepath.Join(rustupHome, "toolchains")
			if entries, err := os.ReadDir(toolchains); err == nil {
				for _, e := range entries {
					if !e.IsDir() {
						continue
					}
					libDir := filepath.Join(toolchains, e.Name(), "lib")
					if _, err := os.Stat(libDir); err == nil {
						ldPath := os.Getenv("LD_LIBRARY_PATH")
						if ldPath == "" {
							os.Setenv("LD_LIBRARY_PATH", libDir)
						} else if !strings.Contains(ldPath, libDir) {
							os.Setenv("LD_LIBRARY_PATH", ldPath+":"+libDir)
						}
					}
				}
			}
		}
	}
}
