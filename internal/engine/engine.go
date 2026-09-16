// Package engine runs the BitTorrent clients and exposes a UI-friendly model
// of the transfers. All network access follows the privacy settings: direct,
// bound to a VPN adapter, or through a SOCKS5 proxy.
package engine

import (
	"cmp"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/anacrolix/dht/v2"
	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/iplist"
	"github.com/anacrolix/torrent/storage"
	"golang.org/x/time/rate"

	"github.com/ClearNetSky/Torrent/internal/apperr"
	"github.com/ClearNetSky/Torrent/internal/config"
	"github.com/ClearNetSky/Torrent/internal/fsutil"
	"github.com/ClearNetSky/Torrent/internal/netguard"
)

// Event names emitted to the UI.
const (
	EventState  = "state"
	EventNotice = "notice"
)

// Emitter delivers events to the UI.
type Emitter func(name string, data any)

// Notice is a short message for the UI to show as a toast.
type Notice struct {
	Kind   string `json:"kind"`   // info, success, warning, error
	Key    string `json:"key"`    // translation key
	Detail string `json:"detail"` // e.g. a torrent name; displayed as plain text
}

// NetworkView describes the effective network protection.
type NetworkView struct {
	Mode           string `json:"mode"`
	Online         bool   `json:"online"`
	Error          string `json:"error"`
	KillSwitch     bool   `json:"killSwitch"` // transfers were stopped because the VPN dropped
	Interface      string `json:"interface"`
	BoundIPv4      string `json:"boundIPv4"`
	BoundIPv6      string `json:"boundIPv6"`
	Proxy          string `json:"proxy"`
	ProxyReachable bool   `json:"proxyReachable"`
	ListenPort     int    `json:"listenPort"`
	DHT            bool   `json:"dht"`
	PEX            bool   `json:"pex"`
	UTP            bool   `json:"utp"`
	UPnP           bool   `json:"upnp"`
	Incoming       bool   `json:"incoming"`
	BlocklistRules int    `json:"blocklistRules"`
}

// GlobalView aggregates the whole app.
type GlobalView struct {
	DownRate  int64       `json:"downRate"`
	UpRate    int64       `json:"upRate"`
	DHTNodes  int         `json:"dhtNodes"`
	FreeSpace int64       `json:"freeSpace"`
	Network   NetworkView `json:"network"`
}

// State is the snapshot sent to the UI every second.
type State struct {
	Torrents []TorrentView `json:"torrents"`
	Global   GlobalView    `json:"global"`
}

// Engine owns the torrent clients and the list of transfers.
type Engine struct {
	mu       sync.Mutex
	dataDir  string
	settings config.Settings
	emit     Emitter

	completion completionStore
	storages   map[string]storage.ClientImplCloser
	dlLimit    *rate.Limiter
	ulLimit    *rate.Limiter

	blocklist      iplist.Ranger
	blocklistRules int
	blocklistKey   string

	plan       netPlan
	client     *torrent.Client // public torrents
	privClient *torrent.Client // private torrents (BEP 27), started on demand
	netErr     error
	tripped    bool
	proxyOK    atomic.Bool

	items   map[string]*item
	notices []Notice

	dirty     bool
	lastSave  time.Time
	freeSpace int64
	freeAt    time.Time

	stop   chan struct{}
	done   chan struct{}
	closed bool

	listenHost string // tests only: restricts direct mode to one local address
}

// New creates an engine and restores the saved session. Call Start to go online.
func New(dataDir string, s config.Settings, emit Emitter) (*Engine, error) {
	e := &Engine{
		dataDir:  dataDir,
		settings: s,
		emit:     emit,
		storages: make(map[string]storage.ClientImplCloser),
		items:    make(map[string]*item),
		dlLimit:  rate.NewLimiter(rate.Inf, 0),
		ulLimit:  rate.NewLimiter(rate.Inf, 0),
	}
	if s.RememberTorrents {
		c, err := openBoltCompletion(filepath.Join(dataDir, resumeDBFile))
		if err != nil {
			return nil, apperr.Wrap(apperr.IO, err)
		}
		e.completion = c
		if err := e.loadSessionLocked(); err != nil {
			e.noticeLocked("warning", "notice.sessionLoadFailed", "")
		}
	} else {
		e.wipeHistoryFiles(true)
		e.completion = newMemoryCompletion()
	}
	e.applyLimitsLocked()
	return e, nil
}

// Start brings the clients online and begins emitting state once a second.
func (e *Engine) Start() {
	e.mu.Lock()
	e.goOnlineLocked()
	e.mu.Unlock()
	e.stop = make(chan struct{})
	e.done = make(chan struct{})
	go e.loop()
}

// Close stops every transfer, saves the session and releases all resources.
// When history is disabled, all traces of the session are deleted.
func (e *Engine) Close() {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return
	}
	e.closed = true
	e.mu.Unlock()
	if e.stop != nil {
		close(e.stop)
		<-e.done
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.goOfflineLocked()
	netguard.InstallResolver(nil)
	e.saveSessionLocked()
	e.completion.Close()
	if !e.settings.RememberTorrents {
		e.wipeHistoryFiles(true)
	}
}

func (e *Engine) loop() {
	defer close(e.done)
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for n := 1; ; n++ {
		select {
		case <-e.stop:
			return
		case now := <-tick.C:
			e.mu.Lock()
			if n%2 == 0 {
				e.watchNetworkLocked()
			}
			if n%15 == 0 {
				e.probeProxyLocked()
			}
			st := e.snapshotLocked(now)
			if e.dirty && now.Sub(e.lastSave) >= 10*time.Second {
				e.saveSessionLocked()
			}
			notices := e.notices
			e.notices = nil
			e.mu.Unlock()
			if e.emit != nil {
				e.emit(EventState, st)
				for _, nt := range notices {
					e.emit(EventNotice, nt)
				}
			}
		}
	}
}

// goOnlineLocked (re)starts the clients for the current settings. When the
// VPN or proxy cannot be set up, nothing starts: there is no fallback to a
// direct connection.
func (e *Engine) goOnlineLocked() {
	e.goOfflineLocked()
	e.tripped = false
	e.netErr = nil
	e.loadBlocklistLocked()
	plan, err := buildNetPlan(e.settings)
	e.plan = plan
	if err != nil {
		e.netErr = err
		return
	}
	netguard.InstallResolver(plan.resolver)
	e.proxyOK.Store(true)
	cl, err := e.newClientLocked(plan, false)
	if err != nil {
		e.netErr = err
		return
	}
	e.client = cl
	for _, it := range e.sortedItemsLocked() {
		if it.wantsActive() {
			if err := e.activateLocked(it); err != nil {
				it.err = err.Error()
			}
		}
	}
}

func (e *Engine) goOfflineLocked() {
	for _, it := range e.items {
		e.deactivateLocked(it)
	}
	if e.client != nil {
		e.client.Close()
		e.client = nil
	}
	if e.privClient != nil {
		e.privClient.Close()
		e.privClient = nil
	}
}

// watchNetworkLocked is the VPN kill switch: the moment the adapter loses its
// address every client is closed, and they restart when it returns.
func (e *Engine) watchNetworkLocked() {
	if e.settings.NetworkMode != config.NetworkVPN || e.closed {
		return
	}
	switch {
	case e.client != nil && !e.plan.binding.Alive():
		e.goOfflineLocked()
		e.netErr = apperr.Detail(apperr.VPNInterfaceDown, e.settings.VPNInterface)
		e.tripped = true
		e.noticeLocked("error", "notice.killSwitch", e.settings.VPNInterface)
	case e.client == nil && (!e.tripped || e.settings.AutoReconnect):
		if _, err := netguard.ResolveBinding(e.settings.VPNInterface, e.settings.EnableIPv6); err != nil {
			return
		}
		wasTripped := e.tripped
		e.goOnlineLocked()
		if e.client != nil && wasTripped {
			e.noticeLocked("success", "notice.vpnRestored", e.settings.VPNInterface)
		}
	}
}

func (e *Engine) probeProxyLocked() {
	if e.plan.mode != config.NetworkProxy || e.client == nil {
		return
	}
	s := e.settings.Proxy
	p := netguard.Proxy{Host: s.Host, Port: s.Port}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		e.proxyOK.Store(netguard.ProbeProxy(ctx, p) == nil)
	}()
}

// Reconnect retries going online after a failure or a kill switch stop.
func (e *Engine) Reconnect() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.goOnlineLocked()
	return e.netErr
}

func (e *Engine) loadBlocklistLocked() {
	path := e.settings.BlocklistPath
	if path == "" {
		e.blocklist, e.blocklistRules, e.blocklistKey = nil, 0, ""
		return
	}
	st, err := os.Stat(path)
	if err != nil {
		e.blocklist, e.blocklistRules, e.blocklistKey = nil, 0, ""
		e.noticeLocked("warning", "notice.blocklistFailed", "")
		return
	}
	key := path + "|" + st.ModTime().String()
	if key == e.blocklistKey {
		return
	}
	list, err := loadBlocklist(path)
	if err != nil {
		e.blocklist, e.blocklistRules, e.blocklistKey = nil, 0, ""
		e.noticeLocked("warning", "notice.blocklistFailed", "")
		return
	}
	e.blocklist, e.blocklistRules, e.blocklistKey = list, list.NumRanges(), key
}

func (e *Engine) applyLimitsLocked() {
	set := func(l *rate.Limiter, kib int) {
		if kib <= 0 {
			l.SetLimit(rate.Inf)
		} else {
			l.SetLimit(rate.Limit(kib * 1024))
		}
		// Large enough for any chunk the library reads or writes at once.
		l.SetBurst(1 << 20)
	}
	set(e.dlLimit, e.settings.DownloadLimitKiB)
	set(e.ulLimit, e.settings.UploadLimitKiB)
}

// Settings returns the settings in effect.
func (e *Engine) Settings() config.Settings {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.settings
}

// ApplySettings switches to new settings. Clients restart only when a
// network-relevant option changed.
func (e *Engine) ApplySettings(s config.Settings) {
	e.mu.Lock()
	defer e.mu.Unlock()
	old := e.settings
	e.settings = s
	e.applyLimitsLocked()
	if old.RememberTorrents && !s.RememberTorrents {
		// Resume data stays open until exit, when it is deleted too.
		e.wipeHistoryFiles(false)
	}
	if !old.RememberTorrents && s.RememberTorrents {
		for _, it := range e.items {
			e.writeMetainfoLocked(it)
		}
	}
	if networkChanged(old, s) {
		e.goOnlineLocked()
	} else if old.MaxPeersPerTorrent != s.MaxPeersPerTorrent {
		for _, it := range e.items {
			if it.t != nil {
				it.t.SetMaxEstablishedConns(s.MaxPeersPerTorrent)
			}
		}
	}
	e.freeAt = time.Time{}
	e.dirty = true
}

func networkChanged(a, b config.Settings) bool {
	return a.ListenPort != b.ListenPort || a.EnableDHT != b.EnableDHT || a.EnablePEX != b.EnablePEX ||
		a.EnableUTP != b.EnableUTP || a.EnableIPv6 != b.EnableIPv6 || a.EnableUPnP != b.EnableUPnP ||
		a.EnableWebSeeds != b.EnableWebSeeds || a.Encryption != b.Encryption || a.AnonymousMode != b.AnonymousMode ||
		a.NetworkMode != b.NetworkMode || a.VPNInterface != b.VPNInterface || a.Proxy != b.Proxy ||
		a.BlocklistPath != b.BlocklistPath
}

func (e *Engine) noticeLocked(kind, key, detail string) {
	if len(e.notices) < 50 {
		e.notices = append(e.notices, Notice{Kind: kind, Key: key, Detail: detail})
	}
}

func (e *Engine) sortedItemsLocked() []*item {
	out := make([]*item, 0, len(e.items))
	for _, it := range e.items {
		out = append(out, it)
	}
	slices.SortFunc(out, func(a, b *item) int {
		if c := cmp.Compare(a.AddedAt, b.AddedAt); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	return out
}

// Snapshot returns the current state immediately.
func (e *Engine) Snapshot() State {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.snapshotLocked(time.Now())
}

func (e *Engine) snapshotLocked(now time.Time) State {
	st := State{Torrents: make([]TorrentView, 0, len(e.items))}
	for _, it := range e.sortedItemsLocked() {
		e.refreshItemLocked(it, now)
		v := it.view()
		st.Global.DownRate += v.DownRate
		st.Global.UpRate += v.UpRate
		st.Torrents = append(st.Torrents, v)
	}
	st.Global.DHTNodes = dhtNodes(e.client)
	if now.Sub(e.freeAt) > 10*time.Second {
		e.freeAt = now
		e.freeSpace = -1
		if n, err := fsutil.FreeSpace(e.settings.DownloadDir); err == nil {
			e.freeSpace = n
		}
	}
	st.Global.FreeSpace = e.freeSpace
	st.Global.Network = e.networkViewLocked()
	return st
}

func dhtNodes(cl *torrent.Client) (n int) {
	if cl == nil {
		return 0
	}
	for _, s := range cl.DhtServers() {
		if st, ok := s.Stats().(dht.ServerStats); ok {
			n += st.GoodNodes
		}
	}
	return n
}

func (e *Engine) networkViewLocked() NetworkView {
	s := e.settings
	proxyMode := s.NetworkMode == config.NetworkProxy
	v := NetworkView{
		Mode:           s.NetworkMode,
		Online:         e.client != nil,
		KillSwitch:     e.tripped,
		ProxyReachable: e.proxyOK.Load(),
		DHT:            s.EnableDHT && !proxyMode,
		PEX:            s.EnablePEX,
		UTP:            s.EnableUTP && !proxyMode,
		UPnP:           s.EnableUPnP && s.NetworkMode == config.NetworkDirect,
		Incoming:       !proxyMode,
		BlocklistRules: e.blocklistRules,
	}
	if e.netErr != nil {
		v.Error = e.netErr.Error()
	}
	switch s.NetworkMode {
	case config.NetworkVPN:
		v.Interface = s.VPNInterface
		if e.plan.binding.IPv4.IsValid() {
			v.BoundIPv4 = e.plan.binding.IPv4.String()
		}
		if e.plan.binding.IPv6.IsValid() {
			v.BoundIPv6 = e.plan.binding.IPv6.String()
		}
	case config.NetworkProxy:
		v.Proxy = e.plan.proxyAddr
	}
	if e.client != nil && !proxyMode {
		v.ListenPort = e.client.LocalPort()
	}
	return v
}

func (e *Engine) refreshItemLocked(it *item, now time.Time) {
	if msg := it.writeErr.Swap(nil); msg != nil {
		e.deactivateLocked(it)
		it.err = *msg
		e.noticeLocked("error", "notice.writeError", it.Name)
		return
	}
	if it.t != nil && !it.hasInfo {
		select {
		case <-it.t.GotInfo():
			e.onInfoLocked(it)
		default:
		}
	}
	t := it.t
	if t == nil {
		return
	}
	stats := t.Stats()
	read := stats.BytesReadUsefulData.Int64()
	written := stats.BytesWrittenData.Int64()
	if dt := now.Sub(it.lastSample).Seconds(); !it.lastSample.IsZero() && dt > 0 {
		it.downRate = smoothRate(it.downRate, float64(read-it.sessRead)/dt)
		it.upRate = smoothRate(it.upRate, float64(written-it.sessWritten)/dt)
	}
	it.sessRead, it.sessWritten, it.lastSample = read, written, now
	it.peers, it.seeds, it.knownPeers = stats.ActivePeers, stats.ConnectedSeeders, stats.TotalPeers
	if it.hasInfo {
		it.DoneBytes = bytesDone(it)
		e.checkCompletionLocked(it, now)
	}
}

func smoothRate(prev, sample float64) float64 {
	return prev*0.4 + max(0, sample)*0.6
}

// bytesDone counts completed bytes of the wanted, non-padding files.
func bytesDone(it *item) int64 {
	if it.allWanted() && !it.hasPadding() {
		return it.t.BytesCompleted()
	}
	var done int64
	for i, f := range it.t.Files() {
		if i < len(it.files) && !it.files[i].Padding && it.Priorities[i] != PrioritySkip {
			done += f.BytesCompleted()
		}
	}
	return done
}

func (e *Engine) checkCompletionLocked(it *item, now time.Time) {
	if !it.complete() {
		it.CompletedAt = 0
		return
	}
	if it.CompletedAt == 0 {
		it.CompletedAt = now.UnixMilli()
		e.dirty = true
		e.noticeLocked("success", "notice.completed", it.Name)
		if e.settings.MarkOfTheWeb {
			go markOfTheWeb(it.wantedPaths())
		}
	}
	switch {
	case !e.settings.SeedAfterComplete:
		it.Finished = true
		e.deactivateLocked(it)
	case e.settings.SeedRatioLimit > 0 && it.ratio() >= e.settings.SeedRatioLimit:
		it.Finished = true
		e.deactivateLocked(it)
		e.noticeLocked("info", "notice.seedingDone", it.Name)
	}
}

// onInfoLocked runs once metadata arrives for a magnet link.
func (e *Engine) onInfoLocked(it *item) {
	info := it.t.Info()
	if info == nil {
		return
	}
	mi := it.t.Metainfo()
	it.setInfo(mi.InfoBytes, info)
	e.writeMetainfoLocked(it)
	e.dirty = true
	if it.private && !it.onPrivate {
		// BEP 27: private torrents must not use DHT or PEX.
		e.deactivateLocked(it)
		if err := e.activateLocked(it); err != nil {
			it.err = err.Error()
		}
		return
	}
	e.applyPrioritiesLocked(it)
}
