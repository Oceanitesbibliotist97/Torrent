//go:build !windows

package paths

import (
	"os"
	"path/filepath"
)

// DefaultDownloadDir returns ~/Downloads.
func DefaultDownloadDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Downloads")
}
