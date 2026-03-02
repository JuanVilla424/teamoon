package onboarding

import (
	"fmt"

	"github.com/JuanVilla424/teamoon/internal/plugins"
)

func installPlugins() error {
	fmt.Println("\n[7/8] Installing Claude Code plugins...")

	for _, p := range plugins.DefaultPlugins {
		if plugins.IsInstalled(p.Name) {
			fmt.Printf("  [~] %-25s already installed\n", p.Name)
			continue
		}
		if err := plugins.Install(p.Name, p.Marketplace); err != nil {
			return fmt.Errorf("installing %s: %w", p.Name, err)
		}
		fmt.Printf("  [+] %-25s installed\n", p.Name)
	}

	return nil
}

func installPluginsQuiet() error {
	for _, p := range plugins.DefaultPlugins {
		if plugins.IsInstalled(p.Name) {
			continue
		}
		if err := plugins.Install(p.Name, p.Marketplace); err != nil {
			return fmt.Errorf("installing %s: %w", p.Name, err)
		}
	}
	return nil
}

// StreamPlugins installs default plugins with per-plugin progress via SSE.
func StreamPlugins(progress ProgressFunc) error {
	for _, p := range plugins.DefaultPlugins {
		if plugins.IsInstalled(p.Name) {
			progress(map[string]any{"type": "plugin", "name": p.Name, "status": "skipped"})
			continue
		}
		if err := plugins.Install(p.Name, p.Marketplace); err != nil {
			return fmt.Errorf("installing %s: %w", p.Name, err)
		}
		progress(map[string]any{"type": "plugin", "name": p.Name, "status": "done"})
	}
	return nil
}
