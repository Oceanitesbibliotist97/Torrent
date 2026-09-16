package netguard

import (
	"context"
	"io"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func TestBlockedResolverFailsClosed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := BlockedResolver().LookupHost(ctx, "example.com")
	if err == nil {
		t.Fatal("lookup succeeded through the blocked resolver")
	}
	if !strings.Contains(err.Error(), ErrDNSBlocked.Error()) {
		t.Fatalf("lookup did not go through the blocking dialer (system resolver used?): %v", err)
	}
}

func TestBindingDialUsesBoundAddress(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		if c, err := ln.Accept(); err == nil {
			c.Close()
		}
	}()
	b := Binding{Interface: "test", IPv4: netip.MustParseAddr("127.0.0.1")}
	conn, err := b.DialContext(context.Background(), "tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if local := conn.LocalAddr().(*net.TCPAddr); !local.IP.Equal(net.IPv4(127, 0, 0, 1)) {
		t.Errorf("connection left from %v, want 127.0.0.1", local.IP)
	}
}

func TestBindingRefusesMissingFamily(t *testing.T) {
	b := Binding{Interface: "test", IPv4: netip.MustParseAddr("127.0.0.1")}
	if _, err := b.DialContext(context.Background(), "tcp", "[::1]:9"); err == nil {
		t.Fatal("IPv6 destination must be refused without an IPv6 binding")
	}
	if _, err := b.ListenPacket("udp6", ":0"); err == nil {
		t.Fatal("udp6 socket must be refused without an IPv6 binding")
	}
}

// TestProxyDialerSendsHostnameToProxy runs a minimal SOCKS5 server and checks
// that destination names reach the proxy unresolved.
func TestProxyDialerSendsHostnameToProxy(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	requested := make(chan string, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		buf := make([]byte, 512)
		// Greeting: VER NMETHODS METHODS...
		if _, err := io.ReadFull(c, buf[:2]); err != nil {
			return
		}
		if _, err := io.ReadFull(c, buf[:buf[1]]); err != nil {
			return
		}
		c.Write([]byte{5, 0})
		// Request: VER CMD RSV ATYP ADDR PORT
		if _, err := io.ReadFull(c, buf[:4]); err != nil {
			return
		}
		switch buf[3] {
		case 3:
			io.ReadFull(c, buf[:1])
			n := int(buf[0])
			io.ReadFull(c, buf[:n])
			requested <- "domain:" + string(buf[:n])
		case 1:
			io.ReadFull(c, buf[:4])
			requested <- "ipv4:" + net.IP(buf[:4]).String()
		default:
			requested <- "other"
		}
		io.ReadFull(c, buf[:2])
		c.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0})
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	d, err := NewProxyDialer(ctx, Proxy{Host: "127.0.0.1", Port: ln.Addr().(*net.TCPAddr).Port})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := d.DialContext(ctx, "tcp", "tracker.example.org:80")
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
	if got := <-requested; got != "domain:tracker.example.org" {
		t.Fatalf("proxy received %q, want the unresolved host name", got)
	}
}

func TestLooksLikeVPN(t *testing.T) {
	for _, name := range []string{"ProtonVPN", "WireGuard Tunnel", "Mullvad", "wg0", "OpenVPN TAP-Windows6"} {
		if !looksLikeVPN(name) {
			t.Errorf("%q should look like a VPN", name)
		}
	}
	for _, name := range []string{"Ethernet", "Wi-Fi", "Teredo Tunneling Pseudo-Interface", "vEthernet (WSL)"} {
		if looksLikeVPN(name) {
			t.Errorf("%q should not look like a VPN", name)
		}
	}
}
