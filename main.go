package main

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"github.com/ClearNetSky/Torrent/internal/appinfo"
	"github.com/ClearNetSky/Torrent/internal/config"
	"github.com/ClearNetSky/Torrent/internal/paths"
)

//go:embed all:frontend/dist
var assets embed.FS

// silentLogger discards framework logs: the app writes no logs anywhere.
type silentLogger struct{}

func (silentLogger) Print(string)   {}
func (silentLogger) Trace(string)   {}
func (silentLogger) Debug(string)   {}
func (silentLogger) Info(string)    {}
func (silentLogger) Warning(string) {}
func (silentLogger) Error(string)   {}
func (silentLogger) Fatal(string)   {}

func main() {
	loc, err := paths.Resolve()
	if err != nil {
		showFatal(appinfo.Name, "Cannot create the data folder: "+err.Error())
		os.Exit(1)
	}
	store := config.NewStore(loc.Dir)
	settings, firstRun, loadErr := store.Load()
	app := NewApp(loc, store, settings, firstRun, loadErr, os.Args[1:])

	dist, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		showFatal(appinfo.Name, err.Error())
		os.Exit(1)
	}
	// One instance per data folder: separate portable copies can run side by side.
	dirHash := sha256.Sum256([]byte(loc.Dir))

	err = wails.Run(&options.App{
		Title:            appinfo.Name,
		Width:            1280,
		Height:           820,
		MinWidth:         940,
		MinHeight:        600,
		BackgroundColour: options.NewRGB(14, 17, 22),
		AssetServer:      &assetserver.Options{Assets: dist},
		OnStartup:        app.startup,
		OnBeforeClose:    app.beforeClose,
		OnShutdown:       app.shutdown,
		Bind:             []any{app},
		Logger:           silentLogger{},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               appinfo.ID + "." + hex.EncodeToString(dirHash[:6]),
			OnSecondInstanceLaunch: app.onSecondInstance,
		},
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     true,
			DisableWebViewDrop: true,
		},
		EnableDefaultContextMenu:         false,
		EnableFraudulentWebsiteDetection: false,
		Windows: &windows.Options{
			// Keep WebView2 data inside the portable folder instead of %APPDATA%.
			WebviewUserDataPath: filepath.Join(loc.Dir, "webview"),
			Theme:               windows.SystemDefault,
			DisablePinchZoom:    true,
			// Only load system DLLs from System32, never from the .exe folder
			// (a portable app often sits in Downloads next to untrusted files).
			DLLSearchPaths: windows.DLLSearchSystem32,
		},
	})
	if err != nil {
		showFatal(appinfo.Name, err.Error())
		os.Exit(1)
	}
}
