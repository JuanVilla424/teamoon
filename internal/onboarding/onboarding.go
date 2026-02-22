package onboarding

import (
	"fmt"

	"github.com/JuanVilla424/teamoon/internal/skills"
)

type stepStatus struct {
	name    string
	done    bool
	warning string
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
