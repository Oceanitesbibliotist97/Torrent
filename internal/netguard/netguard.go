// Package netguard builds leak-resistant network primitives: sockets bound to
// a VPN adapter, a SOCKS5 dialer that never resolves names locally, and DNS
// resolvers that fail closed instead of silently using the default route.
package netguard

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

var (
	ErrInterfaceMissing = errors.New("network interface not found")
	ErrInterfaceDown    = errors.New("network interface is down or has no usable address")
	ErrDNSBlocked       = errors.New("dns lookups are blocked to prevent leaks")
	ErrNoRoute          = errors.New("destination is not reachable through the bound interface")
	ErrUDPBlocked       = errors.New("udp is disabled while using a proxy")
)

const dialTimeout = 20 * time.Second

// systemResolver is the process default, captured before any override.
var systemResolver = net.DefaultResolver

// InstallResolver replaces the process-wide resolver that libraries use when
// they resolve names on their own. Passing nil restores the system resolver.
func InstallResolver(r *net.Resolver) {
	if r == nil {
		r = systemResolver
	}
	net.DefaultResolver = r
}

// BlockedResolver fails every DNS query instead of letting it leave unprotected.
func BlockedResolver() *net.Resolver {
	return &net.Resolver{
		PreferGo: true,
		Dial: func(context.Context, string, string) (net.Conn, error) {
			return nil, ErrDNSBlocked
		},
	}
}

// BlockedListenPacket refuses to open UDP sockets.
func BlockedListenPacket(string, string) (net.PacketConn, error) {
	return nil, ErrUDPBlocked
}

// Interface describes a network adapter the user can bind to.
type Interface struct {
	Name      string   `json:"name"`
	Addresses []string `json:"addresses"`
	Up        bool     `json:"up"`
	LikelyVPN bool     `json:"likelyVPN"`
}

var (
	vpnHints = []string{
		"vpn", "wireguard", "wintun", "openvpn", "tap-windows", "tun", "wg", "proton", "mullvad", "nord",
		"ivpn", "surfshark", "windscribe", "private internet access", "pia", "airvpn", "torguard",
		"cyberghost", "hide.me", "amnezia", "ovpn",
	}
	notVPNHints = []string{"teredo", "isatap", "6to4", "bluetooth", "loopback", "vethernet", "virtualbox", "vmware", "hyper-v"}
)

func looksLikeVPN(name string) bool {
	n := strings.ToLower(name)
	for _, h := range notVPNHints {
		if strings.Contains(n, h) {
			return false
		}
	}
	for _, h := range vpnHints {
		if strings.Contains(n, h) {
			return true
		}
	}
	return false
}

// ListInterfaces returns non-loopback adapters that have a usable address,
// likely VPN adapters first.
func ListInterfaces() ([]Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	out := []Interface{}
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs := usableAddrs(ifc)
		if len(addrs) == 0 {
			continue
		}
		strs := make([]string, len(addrs))
		for i, a := range addrs {
			strs[i] = a.String()
		}
		out = append(out, Interface{
			Name:      ifc.Name,
			Addresses: strs,
			Up:        ifc.Flags&net.FlagUp != 0,
			LikelyVPN: looksLikeVPN(ifc.Name),
		})
	}
	slices.SortStableFunc(out, func(a, b Interface) int {
		if a.LikelyVPN != b.LikelyVPN {
			if a.LikelyVPN {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return out, nil
}

func usableAddrs(ifc net.Interface) []netip.Addr {
	addrs, err := ifc.Addrs()
	if err != nil {
		return nil
	}
	var out []netip.Addr
	for _, a := range addrs {
		var ip net.IP
		switch v := a.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		addr, ok := netip.AddrFromSlice(ip)
		if !ok {
			continue
		}
		addr = addr.Unmap()
		if addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsMulticast() || addr.IsUnspecified() {
			continue
		}
		out = append(out, addr)
	}
	return out
}

// Binding pins traffic to the addresses of one network adapter. On Windows
// (strong host model) a socket bound to the VPN adapter's address can only
// leave through that adapter, and fails once the VPN disconnects.
type Binding struct {
	Interface string
	IPv4      netip.Addr
	IPv6      netip.Addr
}

// ResolveBinding looks up the current addresses of the named adapter.
func ResolveBinding(name string, allowIPv6 bool) (Binding, error) {
	ifc, err := net.InterfaceByName(name)
	if err != nil {
		return Binding{}, fmt.Errorf("%w: %s", ErrInterfaceMissing, name)
	}
	if ifc.Flags&net.FlagUp == 0 {
		return Binding{}, fmt.Errorf("%w: %s", ErrInterfaceDown, name)
	}
	b := Binding{Interface: name}
	for _, a := range usableAddrs(*ifc) {
		switch {
		case a.Is4() && !b.IPv4.IsValid():
			b.IPv4 = a
		case a.Is6() && allowIPv6 && !b.IPv6.IsValid():
			b.IPv6 = a
		}
	}
	if !b.IPv4.IsValid() && !b.IPv6.IsValid() {
		return Binding{}, fmt.Errorf("%w: %s", ErrInterfaceDown, name)
	}
	return b, nil
}

// Alive reports whether the adapter is up and still owns the bound addresses.
func (b Binding) Alive() bool {
	ifc, err := net.InterfaceByName(b.Interface)
	if err != nil || ifc.Flags&net.FlagUp == 0 {
		return false
	}
	addrs := usableAddrs(*ifc)
	if b.IPv4.IsValid() && !slices.Contains(addrs, b.IPv4) {
		return false
	}
	if b.IPv6.IsValid() && !slices.Contains(addrs, b.IPv6) {
		return false
	}
	return true
}

func (b Binding) localFor(remote netip.Addr) (netip.Addr, bool) {
	if remote.Is4() {
		return b.IPv4, b.IPv4.IsValid()
	}
	return b.IPv6, b.IPv6.IsValid()
}

// Resolver returns a DNS resolver whose queries originate from the bound
// addresses, so lookups follow the VPN instead of the default route.
func (b Binding) Resolver() *net.Resolver {
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			ap, err := netip.ParseAddrPort(address)
			if err != nil {
				return nil, err
			}
			local, ok := b.localFor(ap.Addr().Unmap())
			if !ok {
				return nil, ErrNoRoute
			}
			d := net.Dialer{Timeout: 10 * time.Second}
			if strings.HasPrefix(network, "udp") {
				d.LocalAddr = &net.UDPAddr{IP: local.AsSlice()}
			} else {
				d.LocalAddr = &net.TCPAddr{IP: local.AsSlice()}
			}
			return d.DialContext(ctx, network, address)
		},
	}
}

// DialContext dials from the bound addresses. Destinations in an address
// family the binding lacks are refused rather than sent via another adapter.
func (b Binding) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	var ips []netip.Addr
	if ip, err := netip.ParseAddr(host); err == nil {
		ips = []netip.Addr{ip}
	} else if ips, err = b.Resolver().LookupNetIP(ctx, "ip", host); err != nil {
		return nil, err
	}
	proto := strings.TrimRight(network, "46")
	var lastErr error
	for _, ip := range ips {
		ip = ip.Unmap()
		if (strings.HasSuffix(network, "4") && !ip.Is4()) || (strings.HasSuffix(network, "6") && !ip.Is6()) {
			continue
		}
		local, ok := b.localFor(ip)
		if !ok {
			continue
		}
		d := net.Dialer{Timeout: dialTimeout}
		family := proto + "4"
		if ip.Is6() {
			family = proto + "6"
		}
		if proto == "udp" {
			d.LocalAddr = &net.UDPAddr{IP: local.AsSlice()}
		} else {
			d.LocalAddr = &net.TCPAddr{IP: local.AsSlice()}
		}
		conn, err := d.DialContext(ctx, family, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("%w: %s via %s", ErrNoRoute, address, b.Interface)
	}
	return nil, lastErr
}

// ListenPacket opens an ephemeral UDP socket on the bound address.
func (b Binding) ListenPacket(network, _ string) (net.PacketConn, error) {
	if network != "udp6" && b.IPv4.IsValid() {
		return net.ListenPacket("udp4", net.JoinHostPort(b.IPv4.String(), "0"))
	}
	if network != "udp4" && b.IPv6.IsValid() {
		return net.ListenPacket("udp6", net.JoinHostPort(b.IPv6.String(), "0"))
	}
	return nil, ErrNoRoute
}

// Proxy is a SOCKS5 endpoint.
type Proxy struct {
	Host     string
	Port     int
	Username string
	Password string
}

// address resolves the proxy host with the system resolver. This is the only
// name the app ever resolves directly in proxy mode.
func (p Proxy) address(ctx context.Context) (string, error) {
	if p.Port < 1 || p.Port > 65535 {
		return "", fmt.Errorf("invalid proxy port %d", p.Port)
	}
	host := p.Host
	if _, err := netip.ParseAddr(host); err != nil {
		addrs, err := systemResolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return "", fmt.Errorf("resolve proxy %s: %w", host, err)
		}
		if len(addrs) == 0 {
			return "", fmt.Errorf("resolve proxy %s: no addresses", host)
		}
		host = addrs[0].Unmap().String()
	}
	return net.JoinHostPort(host, strconv.Itoa(p.Port)), nil
}

// NewProxyDialer returns a dialer that tunnels TCP through SOCKS5. Destination
// host names are sent to the proxy unresolved (socks5h behaviour), so they are
// never looked up locally.
func NewProxyDialer(ctx context.Context, p Proxy) (proxy.ContextDialer, error) {
	addr, err := p.address(ctx)
	if err != nil {
		return nil, err
	}
	var auth *proxy.Auth
	if p.Username != "" || p.Password != "" {
		auth = &proxy.Auth{User: p.Username, Password: p.Password}
	}
	forward := &net.Dialer{Timeout: dialTimeout, Resolver: BlockedResolver()}
	d, err := proxy.SOCKS5("tcp", addr, auth, forward)
	if err != nil {
		return nil, err
	}
	cd, ok := d.(proxy.ContextDialer)
	if !ok {
		return nil, errors.New("socks5 dialer does not support contexts")
	}
	return cd, nil
}

// ProbeProxy checks that the proxy accepts TCP connections.
func ProbeProxy(ctx context.Context, p Proxy) error {
	addr, err := p.address(ctx)
	if err != nil {
		return err
	}
	d := net.Dialer{Timeout: 5 * time.Second}
	c, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	return c.Close()
}
