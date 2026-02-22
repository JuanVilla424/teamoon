package main

import (
	"fmt"
	"os"

	"github.com/JuanVilla424/teamoon/internal/skills"
)

func main() {
	if err := skills.Install(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Skills installed successfully")
}
