package onboarding

import (
	"os"
	"path/filepath"
)

func teamoonHome() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "teamoon")
}

func ensureSymlink(target, link string) error {
	if dest, err := os.Readlink(link); err == nil && dest == target {
		return nil
	}
	os.RemoveAll(link)
	if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
		return err
	}
	return os.Symlink(target, link)
}
