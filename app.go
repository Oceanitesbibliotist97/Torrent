package main

import (
	"context"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/ClearNetSky/Torrent/internal/appinfo"
	"github.com/ClearNetSky/Torrent/internal/apperr"
	"github.com/ClearNetSky/Torrent/internal/config"
	"github.com/ClearNetSky/Torrent/internal/engine"
	"github.com/ClearNetSky/Torrent/internal/fsutil"
	"github.com/ClearNetSky/Torrent/internal/netguard"
	"github.com/ClearNetSky/Torrent/internal/paths"
	"github.com/ClearNetSky/Torrent/internal/safety"
	"github.com/ClearNetSky/Torrent/internal/secret"
)

const (
	maxIDs         = 10_000
	eventOpenArgs  = "open-args"
	maxOpenArgs    = 20
	maxDialogTitle = 120
)

// App is bound to the frontend. Every exported method can be called from
// JavaScript, so every argument is treated as untrusted.
type App struct {
	ctx      context.Context
	loc      paths.Location
	store    *config.Store
	firstRun bool
	loadErr  error

	mu        sync.Mutex
	settings  config.Settings
	engine    *engine.Engine
	engineErr error
	args      []string
}

// NewApp creates the application backend.
func NewApp(loc paths.Location, store *config.Store, s config.Settings, firstRun bool, loadErr error, args []string) *App {
	return &App{
		loc:      loc,
		store:    store,
		settings: s,
		firstRun: firstRun,
		loadErr:  loadErr,
		args:     openableArgs(args),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.settings.RandomizePort {
		a.settings.ListenPort = config.RandomPort()
	}
	if a.firstRun || a.settings.RandomizePort {
		_ = a.store.Save(a.settings)
	}
	eng, err := engine.New(a.loc.Dir, a.settings, func(name string, data any) {
		runtime.EventsEmit(ctx, name, data)
	})
	if err != nil {
		a.engineErr = err
		return
	}
	a.engine = eng
	eng.Start()
}

// beforeClose stops every transfer while the window is still alive, so the
// engine never emits events into a destroyed frontend.
func (a *App) beforeClose(context.Context) bool {
	a.closeEngine()
	return false
}

func (a *App) shutdown(context.Context) {
	a.closeEngine()
}

func (a *App) closeEngine() {
	a.mu.Lock()
	eng := a.engine
	a.mu.Unlock()
	if eng != nil {
		eng.Close()
	}
}

func (a *App) onSecondInstance(data options.SecondInstanceData) {
	if a.ctx == nil {
		return
	}
	runtime.WindowUnminimise(a.ctx)
	runtime.WindowShow(a.ctx)
	if args := openableArgs(data.Args); len(args) > 0 {
		runtime.EventsEmit(a.ctx, eventOpenArgs, args)
	}
}

// openableArgs keeps command-line arguments that are magnet links or existing
// .torrent files, e.g. from "Open with" or a file dropped on the .exe.
func openableArgs(args []string) []string {
	out := []string{}
	for _, arg := range args {
		arg = strings.TrimSpace(arg)
		switch {
		case strings.HasPrefix(strings.ToLower(arg), "magnet:?"):
			if len(arg) <= safety.MaxMagnetLength {
				out = append(out, arg)
			}
		case strings.EqualFold(filepath.Ext(arg), ".torrent"):
			abs, err := filepath.Abs(arg)
			if err != nil {
				continue
			}
			if st, err := os.Stat(abs); err == nil && st.Mode().IsRegular() {
				out = append(out, abs)
			}
		}
		if len(out) == maxOpenArgs {
			break
		}
	}
	return out
}

func (a *App) eng() (*engine.Engine, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.engine == nil {
		return nil, apperr.New(apperr.EngineOffline)
	}
	return a.engine, nil
}

func validIDs(ids []string) error {
	if len(ids) > maxIDs {
		return apperr.New(apperr.InvalidInput)
	}
	for _, id := range ids {
		if !safety.IsTorrentID(id) {
			return apperr.New(apperr.InvalidInput)
		}
	}
	return nil
}

// Bootstrap is everything the UI needs when it starts.
type Bootstrap struct {
	AppName       string         `json:"appName"`
	Version       string         `json:"version"`
	Settings      SettingsView   `json:"settings"`
	FirstRun      bool           `json:"firstRun"`
	Portable      bool           `json:"portable"`
	DataDir       string         `json:"dataDir"`
	Links         []appinfo.Link `json:"links"`
	EngineError   string         `json:"engineError"`
	SettingsError string         `json:"settingsError"`
	Args          []string       `json:"args"`
	SecretStorage bool           `json:"secretStorage"`
	Platform      string         `json:"platform"`
}

// SettingsView is the settings as the UI sees them. A saved proxy password is
// never sent back to the UI; it only learns whether one exists.
type SettingsView struct {
	config.Settings
	ProxyPasswordSet   bool `json:"proxyPasswordSet"`
	ClearProxyPassword bool `json:"clearProxyPassword"`
}

func (a *App) settingsViewLocked() SettingsView {
	v := SettingsView{Settings: a.settings, ProxyPasswordSet: a.settings.Proxy.Password != ""}
	v.Proxy.Password = ""
	return v
}

// GetBootstrap returns the initial UI state. Command-line arguments are
// handed over only once.
func (a *App) GetBootstrap() Bootstrap {
	a.mu.Lock()
	defer a.mu.Unlock()
	b := Bootstrap{
		AppName:       appinfo.Name,
		Version:       appinfo.Version,
		Settings:      a.settingsViewLocked(),
		FirstRun:      a.firstRun,
		Portable:      a.loc.Portable,
		DataDir:       a.loc.Dir,
		Links:         appinfo.DonationLinks,
		Args:          a.args,
		SecretStorage: secret.Supported,
		Platform:      goruntime.GOOS,
	}
	a.args = []string{}
	if a.engineErr != nil {
		b.EngineError = a.engineErr.Error()
	}
	if a.loadErr != nil {
		b.SettingsError = apperr.Wrap(apperr.IO, a.loadErr).Error()
	}
	return b
}

// GetState returns the current transfers without waiting for the next tick.
func (a *App) GetState() (engine.State, error) {
	eng, err := a.eng()
	if err != nil {
		return engine.State{}, err
	}
	return eng.Snapshot(), nil
}

// SaveSettings validates, persists and applies new settings. An empty proxy
// password keeps the saved one unless ClearProxyPassword is set.
func (a *App) SaveSettings(in SettingsView) (SettingsView, error) {
	s := in.Settings
	a.mu.Lock()
	defer a.mu.Unlock()
	switch {
	case in.ClearProxyPassword:
		s.Proxy.Password = ""
	case s.Proxy.Password == "":
		s.Proxy.Password = a.settings.Proxy.Password
	}
	s.Normalize()
	if err := s.Validate(); err != nil {
		return SettingsView{}, err
	}
	if err := a.store.Save(s); err != nil {
		return SettingsView{}, apperr.Wrap(apperr.IO, err)
	}
	a.settings = s
	a.firstRun = false
	if a.engine != nil {
		a.engine.ApplySettings(s)
	}
	return a.settingsViewLocked(), nil
}

// ListInterfaces returns network adapters for the VPN binding picker.
func (a *App) ListInterfaces() ([]netguard.Interface, error) {
	return netguard.ListInterfaces()
}

// PickTorrentFiles shows a native open dialog for .torrent files.
func (a *App) PickTorrentFiles(title string) ([]string, error) {
	return runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   safety.CleanText(title, maxDialogTitle),
		Filters: []runtime.FileFilter{{DisplayName: "Torrent (*.torrent)", Pattern: "*.torrent"}},
	})
}

// PickFolder shows a native folder dialog.
func (a *App) PickFolder(title, current string) (string, error) {
	opts := runtime.OpenDialogOptions{Title: safety.CleanText(title, maxDialogTitle), CanCreateDirectories: true}
	if filepath.IsAbs(current) {
		opts.DefaultDirectory = current
	}
	return runtime.OpenDirectoryDialog(a.ctx, opts)
}

// PickBlocklist shows a native open dialog for blocklist files.
func (a *App) PickBlocklist(title string) (string, error) {
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   safety.CleanText(title, maxDialogTitle),
		Filters: []runtime.FileFilter{{DisplayName: "Blocklist (*.p2p, *.txt, *.dat, *.gz)", Pattern: "*.p2p;*.txt;*.dat;*.gz"}},
	})
}

// FreeSpace returns the free bytes for a folder, or -1 when unknown.
func (a *App) FreeSpace(path string) int64 {
	if !filepath.IsAbs(path) {
		return -1
	}
	n, err := fsutil.FreeSpace(path)
	if err != nil {
		return -1
	}
	return n
}

// ClipboardMagnet returns the clipboard text only when it is a valid magnet
// link, so arbitrary clipboard contents never reach the UI.
func (a *App) ClipboardMagnet() string {
	text, err := runtime.ClipboardGetText(a.ctx)
	if err != nil {
		return ""
	}
	text = strings.TrimSpace(text)
	if _, err := safety.ParseMagnet(text); err != nil {
		return ""
	}
	return text
}

// PreviewTorrent reads a .torrent file for the add dialog.
func (a *App) PreviewTorrent(path string) (engine.Preview, error) {
	eng, err := a.eng()
	if err != nil {
		return engine.Preview{}, err
	}
	return eng.PreviewTorrentFile(path)
}

// AddTorrentFile adds a .torrent file.
func (a *App) AddTorrentFile(path string, opts engine.AddOptions) (string, error) {
	eng, err := a.eng()
	if err != nil {
		return "", err
	}
	return eng.AddTorrentFile(path, opts)
}

// PreviewMagnet validates a magnet link for the add dialog.
func (a *App) PreviewMagnet(uri string) (engine.MagnetPreview, error) {
	eng, err := a.eng()
	if err != nil {
		return engine.MagnetPreview{}, err
	}
	return eng.PreviewMagnet(uri)
}

// AddMagnet adds a magnet link.
func (a *App) AddMagnet(uri string, opts engine.AddOptions) (string, error) {
	eng, err := a.eng()
	if err != nil {
		return "", err
	}
	return eng.AddMagnet(uri, opts)
}

// Pause pauses transfers.
func (a *App) Pause(ids []string) error {
	if err := validIDs(ids); err != nil {
		return err
	}
	eng, err := a.eng()
	if err != nil {
		return err
	}
	eng.Pause(ids)
	return nil
}

// Resume resumes transfers.
func (a *App) Resume(ids []string) error {
	if err := validIDs(ids); err != nil {
		return err
	}
	eng, err := a.eng()
	if err != nil {
		return err
	}
	return eng.Resume(ids)
}

// Remove removes transfers, optionally deleting their files.
func (a *App) Remove(ids []string, deleteFiles bool) error {
	if err := validIDs(ids); err != nil {
		return err
	}
	eng, err := a.eng()
	if err != nil {
		return err
	}
	return eng.Remove(ids, deleteFiles)
}

// Recheck verifies a transfer's data.
func (a *App) Recheck(id string) error {
	if err := validIDs([]string{id}); err != nil {
		return err
	}
	eng, err := a.eng()
	if err != nil {
		return err
	}
	return eng.Recheck(id)
}

// SetFilePriority changes file priorities (0 skip, 1 normal, 2 high).
func (a *App) SetFilePriority(id string, indices []int, priority int) error {
	if err := validIDs([]string{id}); err != nil {
		return err
	}
	eng, err := a.eng()
	if err != nil {
		return err
	}
	return eng.SetFilePriority(id, indices, priority)
}

// GetDetails returns one section of the details panel.
func (a *App) GetDetails(id, section string) (engine.Details, error) {
	if err := validIDs([]string{id}); err != nil {
		return engine.Details{}, err
	}
	eng, err := a.eng()
	if err != nil {
		return engine.Details{}, err
	}
	return eng.Details(id, section)
}

// OpenFolder shows a transfer's files in the file manager.
func (a *App) OpenFolder(id string) error {
	if err := validIDs([]string{id}); err != nil {
		return err
	}
	eng, err := a.eng()
	if err != nil {
		return err
	}
	return eng.OpenFolder(id)
}

// GetMagnetLink returns a shareable magnet link.
func (a *App) GetMagnetLink(id string) (string, error) {
	if err := validIDs([]string{id}); err != nil {
		return "", err
	}
	eng, err := a.eng()
	if err != nil {
		return "", err
	}
	return eng.MagnetLink(id)
}

// Reconnect retries the protected connection after a failure.
func (a *App) Reconnect() error {
	eng, err := a.eng()
	if err != nil {
		return err
	}
	return eng.Reconnect()
}

// OpenLink opens an allowlisted external page by ID. Arbitrary URLs are refused.
func (a *App) OpenLink(id string) error {
	l, ok := appinfo.LinkByID(id)
	if !ok {
		return apperr.New(apperr.LinkNotAllowed)
	}
	runtime.BrowserOpenURL(a.ctx, l.URL)
	return nil
}

// OpenDataFolder shows the portable data folder.
func (a *App) OpenDataFolder() error {
	if err := engine.Reveal(a.loc.Dir, true); err != nil {
		return apperr.Wrap(apperr.IO, err)
	}
	return nil
}
