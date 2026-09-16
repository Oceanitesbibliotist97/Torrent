//go:build windows

package engine

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

// zoneIdentifier marks a file as downloaded from the Internet (zone 3), so
// SmartScreen and Office Protected View treat it with caution. No source URL
// is recorded, so the mark reveals nothing about where the file came from.
const zoneIdentifier = "[ZoneTransfer]\r\nZoneId=3\r\n"

func markOfTheWeb(paths []string) {
	for _, p := range paths {
		if st, err := os.Stat(p); err != nil || !st.Mode().IsRegular() {
			continue
		}
		_ = os.WriteFile(p+":Zone.Identifier", []byte(zoneIdentifier), 0o644)
	}
}

// revealInFileManager opens Explorer at path, selecting it when it is a file.
func revealInFileManager(path string, isDir bool) error {
	// Quotes cannot appear in Windows paths; refusing them rules out argument injection.
	if strings.ContainsRune(path, '"') {
		return errors.New("invalid path")
	}
	winDir, err := windows.GetSystemWindowsDirectory()
	if err != nil {
		return err
	}
	explorer := filepath.Join(winDir, "explorer.exe")
	args := `"` + path + `"`
	if !isDir {
		args = `/select,"` + path + `"`
	}
	cmd := exec.Command(explorer)
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `"` + explorer + `" ` + args}
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}
