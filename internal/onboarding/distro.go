package onboarding

import (
	"os"
	"strings"
)

type distroFamily string

const (
	distroDebian  distroFamily = "debian"
	distroRHEL    distroFamily = "rhel"
	distroUnknown distroFamily = "unknown"
)

// detectDistro reads /etc/os-release to determine the distro family.
// Falls back to file presence checks for older distros.
func detectDistro() distroFamily {
	data, err := os.ReadFile("/etc/os-release")
	if err == nil {
		content := string(data)
		for _, line := range strings.Split(content, "\n") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			field := parts[0]
			value := strings.Trim(parts[1], `"`)

			switch field {
			case "ID_LIKE":
				if strings.Contains(value, "debian") || strings.Contains(value, "ubuntu") {
					return distroDebian
				}
				if strings.Contains(value, "rhel") || strings.Contains(value, "fedora") || strings.Contains(value, "centos") {
					return distroRHEL
				}
			case "ID":
				switch value {
				case "debian", "ubuntu":
					return distroDebian
				case "rhel", "centos", "rocky", "almalinux", "fedora":
					return distroRHEL
				}
			}
		}
	}

	// Fallback: file presence
	if _, err := os.Stat("/etc/debian_version"); err == nil {
		return distroDebian
	}
	if _, err := os.Stat("/etc/redhat-release"); err == nil {
		return distroRHEL
	}
	return distroUnknown
}

var currentDistro = detectDistro()
