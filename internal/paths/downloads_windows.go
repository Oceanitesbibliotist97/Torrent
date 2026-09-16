//go:build windows

package paths

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// DefaultDownloadDir returns the user's Downloads known folder, which may have
// been relocated away from the profile directory.
func DefaultDownloadDir() string {
	if dir, err := windows.KnownFolderPath(windows.FOLDERID_Downloads, 0); err == nil && dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Downloads")
}
