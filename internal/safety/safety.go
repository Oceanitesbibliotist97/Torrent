// Package safety contains defensive checks for untrusted torrent input:
// magnet links, tracker and web seed URLs, peer addresses, file names and paths.
package safety

import (
	"net/netip"
	"net/url"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/anacrolix/torrent/metainfo"

	"github.com/ClearNetSky/Torrent/internal/apperr"
)

const (
	// MaxMagnetLength bounds magnet URIs pasted by the user.
	MaxMagnetLength = 16 << 10
	// MaxTorrentFileSize bounds .torrent files read from disk.
	MaxTorrentFileSize = 32 << 20
	// MaxTrackers bounds the trackers accepted per torrent.
	MaxTrackers = 150
	// MaxPeerAddrs bounds peer addresses taken from a magnet link (BEP 9 x.pe).
	MaxPeerAddrs = 50
	// MaxWebSeeds bounds web seeds accepted per torrent (BEP 19).
	MaxWebSeeds = 20
)

// ParseMagnet validates a magnet URI.
func ParseMagnet(raw string) (metainfo.MagnetV2, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > MaxMagnetLength || !strings.HasPrefix(strings.ToLower(raw), "magnet:?") {
		return metainfo.MagnetV2{}, apperr.New(apperr.InvalidMagnet)
	}
	m, err := metainfo.ParseMagnetV2Uri(raw)
	if err != nil || (!m.InfoHash.Ok && !m.V2InfoHash.Ok) {
		return metainfo.MagnetV2{}, apperr.New(apperr.InvalidMagnet)
	}
	return m, nil
}

// FilterTrackers keeps tracker URLs that are well-formed and safe to contact,
// canonicalizes them and removes duplicates. allowUDP is false in proxy mode,
// where UDP would bypass the proxy. WebSocket (WebRTC) trackers are always
// removed because WebRTC can reveal the real IP address.
func FilterTrackers(tiers [][]string, allowUDP bool) (kept [][]string, dropped []string) {
	seen := make(map[string]struct{})
	total := 0
	for _, tier := range tiers {
		var out []string
		for _, raw := range tier {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			canonical, ok := canonicalURL(raw, func(scheme string) bool {
				switch scheme {
				case "http", "https":
					return true
				case "udp", "udp4", "udp6":
					return allowUDP
				}
				return false
			})
			key := canonical
			if !ok {
				key = raw
			}
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			if !ok || total >= MaxTrackers {
				dropped = append(dropped, raw)
				continue
			}
			out = append(out, canonical)
			total++
		}
		if len(out) > 0 {
			kept = append(kept, out)
		}
	}
	return kept, dropped
}

// FilterWebSeeds keeps public http(s) web seed URLs.
func FilterWebSeeds(urls []string) []string {
	var out []string
	seen := make(map[string]struct{})
	for _, raw := range urls {
		canonical, ok := canonicalURL(strings.TrimSpace(raw), func(scheme string) bool {
			return scheme == "http" || scheme == "https"
		})
		if !ok {
			continue
		}
		if _, dup := seen[canonical]; dup {
			continue
		}
		seen[canonical] = struct{}{}
		out = append(out, canonical)
		if len(out) == MaxWebSeeds {
			break
		}
	}
	return out
}

// canonicalURL parses raw and returns its canonical form when the scheme is
// allowed and the host is public. The engine requires tracker URLs to survive
// a url.Parse/String round trip unchanged (it panics otherwise) and rejects
// URLs with credentials the same way, so both are filtered here.
func canonicalURL(raw string, schemeAllowed func(string) bool) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.Opaque != "" || !schemeAllowed(strings.ToLower(u.Scheme)) {
		return "", false
	}
	if !PublicHost(u.Hostname()) {
		return "", false
	}
	s := u.String()
	again, err := url.Parse(s)
	if err != nil || again.String() != s {
		return "", false
	}
	return s, true
}

// FilterPeerAddrs keeps public "ip:port" peer addresses.
func FilterPeerAddrs(addrs []string) []string {
	var out []string
	for _, a := range addrs {
		ap, err := netip.ParseAddrPort(strings.TrimSpace(a))
		if err != nil || ap.Port() == 0 {
			continue
		}
		ip := ap.Addr().Unmap()
		if !publicIP(ip) {
			continue
		}
		out = append(out, netip.AddrPortFrom(ip, ap.Port()).String())
		if len(out) == MaxPeerAddrs {
			break
		}
	}
	return out
}

var cgnat = netip.MustParsePrefix("100.64.0.0/10")

func publicIP(ip netip.Addr) bool {
	return ip.IsGlobalUnicast() && !ip.IsPrivate() && !cgnat.Contains(ip)
}

// PublicHost reports whether host is a fully qualified DNS name or a globally
// routable IP literal. Loopback, private, link-local and local-only names are
// rejected so untrusted links cannot make the client probe the local network.
func PublicHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "" {
		return false
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		return publicIP(ip.Unmap())
	}
	if host == "localhost" {
		return false
	}
	for _, suffix := range []string{".localhost", ".local", ".lan", ".home.arpa", ".internal", ".intranet", ".corp"} {
		if strings.HasSuffix(host, suffix) {
			return false
		}
	}
	// Single-label names resolve through local search domains.
	return strings.Contains(host, ".")
}

var executableExts = map[string]struct{}{
	".exe": {}, ".scr": {}, ".com": {}, ".pif": {}, ".bat": {}, ".cmd": {}, ".ps1": {}, ".psm1": {},
	".vbs": {}, ".vbe": {}, ".js": {}, ".jse": {}, ".wsf": {}, ".wsh": {}, ".hta": {}, ".msi": {},
	".msp": {}, ".lnk": {}, ".reg": {}, ".jar": {}, ".dll": {}, ".cpl": {}, ".scf": {}, ".inf": {},
	".msc": {}, ".application": {}, ".appref-ms": {}, ".gadget": {}, ".chm": {}, ".url": {},
	".docm": {}, ".xlsm": {}, ".pptm": {},
}

// RiskyFileName reports whether Windows could run the file when it is opened,
// or the name hides its real extension with Unicode direction overrides.
func RiskyFileName(name string) bool {
	if HasDirectionOverride(name) {
		return true
	}
	// Windows ignores trailing dots and spaces, so "setup.exe. " still runs.
	trimmed := strings.TrimRight(name, ". ")
	_, ok := executableExts[strings.ToLower(filepath.Ext(trimmed))]
	return ok
}

// HasDirectionOverride reports Unicode bidi controls, used to disguise names
// like "report<RLO>fdp.exe" as "reportexe.pdf".
func HasDirectionOverride(s string) bool {
	for _, r := range s {
		if isDirectionControl(r) {
			return true
		}
	}
	return false
}

func isDirectionControl(r rune) bool {
	return (r >= 0x202A && r <= 0x202E) || (r >= 0x2066 && r <= 0x2069) || r == 0x200E || r == 0x200F || r == 0x061C
}

// CleanText makes untrusted single-line text safe to display: control and
// direction-override characters are removed and the length is bounded.
func CleanText(s string, maxRunes int) string {
	return clean(s, maxRunes, false)
}

// CleanMultiline is CleanText that keeps line breaks.
func CleanMultiline(s string, maxRunes int) string {
	return clean(s, maxRunes, true)
}

func clean(s string, maxRunes int, keepNewlines bool) string {
	s = strings.ToValidUTF8(s, "�")
	var b strings.Builder
	n := 0
	for _, r := range s {
		switch {
		case r == '\n' && keepNewlines:
		case r == '\n' || r == '\r' || r == '\t':
			r = ' '
		case unicode.IsControl(r) || isDirectionControl(r):
			continue
		}
		if n == maxRunes {
			b.WriteRune('…')
			break
		}
		b.WriteRune(r)
		n++
	}
	return strings.TrimSpace(b.String())
}

// WithinDir reports whether target lies strictly inside base.
func WithinDir(base, target string) bool {
	rel, err := filepath.Rel(filepath.Clean(base), filepath.Clean(target))
	if err != nil || filepath.IsAbs(rel) {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// IsTorrentID reports whether id is a lowercase hex info hash.
func IsTorrentID(id string) bool {
	if len(id) != 40 && len(id) != 64 {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
