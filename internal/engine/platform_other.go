//go:build !windows

package engine

import (
	"os/exec"
	"path/filepath"
	"runtime"
)

// markOfTheWeb is a Windows feature; other platforms have no equivalent.
func markOfTheWeb([]string) {}

// revealInFileManager opens the folder containing path.
func revealInFileManager(path string, isDir bool) error {
	dir := path
	if !isDir {
		dir = filepath.Dir(path)
	}
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	cmd := exec.Command(name, dir)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}
