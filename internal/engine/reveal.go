package engine

// Reveal opens path in the system file manager.
func Reveal(path string, isDir bool) error {
	return revealInFileManager(path, isDir)
}
