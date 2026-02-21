package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	ProjectsDir        string  `json:"projects_dir"`
	ClaudeDir          string  `json:"claude_dir"`
	RefreshIntervalSec int     `json:"refresh_interval_sec"`
	BudgetMonthly      float64 `json:"budget_monthly"`
	ContextLimit       int     `json:"context_limit"`
	WebEnabled         bool    `json:"web_enabled"`
	WebPort            int     `json:"web_port"`
	WebPassword        string  `json:"web_password"`
	WebhookURL         string  `json:"webhook_url,omitempty"`
}

func DefaultConfig() Config {
	home, _ := os.UserHomeDir()
	return Config{
		ProjectsDir:        filepath.Join(home, "Projects"),
		ClaudeDir:          filepath.Join(home, ".claude"),
		RefreshIntervalSec: 30,
		BudgetMonthly:      0,
		ContextLimit:       0,
		WebEnabled:         false,
		WebPort:            7777,
		WebPassword:        "",
	}
}

func ConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "teamoon")
}

func Load() (Config, error) {
	cfg := DefaultConfig()
	path := filepath.Join(ConfigDir(), "config.json")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if saveErr := Save(cfg); saveErr != nil {
				return cfg, saveErr
			}
			return cfg, nil
		}
		return cfg, err
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func Save(cfg Config) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "config.json"), data, 0644)
}
