package onboarding

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/JuanVilla424/teamoon/internal/config"
	"github.com/JuanVilla424/teamoon/internal/skills"
)

// ProgressFunc is the callback type for streaming step progress via SSE.
type ProgressFunc func(map[string]any)

// ToolResult represents a single tool check for the prerequisites step.
type ToolResult struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Required bool   `json:"required"`
	Found    bool   `json:"found"`
}

type stepStatus struct {
	name    string
	done    bool
	warning string
}

// WebConfig holds the config values submitted by the onboarding form.
type WebConfig struct {
	ProjectsDir   string `json:"projects_dir"`
	WebPort       int    `json:"web_port"`
	WebPassword   string `json:"web_password"`
	MaxConcurrent int    `json:"max_concurrent"`
}

// Run executes the full interactive onboarding wizard.
func Run() error {
	fmt.Println("teamoon init — interactive setup wizard")
	fmt.Println("========================================")

	var steps []stepStatus

	// Step 1: Prerequisites
	if err := checkPrereqs(); err != nil {
		return err
	}
	steps = append(steps, stepStatus{name: "Prerequisites", done: true})

	// Step 2: Config
	if err := setupConfig(); err != nil {
		fmt.Printf("  [!] Config: %v\n", err)
		steps = append(steps, stepStatus{name: "Config", warning: err.Error()})
	} else {
		steps = append(steps, stepStatus{name: "Config", done: true})
	}

	// Step 3: Skills
	fmt.Println("\n[3/7] Installing Claude Code skills...")
	if err := skills.Install(); err != nil {
		fmt.Printf("  [!] Some skills failed: %v\n", err)
		steps = append(steps, stepStatus{name: "Skills", warning: err.Error()})
	} else {
		fmt.Println("  [+] All skills installed")
		steps = append(steps, stepStatus{name: "Skills", done: true})
	}

	// Step 4: BMAD
	if err := installBMAD(); err != nil {
		fmt.Printf("  [!] BMAD: %v\n", err)
		steps = append(steps, stepStatus{name: "BMAD", warning: err.Error()})
	} else {
		steps = append(steps, stepStatus{name: "BMAD", done: true})
	}

	// Step 5: Global hooks
	if err := installGlobalHooks(); err != nil {
		fmt.Printf("  [!] Hooks: %v\n", err)
		steps = append(steps, stepStatus{name: "Global Hooks", warning: err.Error()})
	} else {
		steps = append(steps, stepStatus{name: "Global Hooks", done: true})
	}

	// Step 6: MCP servers
	if err := installMCPServers(); err != nil {
		fmt.Printf("  [!] MCP: %v\n", err)
		steps = append(steps, stepStatus{name: "MCP Servers", warning: err.Error()})
	} else {
		steps = append(steps, stepStatus{name: "MCP Servers", done: true})
	}

	// Step 7: Summary
	printSummary(steps)
	return nil
}

func printSummary(steps []stepStatus) {
	fmt.Println("\n[7/7] Summary")
	fmt.Println("========================================")
	for _, s := range steps {
		if s.done {
			fmt.Printf("  [+] %-20s done\n", s.name)
		} else {
			fmt.Printf("  [!] %-20s %s\n", s.name, s.warning)
		}
	}
	fmt.Println("\nRun `teamoon` to open the TUI dashboard.")
	fmt.Println("Run `teamoon serve` to start the web dashboard.")
}

// WebCheckPrereqs checks prerequisites and returns structured results.
func WebCheckPrereqs() ([]ToolResult, error) {
	var results []ToolResult
	var missing []string
	for _, tool := range tools {
		version := getVersion(tool.name)
		found := version != ""
		if !found && tool.required {
			missing = append(missing, tool.name)
		}
		results = append(results, ToolResult{
			Name:     tool.name,
			Version:  version,
			Required: tool.required,
			Found:    found,
		})
	}
	if len(missing) > 0 {
		return results, fmt.Errorf("missing required tools: %s", strings.Join(missing, ", "))
	}
	return results, nil
}

// WebSaveConfig saves config from web-submitted values (no stdin interaction).
func WebSaveConfig(wc WebConfig) error {
	return saveConfigWeb(wc)
}

// WebInstallSkills installs Claude Code skills.
func WebInstallSkills() error {
	return skills.Install()
}

// WebInstallBMAD installs BMAD commands (force, no confirmation prompt).
func WebInstallBMAD() error {
	return installBMADWeb()
}

// WebInstallHooks installs global hooks and merges settings.json.
func WebInstallHooks() error {
	return installGlobalHooksQuiet()
}

// WebInstallMCP installs default MCP servers.
func WebInstallMCP() error {
	return installMCPServersQuiet()
}

// StreamSkills sets up the teamoon home symlink and installs skills with per-skill progress.
func StreamSkills(progress ProgressFunc) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	tmSkillsDir := filepath.Join(teamoonHome(), "skills")
	if err := os.MkdirAll(tmSkillsDir, 0755); err != nil {
		return fmt.Errorf("create skills dir: %w", err)
	}

	// Migrate existing skills dir to teamoon home and create symlink
	agentsSkillsDir := filepath.Join(home, ".agents", "skills")
	info, lErr := os.Lstat(agentsSkillsDir)
	if lErr == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
		entries, _ := os.ReadDir(agentsSkillsDir)
		for _, e := range entries {
			src := filepath.Join(agentsSkillsDir, e.Name())
			dst := filepath.Join(tmSkillsDir, e.Name())
			if _, err := os.Stat(dst); err != nil {
				os.Rename(src, dst)
			}
		}
		os.RemoveAll(agentsSkillsDir)
	}

	if err := ensureSymlink(tmSkillsDir, agentsSkillsDir); err != nil {
		return fmt.Errorf("symlink skills: %w", err)
	}
	progress(map[string]any{"type": "symlink", "name": "skills", "status": "done"})

	return skills.InstallWithProgress(func(name, status string) {
		progress(map[string]any{"type": "skill", "name": name, "status": status})
	})
}

// StreamBMAD installs BMAD commands to teamoon home with per-file progress.
func StreamBMAD(progress ProgressFunc) error {
	return installBMADStream(progress)
}

// StreamHooks installs security hooks to teamoon home with per-hook progress.
func StreamHooks(progress ProgressFunc) error {
	return installHooksStream(progress)
}

// StreamMCP installs MCP servers with per-server progress.
func StreamMCP(progress ProgressFunc) error {
	existing := config.ReadGlobalMCPServers()
	for _, srv := range defaultMCPServers {
		if _, ok := existing[srv.name]; ok {
			progress(map[string]any{"type": "server", "name": srv.name, "status": "skipped"})
			continue
		}
		if err := config.InstallMCPToGlobal(srv.name, srv.command, srv.args, nil); err != nil {
			return fmt.Errorf("installing %s: %w", srv.name, err)
		}
		progress(map[string]any{"type": "server", "name": srv.name, "status": "done"})
	}
	return nil
}
