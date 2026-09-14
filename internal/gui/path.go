package gui

import (
	"os"
	"path/filepath"
)

func nearestExistingParent(path string) (string, bool) {
	if path == "" {
		return "", false
	}
	directory := filepath.Clean(filepath.Dir(path))
	for {
		if info, err := os.Stat(directory); err == nil && info.IsDir() {
			return directory, true
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", false
		}
		directory = parent
	}
}
