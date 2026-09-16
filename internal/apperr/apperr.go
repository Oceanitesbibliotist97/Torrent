// Package apperr defines stable error codes that the UI translates.
//
// Errors cross the Go/JS boundary as strings in the form "code" or
// "code: detail", so the frontend can localize the code and show the detail.
package apperr

import "errors"

// Code is a stable, translatable error identifier.
type Code string

const (
	InvalidMagnet        Code = "invalidMagnet"
	InvalidTorrent       Code = "invalidTorrent"
	TooLarge             Code = "tooLarge"
	Duplicate            Code = "duplicate"
	NotFound             Code = "notFound"
	InvalidPath          Code = "invalidPath"
	InvalidInput         Code = "invalidInput"
	EngineOffline        Code = "engineOffline"
	Network              Code = "network"
	VPNInterfaceRequired Code = "vpnInterfaceRequired"
	VPNInterfaceDown     Code = "vpnInterfaceDown"
	ProxyHostRequired    Code = "proxyHostRequired"
	ProxyCredentials     Code = "proxyCredentials"
	Blocklist            Code = "blocklist"
	IO                   Code = "io"
	LinkNotAllowed       Code = "linkNotAllowed"
	Internal             Code = "internal"
)

// Error carries a Code and an optional human-readable detail.
type Error struct {
	Code   Code
	Detail string
}

func (e *Error) Error() string {
	if e.Detail == "" {
		return string(e.Code)
	}
	return string(e.Code) + ": " + e.Detail
}

// New returns an error with only a code.
func New(code Code) error {
	return &Error{Code: code}
}

// Detail returns an error with a code and detail text.
func Detail(code Code, detail string) error {
	return &Error{Code: code, Detail: detail}
}

// Wrap attaches a code to err, keeping an existing code if err already has one.
func Wrap(code Code, err error) error {
	if err == nil {
		return nil
	}
	var ae *Error
	if errors.As(err, &ae) {
		return err
	}
	return &Error{Code: code, Detail: err.Error()}
}

// CodeOf extracts the code from err, or "" when it has none.
func CodeOf(err error) Code {
	var ae *Error
	if errors.As(err, &ae) {
		return ae.Code
	}
	return ""
}
