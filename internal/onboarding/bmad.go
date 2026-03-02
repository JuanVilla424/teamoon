package onboarding

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed assets/bmad
var bmadFS embed.FS

type bmadVersions struct {
	Latest    string   `json:"latest"`
	Supported []string `json:"supported"`
}

func installBMAD() error {
	fmt.Println("\n[4/8] Installing BMAD commands...")

	// Read versions manifest
	data, err := bmadFS.ReadFile("assets/bmad/versions.json")
	if err != nil {
		return fmt.Errorf("reading versions.json: %w", err)
	}
	var versions bmadVersions
	if err := json.Unmarshal(data, &versions); err != nil {
		return fmt.Errorf("parsing versions.json: %w", err)
	}

	ver := versions.Latest
	fmt.Printf("  BMAD version: %s\n", ver)

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	destDir := filepath.Join(home, ".claude", "commands", "bmad")

	// Check if already installed
	if _, err := os.Stat(destDir); err == nil {
		if !confirm("BMAD commands already exist. Update?", false) {
			fmt.Println("  [~] Keeping existing BMAD commands")
			return nil
		}
	}

	// Extract embedded files
	srcRoot := fmt.Sprintf("assets/bmad/%s/commands/bmad", ver)
	count := 0

	err = fs.WalkDir(bmadFS, srcRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Compute relative path from srcRoot
		rel, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(destDir, rel)

		if d.IsDir() {
			return os.MkdirAll(dest, 0755)
		}

		content, err := bmadFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", path, err)
		}

		if err := os.WriteFile(dest, content, 0644); err != nil {
			return fmt.Errorf("write %s: %w", dest, err)
		}
		count++
		return nil
	})

	if err != nil {
		return fmt.Errorf("extracting BMAD files: %w", err)
	}

	fmt.Printf("  [+] %d commands installed to %s\n", count, destDir)
	return nil
}

// installBMADStream installs BMAD commands to teamoon home with progress streaming.
func installBMADStream(progress ProgressFunc) error {
	data, err := bmadFS.ReadFile("assets/bmad/versions.json")
	if err != nil {
		return fmt.Errorf("reading versions.json: %w", err)
	}
	var versions bmadVersions
	if err := json.Unmarshal(data, &versions); err != nil {
		return fmt.Errorf("parsing versions.json: %w", err)
	}

	ver := versions.Latest
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	destDir := filepath.Join(teamoonHome(), "commands", "bmad")
	srcRoot := fmt.Sprintf("assets/bmad/%s/commands/bmad", ver)

	// Count total files
	total := 0
	_ = fs.WalkDir(bmadFS, srcRoot, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			total++
		}
		return err
	})

	count := 0
	err = fs.WalkDir(bmadFS, srcRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(destDir, rel)
		if d.IsDir() {
			return os.MkdirAll(dest, 0755)
		}
		content, err := bmadFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", path, err)
		}
		if err := os.WriteFile(dest, content, 0644); err != nil {
			return fmt.Errorf("write %s: %w", dest, err)
		}
		count++
		progress(map[string]any{"type": "progress", "count": count, "total": total, "file": rel})
		return nil
	})
	if err != nil {
		return fmt.Errorf("extracting BMAD files: %w", err)
	}

	// Symlink: ~/.claude/commands/bmad → teamoon home
	claudeBmad := filepath.Join(home, ".claude", "commands", "bmad")
	if err := ensureSymlink(destDir, claudeBmad); err != nil {
		return fmt.Errorf("symlink bmad: %w", err)
	}
	progress(map[string]any{"type": "symlink", "name": "bmad", "status": "done"})

	return nil
}

// installBMADWeb installs BMAD without confirmation prompt or stdout output.
func installBMADWeb() error {
	data, err := bmadFS.ReadFile("assets/bmad/versions.json")
	if err != nil {
		return fmt.Errorf("reading versions.json: %w", err)
	}
	var versions bmadVersions
	if err := json.Unmarshal(data, &versions); err != nil {
		return fmt.Errorf("parsing versions.json: %w", err)
	}

	ver := versions.Latest
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	destDir := filepath.Join(home, ".claude", "commands", "bmad")
	srcRoot := fmt.Sprintf("assets/bmad/%s/commands/bmad", ver)

	return fs.WalkDir(bmadFS, srcRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(destDir, rel)
		if d.IsDir() {
			return os.MkdirAll(dest, 0755)
		}
		content, err := bmadFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", path, err)
		}
		return os.WriteFile(dest, content, 0644)
	})
}
