//go:build !windows

package fsutil

import "golang.org/x/sys/unix"

// FreeSpace returns the bytes available to the current user on the volume that
// holds path (or its nearest existing parent).
func FreeSpace(path string) (int64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(existingAncestor(path), &st); err != nil {
		return 0, err
	}
	return int64(st.Bavail) * int64(st.Bsize), nil
}
