package safety

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseMagnet(t *testing.T) {
	const valid = "magnet:?xt=urn:btih:c9e15763f722f23e98a29decdfae341b98d53056&dn=Example&tr=udp%3A%2F%2Ftracker.example.org%3A1337%2Fannounce"
	m, err := ParseMagnet("  " + valid + "\n")
	if err != nil {
		t.Fatalf("valid magnet rejected: %v", err)
	}
	if m.DisplayName != "Example" || len(m.Trackers) != 1 {
		t.Errorf("unexpected parse result: %+v", m)
	}
	for _, bad := range []string{
		"",
		"https://example.org/file.torrent",
		"magnet:?dn=no-hash",
		"magnet:?xt=urn:btih:1234",
		"magnet:?xt=urn:btih:" + strings.Repeat("a", MaxMagnetLength),
	} {
		if _, err := ParseMagnet(bad); err == nil {
			t.Errorf("expected %.60q to be rejected", bad)
		}
	}
}

func TestFilterTrackers(t *testing.T) {
	tiers := [][]string{
		{"udp://tracker.example.org:1337/announce", "HTTP://tracker.example.org/announce"},
		{
			"https://user:pass@tracker.example.org/announce",
			"wss://tracker.example.org",
			"http://127.0.0.1:8080/announce",
			"http://192.168.1.10/announce",
			"http://tracker/announce",
		},
		{"http://tracker.example.org/announce"},
	}

	kept, dropped := FilterTrackers(tiers, false)
	want := [][]string{{"http://tracker.example.org/announce"}}
	if !reflect.DeepEqual(kept, want) {
		t.Errorf("proxy mode kept = %v, want %v", kept, want)
	}
	if len(dropped) != 6 {
		t.Errorf("proxy mode dropped %d URLs, want 6: %v", len(dropped), dropped)
	}

	kept, _ = FilterTrackers(tiers, true)
	want = [][]string{{"udp://tracker.example.org:1337/announce", "http://tracker.example.org/announce"}}
	if !reflect.DeepEqual(kept, want) {
		t.Errorf("direct mode kept = %v, want %v", kept, want)
	}
}

func TestFilterWebSeeds(t *testing.T) {
	got := FilterWebSeeds([]string{"https://mirror.example.org/file.iso", "ftp://mirror.example.org/x", "http://10.0.0.2/x", "https://mirror.example.org/file.iso"})
	want := []string{"https://mirror.example.org/file.iso"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestFilterPeerAddrs(t *testing.T) {
	got := FilterPeerAddrs([]string{"1.2.3.4:6881", "192.168.1.5:6881", "[2001:db8::1]:51413", "example.org:6881", "5.6.7.8:0", "[::ffff:9.9.9.9]:80", "127.0.0.1:1"})
	want := []string{"1.2.3.4:6881", "[2001:db8::1]:51413", "9.9.9.9:80"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestPublicHost(t *testing.T) {
	cases := map[string]bool{
		"tracker.example.org":  true,
		"8.8.8.8":              true,
		"2001:4860:4860::8888": true,
		"":                     false,
		"localhost":            false,
		"printer.local":        false,
		"tracker":              false,
		"127.0.0.1":            false,
		"10.1.2.3":             false,
		"192.168.0.1":          false,
		"100.64.0.1":           false,
		"169.254.1.1":          false,
		"0.0.0.0":              false,
		"::1":                  false,
		"fd00::1":              false,
		"fe80::1":              false,
	}
	for host, want := range cases {
		if got := PublicHost(host); got != want {
			t.Errorf("PublicHost(%q) = %v, want %v", host, got, want)
		}
	}
}

func TestRiskyFileName(t *testing.T) {
	for _, name := range []string{"setup.exe", "Run.BAT", "invoice.pdf.exe", "archive.exe. ", "report‮fdp.txt", "script.ps1", "shortcut.lnk", "macro.docm"} {
		if !RiskyFileName(name) {
			t.Errorf("%q should be risky", name)
		}
	}
	for _, name := range []string{"movie.mp4", "Song.MP3", "ubuntu-24.04.iso", "readme.txt", "exe", "photos/summer.jpg"} {
		if RiskyFileName(name) {
			t.Errorf("%q should not be risky", name)
		}
	}
}

func TestCleanText(t *testing.T) {
	if got := CleanText("a\x00b‮c\td\ne", 100); got != "abc d e" {
		t.Errorf("CleanText = %q", got)
	}
	if got := CleanText("abcdef", 3); got != "abc…" {
		t.Errorf("truncation = %q", got)
	}
	if got := CleanMultiline("line1\nline2\x07", 100); got != "line1\nline2" {
		t.Errorf("CleanMultiline = %q", got)
	}
}

func TestWithinDir(t *testing.T) {
	base := t.TempDir()
	if !WithinDir(base, filepath.Join(base, "a", "b.txt")) {
		t.Error("child path should be inside")
	}
	for _, target := range []string{base, filepath.Join(base, "..", "x"), filepath.Dir(base), filepath.Join(base, "a", "..", "..", "y")} {
		if WithinDir(base, target) {
			t.Errorf("%q must not be considered inside %q", target, base)
		}
	}
}

func TestIsTorrentID(t *testing.T) {
	if !IsTorrentID("c9e15763f722f23e98a29decdfae341b98d53056") {
		t.Error("valid v1 id rejected")
	}
	for _, bad := range []string{"", "C9E15763F722F23E98A29DECDFAE341B98D53056", "../../etc/passwd", "zz"} {
		if IsTorrentID(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
}
