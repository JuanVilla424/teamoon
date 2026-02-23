package onboarding

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/JuanVilla424/teamoon/internal/config"
)

func setupConfig() error {
	fmt.Println("\n[2/7] Setting up configuration...")

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	configDir := filepath.Join(home, ".config", "teamoon")
	configPath := filepath.Join(configDir, "config.json")

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("  Config already exists: %s\n", configPath)
		if !confirm("Reconfigure?", false) {
			fmt.Println("  [~] Keeping existing config")
			return nil
		}
	}

	cfg := config.DefaultConfig()

	// Interactive prompts
	projectsDir := ask("Projects directory", cfg.ProjectsDir)
	cfg.ProjectsDir = expandHome(projectsDir, home)

	portStr := ask("Web dashboard port", strconv.Itoa(cfg.WebPort))
	if p, err := strconv.Atoi(portStr); err == nil && p > 0 && p < 65536 {
		cfg.WebPort = p
	}

	hostStr := ask("Bind address (localhost or 0.0.0.0)", cfg.WebHost)
	if hostStr == "localhost" || hostStr == "0.0.0.0" || hostStr == "127.0.0.1" {
		cfg.WebHost = hostStr
	}

	cfg.WebPassword = ask("Web password (empty = no auth)", cfg.WebPassword)

	concStr := ask("Max concurrent autopilot sessions", strconv.Itoa(cfg.MaxConcurrent))
	if c, err := strconv.Atoi(concStr); err == nil && c > 0 {
		cfg.MaxConcurrent = c
	}

	// Save
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("  [+] Config saved: %s\n", configPath)
	return nil
}

func expandHome(path, home string) string {
	if len(path) >= 2 && path[:2] == "~/" {
		return filepath.Join(home, path[2:])
	}
	if path == "~" {
		return home
	}
	return path
}

// saveConfigWeb saves config from web-submitted values (non-interactive).
func saveConfigWeb(wc WebConfig) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	cfg := config.DefaultConfig()
	if wc.ProjectsDir != "" {
		cfg.ProjectsDir = expandHome(wc.ProjectsDir, home)
	}
	if wc.WebPort > 0 && wc.WebPort < 65536 {
		cfg.WebPort = wc.WebPort
	}
	if wc.WebHost != "" {
		cfg.WebHost = wc.WebHost
	}
	cfg.WebPassword = wc.WebPassword
	if wc.MaxConcurrent > 0 {
		cfg.MaxConcurrent = wc.MaxConcurrent
	}

	return config.Save(cfg)
}
