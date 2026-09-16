// Development-only mock of the Wails runtime and Go bindings, so the UI can be
// designed in a normal browser (see tools/preview). Never shipped in the .exe.
(function () {
  const params = new URLSearchParams(location.search);
  const lang = params.get('lang') || 'en';
  const theme = params.get('theme') || 'dark';
  const mode = params.get('mode') || 'direct';
  const empty = params.has('empty');
  const GB = 1024 ** 3;
  const MB = 1024 ** 2;
  const now = Date.now();
  const listeners = {};

  const settings = {
    language: lang, theme, downloadDir: 'C:\\Users\\Demo\\Downloads', startImmediately: true, warnExecutables: true,
    markOfTheWeb: true, rememberTorrents: true, downloadLimitKiB: 0, uploadLimitKiB: 2048, seedAfterComplete: true,
    seedRatioLimit: 1, maxPeersPerTorrent: 50, listenPort: 48213, randomizePort: false, enableDHT: mode !== 'proxy',
    enablePEX: true, enableUTP: true, enableIPv6: true, enableUPnP: false, enableWebSeeds: true, encryption: 'require',
    anonymousMode: true, networkMode: mode, vpnInterface: mode === 'vpn' ? 'ProtonVPN' : '', autoReconnect: true,
    proxy: { host: mode === 'proxy' ? '127.0.0.1' : '', port: 1080, username: '' }, blocklistPath: '',
    proxyPasswordSet: false, clearProxyPassword: false,
  };

  const seedList = [
    ['ubuntu-24.04.2-desktop-amd64.iso', 'downloading', 0.634, 5.9 * GB, 6.8 * MB, 420 * 1024, 42, 18],
    ['Blender 4.4 — Open Movie Collection', 'downloading', 0.212, 38.2 * GB, 2.1 * MB, 96 * 1024, 17, 6],
    ['debian-12.10.0-amd64-DVD-1.iso', 'seeding', 1, 3.7 * GB, 0, 1.3 * MB, 9, 0],
    ['Big Buck Bunny (4K, 60fps)', 'completed', 1, 12.4 * GB, 0, 0, 0, 0],
    ['LibreOffice 25.2 portable', 'stalled', 0.087, 412 * MB, 0, 0, 0, 0],
    ['Sintel.2010.1080p.mkv', 'paused', 0.48, 1.2 * GB, 0, 0, 0, 0],
    ['Wikipedia offline dump (ru) 2025-06', 'metadata', 0, 0, 0, 0, 3, 0],
    ['Arch Linux 2025.06.01', 'error', 0.91, 1.1 * GB, 0, 0, 0, 0],
  ];
  const torrents = empty ? [] : seedList.map((s, i) => ({
    id: (i.toString(16) + 'c9e15763f722f23e98a29decdfae341b98d5305').slice(0, 40),
    name: s[0], state: s[1], progress: s[2], size: s[3], totalSize: s[3], done: Math.round(s[3] * s[2]),
    downRate: s[4], upRate: s[5], eta: s[4] ? Math.round((s[3] * (1 - s[2])) / s[4]) : -1, peers: s[6], seeds: s[7],
    knownPeers: s[6] * 4, ratio: [0.12, 0.03, 2.41, 1.02, 0, 0.3, 0, 0.8][i], downloaded: Math.round(s[3] * s[2]),
    uploaded: Math.round(s[3] * s[2] * 0.4), addedAt: now - i * 3600e3 * 7, completedAt: s[2] === 1 ? now - 3600e3 : 0,
    savePath: settings.downloadDir, error: s[1] === 'error' ? 'io: disk full' : '', private: i === 3, hasMeta: s[1] !== 'metadata',
    risky: i === 4,
  }));

  function global() {
    const online = params.get('offline') === null;
    return {
      downRate: torrents.reduce((a, t) => a + t.downRate, 0),
      upRate: torrents.reduce((a, t) => a + t.upRate, 0),
      dhtNodes: mode === 'proxy' ? 0 : 312,
      freeSpace: 214 * GB,
      network: {
        mode, online, error: online ? '' : 'vpnInterfaceDown: ProtonVPN', killSwitch: !online, interface: mode === 'vpn' ? 'ProtonVPN' : '',
        boundIPv4: mode === 'vpn' ? '10.2.0.2' : '', boundIPv6: '', proxy: mode === 'proxy' ? '127.0.0.1:1080' : '', proxyReachable: true,
        listenPort: mode === 'proxy' ? 0 : 48213, dht: settings.enableDHT && mode !== 'proxy', pex: true, utp: mode !== 'proxy', upnp: false,
        incoming: mode !== 'proxy', blocklistRules: 0,
      },
    };
  }

  const state = () => ({ torrents: JSON.parse(JSON.stringify(torrents)), global: global() });

  setInterval(() => {
    for (const t of torrents) {
      if (t.state === 'downloading') {
        t.downRate = Math.max(0, t.downRate * (0.85 + Math.random() * 0.3));
        t.progress = Math.min(0.999, t.progress + t.downRate / t.size);
        t.done = Math.round(t.size * t.progress);
      }
    }
    (listeners.state || []).forEach((fn) => fn(state()));
  }, 1000);

  const files = [
    { index: 0, path: 'Blender Open Movies/Sintel/sintel-4k.mkv', size: 9.4 * GB, done: 2.1 * GB, priority: 2, risky: false },
    { index: 1, path: 'Blender Open Movies/Spring/spring-4k.mkv', size: 7.1 * GB, done: 1.4 * GB, priority: 1, risky: false },
    { index: 2, path: 'Blender Open Movies/Charge/charge-4k.mkv', size: 11.2 * GB, done: 0.6 * GB, priority: 1, risky: false },
    { index: 3, path: 'Blender Open Movies/extras/subtitles.zip', size: 18 * MB, done: 18 * MB, priority: 1, risky: false },
    { index: 4, path: 'Blender Open Movies/extras/install-codec.exe', size: 2.4 * MB, done: 0, priority: 0, risky: true },
  ];
  const peers = [
    { address: '185.21.216.4:51413', client: 'qBittorrent 5.1.0', network: 'tcp4', source: 'Tr', progress: 1, downRate: 2.4 * MB, upRate: 40 * 1024 },
    { address: '[2a01:4f8:c17:1::2]:6881', client: 'Transmission 4.0.6', network: 'tcp6', source: 'Hg', progress: 0.72, downRate: 1.1 * MB, upRate: 120 * 1024 },
    { address: '93.184.216.34:49160', client: '', network: 'udp4', source: 'X', progress: 0.35, downRate: 310 * 1024, upRate: 0 },
  ];

  const App = {
    GetBootstrap: async () => ({
      appName: 'Torrent', version: '1.0.0', settings, firstRun: false, portable: true,
      dataDir: 'D:\\Apps\\Torrent\\TorrentData', engineError: '', settingsError: '', args: [], secretStorage: true, platform: 'windows',
      links: [
        { id: 'buymeacoffee', name: 'Buy Me a Coffee', url: 'https://buymeacoffee.com/aristarh.ucolov' },
        { id: 'kofi', name: 'Ko-fi', url: 'https://ko-fi.com/aristarhucolov' },
        { id: 'donationalerts', name: 'DonationAlerts', url: 'https://www.donationalerts.com/r/aristarh_ucolov' },
      ],
    }),
    GetState: async () => state(),
    SaveSettings: async (s) => Object.assign(settings, s, { proxy: { ...s.proxy, password: undefined } }),
    ListInterfaces: async () => [
      { name: 'ProtonVPN', addresses: ['10.2.0.2'], up: true, likelyVPN: true },
      { name: 'Ethernet', addresses: ['192.168.1.24', 'fd00::24'], up: true, likelyVPN: false },
      { name: 'Wi-Fi', addresses: ['192.168.1.37'], up: true, likelyVPN: false },
    ],
    PickTorrentFiles: async () => ['C:\\Users\\Demo\\Downloads\\blender-open-movies.torrent'],
    PickFolder: async (title, current) => current,
    PickBlocklist: async () => '',
    FreeSpace: async () => 214 * GB,
    ClipboardMagnet: async () => 'magnet:?xt=urn:btih:c9e15763f722f23e98a29decdfae341b98d53056&dn=Sintel&tr=udp%3A%2F%2Ftracker.opentrackr.org%3A1337%2Fannounce',
    PreviewTorrent: async (path) => ({
      id: 'ab'.repeat(20), path, name: 'Blender Open Movies', totalSize: 27.7 * GB, fileCount: 5, private: false,
      comment: '', createdBy: 'mktorrent 1.1', createdAt: 1717000000, trackers: 3, riskyFiles: 1, exists: false,
      files: files.map((f) => ({ index: f.index, path: f.path, size: f.size, risky: f.risky })),
    }),
    AddTorrentFile: async () => 'ab'.repeat(20),
    PreviewMagnet: async (uri) => {
      if (!/^magnet:\?xt=urn:bt(ih|mh):/i.test(uri.trim())) throw 'invalidMagnet';
      return { id: 'c9e15763f722f23e98a29decdfae341b98d53056', name: 'Sintel', trackers: 1, skipped: mode === 'proxy' ? 1 : 0, exists: false };
    },
    AddMagnet: async () => 'c9e15763f722f23e98a29decdfae341b98d53056',
    Pause: async () => {}, Resume: async () => {}, Remove: async () => {}, Recheck: async () => {},
    SetFilePriority: async () => {},
    GetDetails: async (id, section) => {
      const t = torrents.find((x) => x.id === id) || torrents[0];
      return {
        id: t.id, infoHashV2: '', name: t.name, savePath: t.savePath, comment: 'Creative Commons Attribution 4.0\nhttps://studio.blender.org',
        createdBy: 'mktorrent 1.1', createdAt: 1717000000, pieceLength: 4 * MB, pieces: Math.ceil((t.size || 1) / (4 * MB)), private: t.private,
        files: section === 'files' ? files : [], peers: section === 'peers' ? peers : [],
        trackers: section === 'trackers' ? [
          { url: 'https://tracker.example.org/announce', tier: 0, skipped: false },
          { url: 'udp://tracker.opentrackr.org:1337/announce', tier: 1, skipped: mode === 'proxy' },
        ] : [],
      };
    },
    OpenFolder: async () => {}, GetMagnetLink: async () => 'magnet:?xt=urn:btih:demo', Reconnect: async () => {},
    OpenLink: async () => {}, OpenDataFolder: async () => {},
  };

  window.go = { main: { App } };
  window.runtime = {
    EventsOn(name, fn) {
      (listeners[name] = listeners[name] || []).push(fn);
      return () => { listeners[name] = listeners[name].filter((f) => f !== fn); };
    },
    OnFileDrop() {},
    ClipboardSetText: async () => true,
    WindowSetDarkTheme() {}, WindowSetLightTheme() {}, WindowSetSystemDefaultTheme() {},
  };

  // Screenshot helpers: ?view=settings, ?select=1, ?dialog=magnet|file|remove
  window.addEventListener('load', () => {
    setTimeout(() => {
      const view = params.get('view');
      if (view) document.querySelector(`[data-view="${view}"]`)?.click();
      const filter = params.get('filter');
      if (filter) document.querySelector(`[data-filter="${filter}"]`)?.click();
      const select = params.get('select');
      if (select !== null) document.querySelectorAll('.row')[Number(select)]?.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, button: 0 }));
      const tab = params.get('tab');
      if (tab) setTimeout(() => [...document.querySelectorAll('.tab')].find((b) => b.textContent.trim().toLowerCase().startsWith(tab))?.click(), 50);
      const dialog = params.get('dialog');
      if (dialog === 'magnet') document.dispatchEvent(new KeyboardEvent('keydown', { key: 'v', ctrlKey: true, bubbles: true }));
      if (dialog === 'file') document.dispatchEvent(new KeyboardEvent('keydown', { key: 'o', ctrlKey: true, bubbles: true }));
      if (dialog === 'menu') document.querySelectorAll('.row')[0]?.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, clientX: 520, clientY: 200 }));
      if (params.get('scroll')) document.querySelector('.page')?.scrollTo(0, Number(params.get('scroll')));
    }, 400);
  });
})();
