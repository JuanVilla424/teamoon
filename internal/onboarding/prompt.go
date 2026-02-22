package onboarding

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

var scanner = bufio.NewScanner(os.Stdin)

// ask displays a prompt with a default value and returns the user's input.
// If the user enters nothing, the default is returned.
func ask(prompt, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("  %s [%s]: ", prompt, defaultVal)
	} else {
		fmt.Printf("  %s: ", prompt)
	}
	scanner.Scan()
	input := strings.TrimSpace(scanner.Text())
	if input == "" {
		return defaultVal
	}
	return input
}

// confirm displays a y/n prompt and returns true for yes.
func confirm(prompt string, defaultYes bool) bool {
	hint := "y/N"
	if defaultYes {
		hint = "Y/n"
	}
	fmt.Printf("  %s [%s]: ", prompt, hint)
	scanner.Scan()
	input := strings.TrimSpace(strings.ToLower(scanner.Text()))
	if input == "" {
		return defaultYes
	}
	return input == "y" || input == "yes"
}
