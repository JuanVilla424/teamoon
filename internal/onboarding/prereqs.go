package onboarding

import (
	"fmt"
	"os/exec"
	"strings"
)

type toolCheck struct {
	name     string
	required bool
}

var tools = []toolCheck{
	{"node", true},
	{"npx", true},
	{"git", true},
	{"gh", false},
	{"claude", false},
}

func checkPrereqs() error {
	fmt.Println("\n[1/7] Checking prerequisites...")

	var missing []string
	for _, tool := range tools {
		version := getVersion(tool.name)
		if version == "" {
			tag := "optional"
			if tool.required {
				tag = "REQUIRED"
				missing = append(missing, tool.name)
			}
			fmt.Printf("  [x] %-10s not found (%s)\n", tool.name, tag)
		} else {
			fmt.Printf("  [+] %-10s %s\n", tool.name, version)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required tools: %s", strings.Join(missing, ", "))
	}
	return nil
}

func getVersion(name string) string {
	cmd := exec.Command(name, "--version")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	// Return first line, trimmed
	line := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	return line
}
