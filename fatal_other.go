//go:build !windows

package main

import (
	"fmt"
	"os"
)

// showFatal reports a startup failure.
func showFatal(title, msg string) {
	fmt.Fprintln(os.Stderr, title+": "+msg)
}
