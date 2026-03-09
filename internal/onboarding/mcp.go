package onboarding

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/JuanVilla424/teamoon/internal/config"
)

type defaultMCP struct {
	name    string
	command string
	args    []string
}

var defaultMCPServers = []defaultMCP{
	{"context7", "npx", []string{"-y", "@context7/mcp-server"}},
	{"memory", "npx", []string{"-y", "@modelcontextprotocol/server-memory"}},
	{"sequential-thinking", "npx", []string{"-y", "@modelcontextprotocol/server-sequential-thinking"}},
	{"chrome-devtools", "npx", []string{"-y", "chrome-devtools-mcp@latest", "--isolated=true", "--no-usage-statistics", "--chromeArg=--no-sandbox", "--chromeArg=--disable-setuid-sandbox", "--chromeArg=--disable-gpu", "--chromeArg=--disable-dev-shm-usage"}},
}

// OptionalMCPInfo describes an optional MCP for API responses.
type OptionalMCPInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Installed   bool   `json:"installed"`
}

type optionalMCP struct {
	name        string
	command     string
	args        []string
	description string
	setupFunc   func() error
}

var optionalMCPServers = []optionalMCP{
	{
		name:        "pencil",
		command:     "pencil-mcp",
		args:        []string{"--app", "desktop"},
		description: "Pencil.dev — AI-native design tool for wireframing and UI prototyping",
		setupFunc:   installPencilWrapper,
	},
}

// ListOptionalMCP returns the list of optional MCPs with their install status.
func ListOptionalMCP() []OptionalMCPInfo {
	existing := config.ReadGlobalMCPServers()
	var result []OptionalMCPInfo
	for _, srv := range optionalMCPServers {
		_, installed := existing[srv.name]
		result = append(result, OptionalMCPInfo{
			Name:        srv.name,
			Description: srv.description,
			Installed:   installed,
		})
	}
	return result
}

// StreamOptionalMCP installs selected optional MCPs with progress streaming.
func StreamOptionalMCP(selected []string, progress ProgressFunc) error {
	sel := make(map[string]bool, len(selected))
	for _, s := range selected {
		sel[s] = true
	}

	existing := config.ReadGlobalMCPServers()
	for _, srv := range optionalMCPServers {
		if !sel[srv.name] {
			continue
		}
		if _, ok := existing[srv.name]; ok {
			progress(map[string]any{"type": "server", "name": srv.name, "status": "skipped"})
			continue
		}
		if srv.setupFunc != nil {
			if err := srv.setupFunc(); err != nil {
				progress(map[string]any{"type": "server", "name": srv.name, "status": "error", "error": err.Error()})
				return fmt.Errorf("setup %s: %w", srv.name, err)
			}
		}
		if err := config.InstallMCPToGlobal(srv.name, srv.command, srv.args, nil); err != nil {
			progress(map[string]any{"type": "server", "name": srv.name, "status": "error", "error": err.Error()})
			return fmt.Errorf("installing %s: %w", srv.name, err)
		}
		// Sync to teamoon config so skeleton steps pick it up
		cfg, _ := config.Load()
		if cfg.MCPServers == nil {
			cfg.MCPServers = make(map[string]config.MCPServer)
		}
		if _, exists := cfg.MCPServers[srv.name]; !exists {
			cfg.MCPServers[srv.name] = config.MCPServer{
				Command: srv.command,
				Args:    srv.args,
				Enabled: true,
			}
			config.AttachKnownSkeletonSteps(cfg.MCPServers)
			config.Save(cfg)
		}
		progress(map[string]any{"type": "server", "name": srv.name, "status": "done"})
	}
	return nil
}

const pencilWrapperScript = `#!/bin/bash
# Pencil MCP wrapper — finds the running Pencil AppImage binary
PENCIL_MCP=$(find /tmp -maxdepth 4 -name "mcp-server-linux-x64" -path "*/Pencil*" -path "*/app.asar.unpacked/*" 2>/dev/null | head -1)
if [ -z "$PENCIL_MCP" ]; then
  echo '{"error":"Pencil is not running. Launch the Pencil AppImage first."}' >&2
  exit 1
fi
exec "$PENCIL_MCP" "$@"
`

func installPencilWrapper() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return err
	}
	wrapperPath := filepath.Join(binDir, "pencil-mcp")
	return os.WriteFile(wrapperPath, []byte(pencilWrapperScript), 0755)
}

func installMCPServers() error {
	fmt.Println("\n[6/8] Installing MCP servers...")

	existing := config.ReadGlobalMCPServers()

	for _, srv := range defaultMCPServers {
		if _, ok := existing[srv.name]; ok {
			fmt.Printf("  [~] %-25s already configured\n", srv.name)
			continue
		}
		if err := config.InstallMCPToGlobal(srv.name, srv.command, srv.args, nil); err != nil {
			return fmt.Errorf("installing %s: %w", srv.name, err)
		}
		fmt.Printf("  [+] %-25s installed\n", srv.name)
	}

	return nil
}

// installMCPServersQuiet installs MCP servers without printing to stdout.
func installMCPServersQuiet() error {
	existing := config.ReadGlobalMCPServers()

	for _, srv := range defaultMCPServers {
		if _, ok := existing[srv.name]; ok {
			continue
		}
		if err := config.InstallMCPToGlobal(srv.name, srv.command, srv.args, nil); err != nil {
			return fmt.Errorf("installing %s: %w", srv.name, err)
		}
	}

	return nil
}
