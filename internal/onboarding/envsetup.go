package onboarding

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/JuanVilla424/teamoon/internal/config"
)

// SetupEnvFile detects CLAUDE_CODE_* environment variables and writes them
// to ~/.config/teamoon/.env for the systemd service. If an existing .env
// is found, it is backed up to .env.bak before overwriting.
func SetupEnvFile() error {
	vars := collectClaudeEnvVars()

	// Fallback: parse shell RC files (useful when running inside systemd)
	if len(vars) == 0 {
		vars = parseShellRCFiles()
	}

	if len(vars) == 0 {
		return nil
	}

	envPath := filepath.Join(config.ConfigDir(), ".env")

	// Backup existing .env
	if _, err := os.Stat(envPath); err == nil {
		backupPath := envPath + ".bak"
		os.Rename(envPath, backupPath)
	}

	// Write sorted for deterministic output
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var buf strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&buf, "%s=%s\n", k, vars[k])
	}

	return os.WriteFile(envPath, []byte(buf.String()), 0600)
}

// collectClaudeEnvVars reads CLAUDE_CODE_* vars from the current process environment.
func collectClaudeEnvVars() map[string]string {
	vars := make(map[string]string)
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "CLAUDE_CODE_") {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) == 2 && parts[1] != "" {
				vars[parts[0]] = parts[1]
			}
		}
	}
	return vars
}

// parseShellRCFiles reads ~/.bashrc and ~/.zshrc looking for
// "export CLAUDE_CODE_*=value" lines.
func parseShellRCFiles() map[string]string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	vars := make(map[string]string)
	for _, rc := range []string{".bashrc", ".zshrc", ".profile"} {
		path := filepath.Join(home, rc)
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if !strings.HasPrefix(line, "export CLAUDE_CODE_") {
				continue
			}
			// Remove "export " prefix
			line = strings.TrimPrefix(line, "export ")
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				val := strings.Trim(parts[1], "\"'")
				if val != "" {
					vars[parts[0]] = val
				}
			}
		}
		f.Close()
	}
	return vars
}
