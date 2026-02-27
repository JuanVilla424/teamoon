package onboarding

import (
	"os"
	"path/filepath"

	"github.com/JuanVilla424/teamoon/internal/config"
	"github.com/JuanVilla424/teamoon/internal/plugins"
)

// Status describes what has and hasn't been set up yet.
type Status struct {
	Needed bool         `json:"needed"`
	Steps  StepStatuses `json:"steps"`
}

// StepStatuses tracks which onboarding steps have been completed.
type StepStatuses struct {
	Config  bool `json:"config"`
	Skills  bool `json:"skills"`
	BMAD    bool `json:"bmad"`
	Hooks   bool `json:"hooks"`
	MCP     bool `json:"mcp"`
	Plugins bool `json:"plugins"`
}

// Check returns the current onboarding status by inspecting the filesystem.
// Checks teamoon home (~/.config/teamoon/) as primary source of truth.
func Check() Status {
	s := StepStatuses{}
	tmHome := teamoonHome()

	// Config — ~/.config/teamoon/config.json exists?
	s.Config = fileExists(filepath.Join(tmHome, "config.json"))

	// Skills — ~/.config/teamoon/skills/ has at least one entry?
	skillsDir := filepath.Join(tmHome, "skills")
	if entries, err := os.ReadDir(skillsDir); err == nil && len(entries) > 0 {
		s.Skills = true
	}

	// BMAD — ~/.config/teamoon/commands/bmad/ exists?
	s.BMAD = dirExists(filepath.Join(tmHome, "commands", "bmad"))

	// Hooks — ~/.config/teamoon/hooks/security-check.sh exists?
	s.Hooks = fileExists(filepath.Join(tmHome, "hooks", "security-check.sh"))

	// MCP — at least one MCP server in settings.json?
	existing := config.ReadGlobalMCPServers()
	s.MCP = len(existing) > 0

	// Plugins — at least one default plugin installed?
	for _, dp := range plugins.DefaultPlugins {
		if plugins.IsInstalled(dp.Name) {
			s.Plugins = true
			break
		}
	}

	needed := !s.Config || !s.Skills || !s.BMAD || !s.Hooks || !s.MCP || !s.Plugins
	return Status{Needed: needed, Steps: s}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
