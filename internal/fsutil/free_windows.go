//go:build windows

package fsutil

import "golang.org/x/sys/windows"

// FreeSpace returns the bytes available to the current user on the volume that
// holds path (or its nearest existing parent).
func FreeSpace(path string) (int64, error) {
	p, err := windows.UTF16PtrFromString(existingAncestor(path))
	if err != nil {
		return 0, err
	}
	var avail, total, free uint64
	if err := windows.GetDiskFreeSpaceEx(p, &avail, &total, &free); err != nil {
		return 0, err
	}
	return int64(avail), nil
}
