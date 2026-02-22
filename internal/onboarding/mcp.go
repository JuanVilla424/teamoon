package onboarding

import (
	"fmt"

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
}

func installMCPServers() error {
	fmt.Println("\n[6/7] Installing MCP servers...")

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
