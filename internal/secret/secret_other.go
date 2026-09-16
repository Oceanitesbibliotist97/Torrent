//go:build !windows

// Package secret protects small secrets, such as a proxy password, at rest.
package secret

import "errors"

// Supported reports whether secrets can be stored encrypted on this platform.
const Supported = false

var errUnsupported = errors.New("encrypted secret storage is not available on this platform")

// Protect refuses to store secrets in plaintext; callers keep them in memory only.
func Protect(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	return "", errUnsupported
}

// Unprotect reverses Protect.
func Unprotect(enc string) (string, error) {
	if enc == "" {
		return "", nil
	}
	return "", errUnsupported
}
