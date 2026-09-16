// Package paths resolves where the application keeps its data.
package paths

import (
	"os"
	"path/filepath"

	"github.com/ClearNetSky/Torrent/internal/appinfo"
)

// PortableDirName is the folder created next to the executable in portable mode.
const PortableDirName = "TorrentData"

// Location describes the data directory in use.
type Location struct {
	Dir      string
	Portable bool
}

// Resolve prefers a folder next to the executable (portable mode, nothing is
// written to the registry or user profile) and falls back to the per-user
// config directory only when the executable's folder is read-only.
// TORRENT_DATA_DIR overrides both, which is useful for development.
func Resolve() (Location, error) {
	if dir := os.Getenv("TORRENT_DATA_DIR"); dir != "" {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return Location{}, err
		}
		return Location{Dir: abs, Portable: true}, ensureDir(abs)
	}
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		dir := filepath.Join(filepath.Dir(exe), PortableDirName)
		if ensureDir(dir) == nil && writable(dir) {
			return Location{Dir: dir, Portable: true}, nil
		}
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return Location{}, err
	}
	dir := filepath.Join(base, appinfo.Name)
	if err := ensureDir(dir); err != nil {
		return Location{}, err
	}
	return Location{Dir: dir, Portable: false}, nil
}

func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0o700)
}

func writable(dir string) bool {
	f, err := os.CreateTemp(dir, ".write-test-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return true
}
