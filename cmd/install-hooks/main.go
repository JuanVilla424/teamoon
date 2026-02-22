package main

import (
	"fmt"
	"os"

	"github.com/JuanVilla424/teamoon/internal/projectinit"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: install-hooks <project-dir>\n")
		os.Exit(1)
	}
	dir := os.Args[1]
	if err := projectinit.InstallHooks(dir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Hooks installed in %s/.claude/\n", dir)
}
