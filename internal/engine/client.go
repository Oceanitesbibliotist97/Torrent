package engine

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	g "github.com/anacrolix/generics"
	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/mse"
	"github.com/anacrolix/torrent/storage"
	"golang.org/x/net/proxy"

	"github.com/ClearNetSky/Torrent/internal/appinfo"
	"github.com/ClearNetSky/Torrent/internal/apperr"
	"github.com/ClearNetSky/Torrent/internal/config"
	"github.com/ClearNetSky/Torrent/internal/netguard"
)

// The app keeps no logs, so nothing about transfers is ever written to disk.
var discardLogger = slog.New(slog.DiscardHandler)

// netPlan captures how clients reach the network in the chosen mode.
type netPlan struct {
	mode      string
	binding   netguard.Binding
	proxy     proxy.ContextDialer
	proxyAddr string
	resolver  *net.Resolver // nil means the system resolver
}

func buildNetPlan(s config.Settings) (netPlan, error) {
	switch s.NetworkMode {
	case config.NetworkVPN:
		b, err := netguard.ResolveBinding(s.VPNInterface, s.EnableIPv6)
		if err != nil {
			return netPlan{mode: s.NetworkMode}, apperr.Detail(apperr.VPNInterfaceDown, s.VPNInterface)
		}
		return netPlan{mode: s.NetworkMode, binding: b, resolver: b.Resolver()}, nil
	case config.NetworkProxy:
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		d, err := netguard.NewProxyDialer(ctx, netguard.Proxy{
			Host:     s.Proxy.Host,
			Port:     s.Proxy.Port,
			Username: s.Proxy.Username,
			Password: s.Proxy.Password,
		})
		if err != nil {
			return netPlan{mode: s.NetworkMode}, apperr.Wrap(apperr.Network, err)
		}
		return netPlan{
			mode:      s.NetworkMode,
			proxy:     d,
			proxyAddr: net.JoinHostPort(s.Proxy.Host, strconv.Itoa(s.Proxy.Port)),
			resolver:  netguard.BlockedResolver(),
		}, nil
	}
	return netPlan{mode: config.NetworkDirect}, nil
}

// clientConfig translates settings into a hardened anacrolix configuration.
// A private client (BEP 27) never uses DHT or PEX.
func (e *Engine) clientConfig(s config.Settings, plan netPlan, private bool) *torrent.ClientConfig {
	cfg := torrent.NewDefaultClientConfig()
	cfg.Slogger = discardLogger
	cfg.DefaultStorage = e.storageLocked(s.DownloadDir)
	cfg.DataDir = s.DownloadDir
	cfg.Seed = true // seeding is stopped by dropping the torrent, see checkCompletionLocked
	cfg.DownloadRateLimiter = e.dlLimit
	cfg.UploadRateLimiter = e.ulLimit
	cfg.EstablishedConnsPerTorrent = s.MaxPeersPerTorrent
	cfg.HalfOpenConnsPerTorrent = max(5, s.MaxPeersPerTorrent/2)
	cfg.IPBlocklist = e.blocklist
	cfg.ListenPort = s.ListenPort
	if private {
		cfg.ListenPort = 0
	}

	// WebRTC (WebTorrent) negotiates through STUN and can expose the real IP.
	cfg.DisableWebtorrent = true
	cfg.DisableWebseeds = !s.EnableWebSeeds
	cfg.DisableIPv6 = !s.EnableIPv6
	cfg.DisableUTP = !s.EnableUTP
	cfg.NoDHT = !s.EnableDHT || private
	cfg.DisablePEX = !s.EnablePEX || private
	cfg.NoDefaultPortForwarding = !s.EnableUPnP || private || plan.mode != config.NetworkDirect

	require := s.Encryption == config.EncryptionRequire
	cfg.HeaderObfuscationPolicy = torrent.HeaderObfuscationPolicy{Preferred: true, RequirePreferred: require}
	if require {
		cfg.CryptoProvides = mse.CryptoMethodRC4
	}
	// After the obfuscated handshake prefer full-stream RC4 over plaintext, so
	// traffic shaping cannot recognise the protocol mid-stream.
	cfg.CryptoSelector = func(provided mse.CryptoMethod) mse.CryptoMethod {
		if provided&mse.CryptoMethodRC4 != 0 {
			return mse.CryptoMethodRC4
		}
		if require {
			return 0 // rejected by the handshake
		}
		return mse.CryptoMethodPlaintext
	}

	if s.AnonymousMode {
		// Fully random peer ID; no client name or version in the handshake or HTTP.
		cfg.Bep20 = ""
		cfg.ExtendedHandshakeClientVersion = ""
		cfg.HTTPUserAgent = ""
		cfg.HttpRequestDirector = func(r *http.Request) error {
			// An empty header stops both the library and net/http from adding their own.
			r.Header.Set("User-Agent", "")
			return nil
		}
	} else {
		cfg.Bep20 = "-TO0100-"
		cfg.ExtendedHandshakeClientVersion = appinfo.Name + " " + appinfo.Version
		cfg.HTTPUserAgent = appinfo.Name + "/" + appinfo.Version
	}
	cfg.UpnpID = appinfo.Name

	if e.listenHost != "" && plan.mode == config.NetworkDirect {
		host := e.listenHost
		cfg.ListenHost = func(string) string { return host }
		cfg.DisableIPv6 = true
	}

	switch plan.mode {
	case config.NetworkVPN:
		b := plan.binding
		cfg.DisableIPv4 = !b.IPv4.IsValid()
		cfg.DisableIPv6 = cfg.DisableIPv6 || !b.IPv6.IsValid()
		cfg.ListenHost = func(network string) string {
			if strings.Contains(network, "6") {
				return b.IPv6.String()
			}
			return b.IPv4.String()
		}
		// The library's TCP dialers are not bound to the listen address, so
		// bound dialers are added in attach instead.
		cfg.DialForPeerConns = false
		cfg.TrackerDialContext = b.DialContext
		cfg.HTTPDialContext = b.DialContext
		cfg.TrackerListenPacket = b.ListenPacket
	case config.NetworkProxy:
		// No UDP at all: SOCKS5 UDP relay is rarely available and would leak.
		cfg.DisableUTP = true
		cfg.NoDHT = true
		cfg.DisableIPv6 = true
		// Listen on loopback only so trackers get a valid port; nothing is accepted.
		cfg.ListenHost = func(string) string { return "127.0.0.1" }
		cfg.AcceptPeerConnections = false
		cfg.DialForPeerConns = false
		cfg.TrackerDialContext = plan.proxy.DialContext
		cfg.HTTPDialContext = plan.proxy.DialContext
		cfg.TrackerListenPacket = netguard.BlockedListenPacket
	}
	return cfg
}

// attach adds the leak-safe dialers for the plan to a new client.
func (plan netPlan) attach(cl *torrent.Client) {
	switch plan.mode {
	case config.NetworkVPN:
		if plan.binding.IPv4.IsValid() {
			cl.AddDialer(torrent.NetworkDialer{Network: "tcp4", Dialer: plan.binding})
		}
		if plan.binding.IPv6.IsValid() {
			cl.AddDialer(torrent.NetworkDialer{Network: "tcp6", Dialer: plan.binding})
		}
		// uTP sockets are bound to the VPN address, so dialing from them is safe.
		for _, l := range cl.Listeners() {
			if d, ok := l.(torrent.Dialer); ok && strings.HasPrefix(d.DialerNetwork(), "udp") {
				cl.AddDialer(d)
			}
		}
	case config.NetworkProxy:
		cl.AddDialer(torrent.NetworkDialer{Network: "tcp", Dialer: plan.proxy})
	}
}

// newClientLocked starts a client, falling back to a random port when the
// configured one is taken by another program.
func (e *Engine) newClientLocked(plan netPlan, private bool) (*torrent.Client, error) {
	s := e.settings
	cl, err := torrent.NewClient(e.clientConfig(s, plan, private))
	if err != nil && s.ListenPort != 0 {
		s.ListenPort = 0
		cl, err = torrent.NewClient(e.clientConfig(s, plan, private))
	}
	if err != nil {
		return nil, apperr.Wrap(apperr.Network, err)
	}
	plan.attach(cl)
	return cl, nil
}

// storageLocked returns the file storage for a download directory. Files are
// written in place (no .part files) so piece-level progress survives pausing.
func (e *Engine) storageLocked(dir string) storage.ClientImpl {
	if st, ok := e.storages[dir]; ok {
		return st
	}
	st := storage.NewFileOpts(storage.NewFileClientOpts{
		ClientBaseDir:   dir,
		PieceCompletion: e.completion,
		UsePartFiles:    g.Some(false),
		Logger:          discardLogger,
	})
	e.storages[dir] = st
	return st
}
