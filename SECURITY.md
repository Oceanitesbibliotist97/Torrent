# Security model

## Threats considered

1. **Hostile torrents and magnet links.** A link from any website can carry crafted tracker URLs, peer addresses, web seeds, file names and metadata.
2. **Network leaks.** Traffic, DNS lookups or WebRTC escaping the VPN or proxy the user chose.
3. **Local attack surface.** Other programs or websites talking to the client, or DLLs planted next to a portable executable.
4. **Dangerous downloads.** Executables disguised as media or documents.
5. **Data at rest.** Settings, secrets and history on a shared or lost machine.

## Mitigations

### Untrusted input (`internal/safety`, `internal/engine/files.go`)

- Magnet links are limited to 16 KiB and must contain a v1 or v2 info hash. `.torrent` files are limited to 32 MiB and 200,000 files, and piece counts are checked against sizes.
- Tracker URLs are canonicalised. URLs with credentials or that do not survive a parse/format round trip are dropped, because the engine panics on them. Only `http`, `https` and `udp` are accepted, and `udp` is also dropped in proxy mode.
- Trackers, web seeds and `x.pe` peers pointing at loopback, private, link-local, CGNAT or local-only names are dropped, so links cannot probe your LAN. Magnet `xs`/`as` sources are ignored.
- Displayed names are stripped of control and bidi override characters; the UI inserts all untrusted text with `textContent`.
- File deletion checks every path lexically **and** after resolving links, so junctions cannot redirect deletion outside the download folder.

### Network (`internal/netguard`, `internal/engine/client.go`)

- **VPN mode:** listen sockets, outgoing TCP dials (the library's own dialers are replaced), uTP, tracker HTTP/UDP sockets, web seeds and DNS queries are all bound to the VPN adapter's addresses. Windows' strong host model keeps such packets on that adapter. Losing the address closes every client (kill switch); nothing falls back to the default route.
- **Proxy mode:** TCP goes through SOCKS5 with remote name resolution. DHT, uTP and UDP trackers are disabled, the process-wide resolver fails closed, and the client listens on loopback only and accepts nothing.
- WebTorrent/WebRTC is always disabled. UPnP/NAT-PMP is off by default and never used with a VPN or proxy.
- Anonymous mode sends a random peer ID, no client version in the extended handshake and no User-Agent. Protocol encryption uses full-stream RC4 after the MSE handshake.
- Private torrents (BEP 27) run in a separate client with DHT and PEX disabled.

Tests cover the fail-closed resolver on Windows, bound dialing, SOCKS5 remote resolution, the proxy-mode configuration and an end-to-end transfer.

### Local surface (`main.go`, `app.go`, `frontend/`)

- No HTTP server: the UI calls Go over in-process WebView2 IPC. Every bound method validates its arguments (hex info hashes, absolute paths, size limits).
- Content-Security-Policy: `default-src 'none'; script-src 'self'; style-src 'self'`, with no remote resources. Only allowlisted links can be opened externally.
- DLL search is restricted to System32. The manifest requests `asInvoker` (no admin rights).
- The clipboard is read only on request and only a valid magnet link is returned to the UI.
- WebView2 fraudulent-website detection, which reports URLs to Microsoft, is disabled, and the default browser context menu is off.

### Downloads and data at rest

- Executables and bidi-disguised names are flagged before download; completed files receive the Mark of the Web (zone 3, no source URL).
- The proxy password is encrypted with DPAPI; the saved password is never returned to the UI.
- Files are written atomically with owner-only permissions. With history off, the session, metainfo and resume data are deleted on exit.

### Build

- Pure Go (`CGO_ENABLED=0`), `-trimpath`, empty build ID: reproducible binaries without build-machine paths. Releases publish SHA-256 checksums.

## Known limitations

- Peers always learn the connecting IP; only a VPN or proxy changes it.
- The third-party engine may keep a file handle open until garbage collection; deletion forces a collection and retries.
- Windows SmartScreen may warn about unsigned executables. Code signing is recommended for public releases.

## Reporting a vulnerability

Please report security issues privately to the maintainer (for example through a private security advisory on the project's repository) rather than in public issues. Include steps to reproduce and the app version from **About**.
