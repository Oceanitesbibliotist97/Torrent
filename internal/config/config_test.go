package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ClearNetSky/Torrent/internal/apperr"
	"github.com/ClearNetSky/Torrent/internal/secret"
)

func TestLoadMissingIsFirstRun(t *testing.T) {
	s, firstRun, err := NewStore(t.TempDir()).Load()
	if err != nil {
		t.Fatal(err)
	}
	if !firstRun {
		t.Error("expected firstRun for a fresh directory")
	}
	if s.NetworkMode != NetworkDirect || s.Encryption != EncryptionRequire || !s.AnonymousMode || s.EnableUPnP {
		t.Errorf("unexpected defaults: %+v", s)
	}
}

func TestSaveLoadRoundTripDoesNotLeakPassword(t *testing.T) {
	dir := t.TempDir()
	st := NewStore(dir)
	s := Defaults()
	s.Language = "ru"
	s.NetworkMode = NetworkProxy
	s.Proxy.Host = "127.0.0.1"
	s.Proxy.Password = "s3cret-pass"
	if err := st.Save(s); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, fileName))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("s3cret-pass")) {
		t.Fatal("proxy password was written in plaintext")
	}
	got, firstRun, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	if firstRun {
		t.Error("firstRun should be false after saving")
	}
	if got.Language != "ru" || got.NetworkMode != NetworkProxy || got.Proxy.Host != "127.0.0.1" {
		t.Errorf("settings not preserved: %+v", got)
	}
	wantPassword := ""
	if secret.Supported {
		wantPassword = "s3cret-pass"
	}
	if got.Proxy.Password != wantPassword {
		t.Errorf("password = %q, want %q", got.Proxy.Password, wantPassword)
	}
}

func TestNormalizeClamps(t *testing.T) {
	s := Settings{
		Language:           "de",
		Theme:              "neon",
		DownloadDir:        "relative/dir",
		ListenPort:         80,
		Encryption:         "none",
		NetworkMode:        "tor",
		MaxPeersPerTorrent: 100000,
		SeedRatioLimit:     -1,
		DownloadLimitKiB:   -5,
		Proxy:              Proxy{Port: 70000},
	}
	s.Normalize()
	if s.Language != "" || s.Theme != "system" || s.Encryption != EncryptionRequire || s.NetworkMode != NetworkDirect {
		t.Errorf("enums not normalized: %+v", s)
	}
	if !filepath.IsAbs(s.DownloadDir) && s.DownloadDir != "" {
		t.Errorf("relative download dir kept: %q", s.DownloadDir)
	}
	if s.ListenPort < 1024 || s.MaxPeersPerTorrent != 500 || s.SeedRatioLimit != 0 || s.DownloadLimitKiB != 0 || s.Proxy.Port != 1080 {
		t.Errorf("numbers not clamped: %+v", s)
	}
}

func TestValidate(t *testing.T) {
	s := Defaults()
	s.NetworkMode = NetworkVPN
	if apperr.CodeOf(s.Validate()) != apperr.VPNInterfaceRequired {
		t.Error("VPN mode without an interface must be rejected")
	}
	s.NetworkMode = NetworkProxy
	if apperr.CodeOf(s.Validate()) != apperr.ProxyHostRequired {
		t.Error("proxy mode without a host must be rejected")
	}
	s.Proxy.Host = "127.0.0.1"
	if err := s.Validate(); err != nil {
		t.Errorf("valid proxy settings rejected: %v", err)
	}
}
