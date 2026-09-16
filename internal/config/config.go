// Package config loads, validates and persists user settings.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ClearNetSky/Torrent/internal/apperr"
	"github.com/ClearNetSky/Torrent/internal/fsutil"
	"github.com/ClearNetSky/Torrent/internal/paths"
	"github.com/ClearNetSky/Torrent/internal/secret"
)

const fileName = "settings.json"

// invalidSuffix is appended to a settings file that cannot be parsed, so the
// user can recover values by hand instead of losing them to a fresh default file.
const invalidSuffix = ".invalid"

// Network modes.
const (
	// NetworkDirect uses the default route. Peers see the real IP address.
	NetworkDirect = "direct"
	// NetworkVPN binds every socket to a VPN adapter, with an optional kill switch.
	NetworkVPN = "vpn"
	// NetworkProxy sends all peer and tracker traffic through SOCKS5.
	NetworkProxy = "proxy"
)

// Encryption policies for the BitTorrent protocol (MSE/PE).
const (
	EncryptionPrefer  = "prefer"
	EncryptionRequire = "require"
)

// Proxy holds SOCKS5 proxy settings.
type Proxy struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	// Password lives in memory; on disk it is stored encrypted (see Store.Save).
	Password string `json:"password,omitempty"`
}

// Settings is everything the user can configure.
type Settings struct {
	Language string `json:"language"` // "", "en" or "ru"; "" means detect on first run
	Theme    string `json:"theme"`    // "system", "dark" or "light"

	DownloadDir      string `json:"downloadDir"`
	StartImmediately bool   `json:"startImmediately"`
	WarnExecutables  bool   `json:"warnExecutables"`
	MarkOfTheWeb     bool   `json:"markOfTheWeb"`
	RememberTorrents bool   `json:"rememberTorrents"`

	DownloadLimitKiB   int     `json:"downloadLimitKiB"` // 0 = unlimited
	UploadLimitKiB     int     `json:"uploadLimitKiB"`   // 0 = unlimited
	SeedAfterComplete  bool    `json:"seedAfterComplete"`
	SeedRatioLimit     float64 `json:"seedRatioLimit"` // 0 = no limit
	MaxPeersPerTorrent int     `json:"maxPeersPerTorrent"`

	ListenPort     int  `json:"listenPort"`
	RandomizePort  bool `json:"randomizePort"`
	EnableDHT      bool `json:"enableDHT"`
	EnablePEX      bool `json:"enablePEX"`
	EnableUTP      bool `json:"enableUTP"`
	EnableIPv6     bool `json:"enableIPv6"`
	EnableUPnP     bool `json:"enableUPnP"`
	EnableWebSeeds bool `json:"enableWebSeeds"`

	Encryption    string `json:"encryption"`
	AnonymousMode bool   `json:"anonymousMode"`
	NetworkMode   string `json:"networkMode"`
	VPNInterface  string `json:"vpnInterface"`
	// AutoReconnect resumes transfers when the VPN comes back after the kill
	// switch stopped them. The kill switch itself is always active.
	AutoReconnect bool   `json:"autoReconnect"`
	Proxy         Proxy  `json:"proxy"`
	BlocklistPath string `json:"blocklistPath"`
}

// Defaults returns privacy-first defaults.
func Defaults() Settings {
	return Settings{
		Theme:              "system",
		DownloadDir:        paths.DefaultDownloadDir(),
		StartImmediately:   true,
		WarnExecutables:    true,
		MarkOfTheWeb:       true,
		RememberTorrents:   true,
		SeedAfterComplete:  true,
		SeedRatioLimit:     1,
		MaxPeersPerTorrent: 50,
		ListenPort:         RandomPort(),
		EnableDHT:          true,
		EnablePEX:          true,
		EnableUTP:          true,
		EnableIPv6:         true,
		EnableUPnP:         false,
		EnableWebSeeds:     true,
		Encryption:         EncryptionRequire,
		AnonymousMode:      true,
		NetworkMode:        NetworkDirect,
		AutoReconnect:      true,
		Proxy:              Proxy{Port: 1080},
	}
}

// RandomPort picks an unprivileged port away from ranges commonly throttled for BitTorrent.
func RandomPort() int {
	return 20000 + rand.IntN(40000)
}

// Normalize clamps every value into its valid range.
func (s *Settings) Normalize() {
	switch s.Language {
	case "", "en", "ru":
	default:
		s.Language = ""
	}
	switch s.Theme {
	case "system", "dark", "light":
	default:
		s.Theme = "system"
	}
	s.DownloadDir = strings.TrimSpace(s.DownloadDir)
	if s.DownloadDir == "" || !filepath.IsAbs(s.DownloadDir) {
		s.DownloadDir = paths.DefaultDownloadDir()
	} else {
		s.DownloadDir = filepath.Clean(s.DownloadDir)
	}
	s.DownloadLimitKiB = clamp(s.DownloadLimitKiB, 0, 10_000_000)
	s.UploadLimitKiB = clamp(s.UploadLimitKiB, 0, 10_000_000)
	if math.IsNaN(s.SeedRatioLimit) || s.SeedRatioLimit < 0 {
		s.SeedRatioLimit = 0
	}
	s.SeedRatioLimit = math.Min(s.SeedRatioLimit, 1000)
	if s.MaxPeersPerTorrent == 0 {
		s.MaxPeersPerTorrent = 50
	}
	s.MaxPeersPerTorrent = clamp(s.MaxPeersPerTorrent, 5, 500)
	if s.ListenPort < 1024 || s.ListenPort > 65535 {
		s.ListenPort = RandomPort()
	}
	switch s.Encryption {
	case EncryptionPrefer, EncryptionRequire:
	default:
		s.Encryption = EncryptionRequire
	}
	switch s.NetworkMode {
	case NetworkDirect, NetworkVPN, NetworkProxy:
	default:
		s.NetworkMode = NetworkDirect
	}
	s.VPNInterface = strings.TrimSpace(s.VPNInterface)
	s.Proxy.Host = strings.TrimSpace(s.Proxy.Host)
	if s.Proxy.Port < 1 || s.Proxy.Port > 65535 {
		s.Proxy.Port = 1080
	}
	s.BlocklistPath = strings.TrimSpace(s.BlocklistPath)
}

// Validate reports settings that cannot be applied as-is.
func (s Settings) Validate() error {
	switch s.NetworkMode {
	case NetworkVPN:
		if s.VPNInterface == "" {
			return apperr.New(apperr.VPNInterfaceRequired)
		}
	case NetworkProxy:
		if s.Proxy.Host == "" {
			return apperr.New(apperr.ProxyHostRequired)
		}
	}
	// RFC 1929 limits SOCKS5 username and password to 255 bytes each.
	if len(s.Proxy.Username) > 255 || len(s.Proxy.Password) > 255 {
		return apperr.New(apperr.ProxyCredentials)
	}
	if s.BlocklistPath != "" && !filepath.IsAbs(s.BlocklistPath) {
		return apperr.New(apperr.InvalidPath)
	}
	return nil
}

func clamp(v, lo, hi int) int {
	return max(lo, min(v, hi))
}

type onDisk struct {
	Version int `json:"version"`
	Settings
	ProxyPasswordProtected string `json:"proxyPasswordProtected,omitempty"`
}

// Store reads and writes settings.json inside the data directory.
type Store struct {
	mu   sync.Mutex
	path string
}

// NewStore returns a Store for dir.
func NewStore(dir string) *Store {
	return &Store{path: filepath.Join(dir, fileName)}
}

// Load returns the saved settings, or defaults with firstRun set when none exist.
func (st *Store) Load() (s Settings, firstRun bool, err error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	data, err := os.ReadFile(st.path)
	if errors.Is(err, os.ErrNotExist) {
		return Defaults(), true, nil
	}
	if err != nil {
		return Defaults(), false, err
	}
	disk := onDisk{Settings: Defaults()}
	if err := json.Unmarshal(data, &disk); err != nil {
		_ = os.Rename(st.path, st.path+invalidSuffix)
		return Defaults(), false, fmt.Errorf("parse %s: %w", fileName, err)
	}
	s = disk.Settings
	s.Proxy.Password = ""
	if pw, err := secret.Unprotect(disk.ProxyPasswordProtected); err == nil {
		s.Proxy.Password = pw
	}
	s.Normalize()
	return s, false, nil
}

// Save writes settings atomically with owner-only permissions. The proxy
// password is encrypted with the OS keystore, or not persisted at all when
// encryption is unavailable.
func (st *Store) Save(s Settings) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	disk := onDisk{Version: 1, Settings: s}
	disk.Proxy.Password = ""
	if enc, err := secret.Protect(s.Proxy.Password); err == nil {
		disk.ProxyPasswordProtected = enc
	}
	data, err := json.MarshalIndent(disk, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(st.path, data, 0o600)
}
