package engine

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
	"golang.org/x/time/rate"

	"github.com/ClearNetSky/Torrent/internal/config"
)

func testEngineShell() *Engine {
	return &Engine{
		storages:   make(map[string]storage.ClientImplCloser),
		items:      make(map[string]*item),
		completion: newMemoryCompletion(),
		dlLimit:    rate.NewLimiter(rate.Inf, 0),
		ulLimit:    rate.NewLimiter(rate.Inf, 0),
	}
}

type fakeDialer struct{}

func (fakeDialer) DialContext(context.Context, string, string) (net.Conn, error) {
	return nil, net.ErrClosed
}

func TestProxyModeConfigHasNoLeakPaths(t *testing.T) {
	e := testEngineShell()
	s := config.Defaults()
	s.NetworkMode = config.NetworkProxy
	s.EnableUPnP = true
	cfg := e.clientConfig(s, netPlan{mode: config.NetworkProxy, proxy: fakeDialer{}}, false)

	if !cfg.NoDHT || !cfg.DisableUTP || !cfg.DisableIPv6 {
		t.Error("proxy mode must disable DHT, uTP and IPv6 (UDP cannot be proxied)")
	}
	if cfg.AcceptPeerConnections || cfg.DialForPeerConns {
		t.Error("proxy mode must not accept or dial peers outside the proxy")
	}
	if !cfg.NoDefaultPortForwarding || !cfg.DisableWebtorrent {
		t.Error("UPnP and WebRTC must be off in proxy mode")
	}
	if cfg.TrackerDialContext == nil || cfg.HTTPDialContext == nil {
		t.Fatal("tracker and HTTP traffic must use the proxy dialer")
	}
	if _, err := cfg.TrackerListenPacket("udp", ":0"); err == nil {
		t.Error("UDP tracker sockets must be refused in proxy mode")
	}
	if got := cfg.ListenHost("tcp4"); got != "127.0.0.1" {
		t.Errorf("proxy mode listens on %q, want loopback", got)
	}
}

func TestAnonymousModeHidesClientIdentity(t *testing.T) {
	e := testEngineShell()
	s := config.Defaults()
	cfg := e.clientConfig(s, netPlan{mode: config.NetworkDirect}, false)
	if cfg.Bep20 != "" || cfg.ExtendedHandshakeClientVersion != "" || cfg.HTTPUserAgent != "" {
		t.Errorf("client identity leaked: bep20=%q v=%q ua=%q", cfg.Bep20, cfg.ExtendedHandshakeClientVersion, cfg.HTTPUserAgent)
	}
	req, _ := http.NewRequest(http.MethodGet, "http://tracker.example.org/announce", nil)
	req.Header.Set("User-Agent", "anacrolix-torrent/v1")
	if err := cfg.HttpRequestDirector(req); err != nil {
		t.Fatal(err)
	}
	if req.Header.Get("User-Agent") != "" {
		t.Error("tracker requests must not carry a User-Agent in anonymous mode")
	}
	if cfg.NoDefaultPortForwarding != true {
		t.Error("UPnP must be off by default")
	}
}

func TestPrivateClientDisablesDHTAndPEX(t *testing.T) {
	e := testEngineShell()
	cfg := e.clientConfig(config.Defaults(), netPlan{mode: config.NetworkDirect}, true)
	if !cfg.NoDHT || !cfg.DisablePEX {
		t.Error("private torrents (BEP 27) must never use DHT or PEX")
	}
}

// TestEndToEndTransfer downloads a file from a real seeder over loopback, then
// checks pausing, resuming from saved progress, persistence and removal.
func TestEndToEndTransfer(t *testing.T) {
	if testing.Short() {
		t.Skip("end-to-end transfer")
	}
	root := t.TempDir()
	// The seeder is a plain library client whose leaked file handles can
	// outlive the test, so its data lives outside t.TempDir and is removed
	// best-effort.
	srcDir, mkErr := os.MkdirTemp("", "torrent-seed-*")
	if mkErr != nil {
		t.Fatal(mkErr)
	}
	t.Cleanup(func() {
		runtime.GC()
		time.Sleep(300 * time.Millisecond)
		os.RemoveAll(srcDir)
	})
	dlDir := filepath.Join(root, "downloads")
	dataDir := filepath.Join(root, "data")
	for _, d := range []string{srcDir, dlDir, dataDir} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}

	content := bytes.Repeat([]byte("privacy-first portable torrent client\n"), 60000)
	srcFile := filepath.Join(srcDir, "sample.bin")
	if err := os.WriteFile(srcFile, content, 0o600); err != nil {
		t.Fatal(err)
	}
	info := metainfo.Info{PieceLength: 256 << 10}
	if err := info.BuildFromFilePath(srcFile); err != nil {
		t.Fatal(err)
	}
	var mi metainfo.MetaInfo
	var err error
	if mi.InfoBytes, err = bencode.Marshal(info); err != nil {
		t.Fatal(err)
	}
	torrentPath := filepath.Join(root, "sample.torrent")
	f, err := os.Create(torrentPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := mi.Write(f); err != nil {
		t.Fatal(err)
	}
	f.Close()

	// Seeder bound to loopback only.
	scfg := torrent.NewDefaultClientConfig()
	scfg.Slogger = discardLogger
	scfg.Seed = true
	scfg.NoDHT = true
	scfg.DisableIPv6 = true
	scfg.DisableUTP = true
	scfg.NoDefaultPortForwarding = true
	scfg.ListenPort = 0
	scfg.ListenHost = func(string) string { return "127.0.0.1" }
	scfg.DefaultStorage = storage.NewFileOpts(storage.NewFileClientOpts{
		ClientBaseDir:   srcDir,
		PieceCompletion: storage.NewMapPieceCompletion(),
		Logger:          discardLogger,
	})
	seeder, err := torrent.NewClient(scfg)
	if err != nil {
		t.Fatal(err)
	}
	defer seeder.Close()
	st, err := seeder.AddTorrent(&mi)
	if err != nil {
		t.Fatal(err)
	}
	<-st.GotInfo()
	if err := st.VerifyData(); err != nil {
		t.Fatal(err)
	}
	seedAddr := (&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: seeder.LocalPort()}).String()

	s := config.Defaults()
	s.DownloadDir = dlDir
	s.EnableDHT = false
	s.EnableUTP = false
	s.EnableIPv6 = false
	s.Encryption = config.EncryptionPrefer
	s.SeedRatioLimit = 0
	e, err := New(dataDir, s, nil)
	if err != nil {
		t.Fatal(err)
	}
	e.listenHost = "127.0.0.1"
	e.Start()
	defer e.Close()

	id, err := e.AddTorrentFile(torrentPath, AddOptions{SavePath: dlDir, Start: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.AddTorrentFile(torrentPath, AddOptions{SavePath: dlDir, Start: true}); err == nil {
		t.Error("adding the same torrent twice must fail")
	}
	addPeer := func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		if tt := e.items[id].t; tt != nil {
			tt.AddPeers([]torrent.PeerInfo{{Addr: torrent.StringAddr(seedAddr), Source: torrent.PeerSourceDirect, Trusted: true}})
		}
	}
	addPeer()
	waitFor(t, 60*time.Second, "download to complete", func() bool {
		v := viewOf(e, id)
		return v.Progress == 1 && v.State == StateSeeding
	})

	got, err := os.ReadFile(filepath.Join(dlDir, "sample.bin"))
	if err != nil || !bytes.Equal(got, content) {
		t.Fatalf("downloaded data differs (err=%v)", err)
	}
	if runtime.GOOS == "windows" {
		waitFor(t, 5*time.Second, "Mark of the Web", func() bool {
			zone, err := os.ReadFile(filepath.Join(dlDir, "sample.bin") + ":Zone.Identifier")
			return err == nil && bytes.Contains(zone, []byte("ZoneId=3"))
		})
	}

	e.Pause([]string{id})
	if v := viewOf(e, id); v.State != StatePaused || v.Progress != 1 {
		t.Fatalf("after pause: state=%s progress=%v", v.State, v.Progress)
	}
	if err := e.Resume([]string{id}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 20*time.Second, "resume from saved progress", func() bool {
		v := viewOf(e, id)
		return v.State == StateSeeding && v.Progress == 1
	})

	link, err := e.MagnetLink(id)
	if err != nil || !bytes.HasPrefix([]byte(link), []byte("magnet:?xt=urn:btih:")) {
		t.Errorf("magnet link = %q, err = %v", link, err)
	}

	// The session survives a restart.
	e.Close()
	e2, err := New(dataDir, s, nil)
	if err != nil {
		t.Fatal(err)
	}
	e2.listenHost = "127.0.0.1"
	if v := viewOf(e2, id); v.Name != "sample.bin" || v.Done != int64(len(content)) {
		t.Errorf("restored view = %+v", v)
	}
	// The library can leave file handles for finalizers (see removeWithRetry);
	// collect them so the temporary directories can be deleted.
	t.Cleanup(func() {
		runtime.GC()
		time.Sleep(300 * time.Millisecond) // finalizers run after the collection returns
	})
	if err := e2.Remove([]string{id}, true); err != nil {
		t.Fatal(err)
	}
	e2.Close()
	if _, err := os.Stat(filepath.Join(dlDir, "sample.bin")); !os.IsNotExist(err) {
		t.Errorf("file still present after remove with delete: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, torrentsDir, id+".torrent")); !os.IsNotExist(err) {
		t.Error("stored metainfo must be deleted with the transfer")
	}
}

func viewOf(e *Engine, id string) TorrentView {
	for _, v := range e.Snapshot().Torrents {
		if v.ID == id {
			return v
		}
	}
	return TorrentView{}
}

func waitFor(t *testing.T, timeout time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}
