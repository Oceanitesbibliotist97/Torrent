// Static, trusted SVG markup. Nothing here is ever built from user data.

const SHIELD = '<path d="M12 3 5 6v5c0 4.5 3 8.5 7 10 4-1.5 7-5.5 7-10V6z"/>';

export const ICONS = {
  plus: '<path d="M12 5v14M5 12h14"/>',
  x: '<path d="M18 6 6 18M6 6l12 12"/>',
  search: '<circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/>',
  play: '<path d="M8 5.5v13a1 1 0 0 0 1.5.9l10-6.5a1 1 0 0 0 0-1.8l-10-6.5A1 1 0 0 0 8 5.5z"/>',
  pause: '<rect x="6.5" y="5" width="3.5" height="14" rx="1"/><rect x="14" y="5" width="3.5" height="14" rx="1"/>',
  trash: '<path d="M4 7h16M10 11v6M14 11v6M6 7l1 12a2 2 0 0 0 2 2h6a2 2 0 0 0 2-2l1-12M9 7V4h6v3"/>',
  chevronDown: '<path d="m6 9 6 6 6-6"/>',
  chevronUp: '<path d="m6 15 6-6 6 6"/>',
  chevronRight: '<path d="m9 6 6 6-6 6"/>',
  folder: '<path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/>',
  file: '<path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z"/><path d="M14 3v5h5"/>',
  fileWarning: '<path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z"/><path d="M14 3v5h5M12 11v4M12 18h.01"/>',
  magnet: '<path d="M5 4h4v6a3 3 0 0 0 6 0V4h4v6a7 7 0 0 1-14 0z"/><path d="M5 8h4M15 8h4"/>',
  link: '<path d="M10 13a5 5 0 0 0 7.07 0l3-3a5 5 0 0 0-7.07-7.07l-1.5 1.5"/><path d="M14 11a5 5 0 0 0-7.07 0l-3 3a5 5 0 0 0 7.07 7.07l1.5-1.5"/>',
  copy: '<rect x="9" y="9" width="11" height="11" rx="2"/><path d="M5 15V6a2 2 0 0 1 2-2h8"/>',
  shield: SHIELD,
  shieldCheck: SHIELD + '<path d="m9 12 2 2 4-4"/>',
  shieldAlert: SHIELD + '<path d="M12 8.5v4M12 15.5h.01"/>',
  shieldOff: '<path d="M3 3l18 18"/><path d="M19 13.5c-.7 3.3-3.3 6.2-7 7.5-4-1.5-7-5.5-7-10V6l2-.9M9.5 3.9 12 3l7 3v4.5"/>',
  sliders: '<path d="M4 6h9M17 6h3M4 12h3M11 12h9M4 18h11M19 18h1"/><circle cx="15" cy="6" r="2"/><circle cx="9" cy="12" r="2"/><circle cx="17" cy="18" r="2"/>',
  heart: '<path d="M12 20s-7-4.35-7-10a4 4 0 0 1 7-2.65A4 4 0 0 1 19 10c0 5.65-7 10-7 10z"/>',
  info: '<circle cx="12" cy="12" r="9"/><path d="M12 11v5M12 8h.01"/>',
  arrowDown: '<path d="M12 5v14M6 13l6 6 6-6"/>',
  arrowUp: '<path d="M12 19V5M6 11l6-6 6 6"/>',
  check: '<path d="m5 12 5 5 9-10"/>',
  checkCircle: '<circle cx="12" cy="12" r="9"/><path d="m8 12 3 3 5-6"/>',
  alertTriangle: '<path d="M10.3 4.3 2.6 18a2 2 0 0 0 1.7 3h15.4a2 2 0 0 0 1.7-3L13.7 4.3a2 2 0 0 0-3.4 0z"/><path d="M12 9v4M12 17h.01"/>',
  alertCircle: '<circle cx="12" cy="12" r="9"/><path d="M12 8v5M12 16h.01"/>',
  list: '<path d="M8 6h13M8 12h13M8 18h13M3.5 6h.01M3.5 12h.01M3.5 18h.01"/>',
  clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
  users: '<circle cx="9" cy="8" r="3.5"/><path d="M2.5 20a6.5 6.5 0 0 1 13 0M16 4.5a3.5 3.5 0 0 1 0 7M21.5 20a6.5 6.5 0 0 0-4-6"/>',
  refresh: '<path d="M20 11a8 8 0 0 0-14.9-3M4 4v4h4"/><path d="M4 13a8 8 0 0 0 14.9 3M20 20v-4h-4"/>',
  globe: '<circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3a14 14 0 0 1 0 18M12 3a14 14 0 0 0 0 18"/>',
  lock: '<rect x="5" y="11" width="14" height="10" rx="2"/><path d="M8 11V8a4 4 0 0 1 8 0v3"/>',
  eyeOff: '<path d="M3 3l18 18"/><path d="M10.6 5.1A10 10 0 0 1 12 5c5 0 9 4.5 10 7a13 13 0 0 1-2.4 3.4M6.6 6.6A13 13 0 0 0 2 12c1 2.5 5 7 10 7a9.7 9.7 0 0 0 5.4-1.6"/><path d="M9.9 9.9a3 3 0 0 0 4.2 4.2"/>',
  userX: '<circle cx="10" cy="8" r="4"/><path d="M3 20a7 7 0 0 1 12-4.9"/><path d="m17 15 4 4m0-4-4 4"/>',
  sun: '<circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"/>',
  moon: '<path d="M20 14.5A8 8 0 1 1 9.5 4a6.5 6.5 0 0 0 10.5 10.5z"/>',
  monitor: '<rect x="3" y="4" width="18" height="12" rx="2"/><path d="M8 20h8M12 16v4"/>',
  coffee: '<path d="M4 9h13v5a5 5 0 0 1-5 5H9a5 5 0 0 1-5-5z"/><path d="M17 11h1.5a2.5 2.5 0 0 1 0 5H17M8 3v3M12 3v3"/>',
  cup: '<path d="M5 8h14l-1.5 10a3 3 0 0 1-3 2.5h-5a3 3 0 0 1-3-2.5z"/><path d="M8 4.5c0 1 1 1 1 2M12 3.5c0 1 1 1.5 1 3"/>',
  gift: '<rect x="3" y="8" width="18" height="5" rx="1"/><path d="M5 13v7h14v-7M12 8v12M12 8S10.5 3.5 8 4.5 9 8 12 8zm0 0s1.5-4.5 4-3.5S15 8 12 8z"/>',
  external: '<path d="M14 4h6v6M20 4l-9 9"/><path d="M18 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h5"/>',
  server: '<rect x="3" y="4" width="18" height="7" rx="2"/><rect x="3" y="13" width="18" height="7" rx="2"/><path d="M7 7.5h.01M7 16.5h.01"/>',
  wifi: '<path d="M5 12.5a10 10 0 0 1 14 0M8.5 16a5 5 0 0 1 7 0M2 9a15 15 0 0 1 20 0M12 20h.01"/>',
  zap: '<path d="M13 2 4 14h7l-1 8 9-12h-7z"/>',
  more: '<circle cx="5" cy="12" r="1.2"/><circle cx="12" cy="12" r="1.2"/><circle cx="19" cy="12" r="1.2"/>',
  hardDrive: '<rect x="3" y="13" width="18" height="7" rx="2"/><path d="M5.5 13 8 5h8l2.5 8M7 16.5h.01M11 16.5h.01"/>',
  history: '<path d="M3 12a9 9 0 1 0 3-6.7L3 8"/><path d="M3 3v5h5M12 7v5l3 2"/>',
  ban: '<circle cx="12" cy="12" r="9"/><path d="m5.7 5.7 12.6 12.6"/>',
  stamp: '<path d="M12 3l2.4 1.8 3-.2.8 2.9 2.5 1.7-1 2.8 1 2.8-2.5 1.7-.8 2.9-3-.2L12 21l-2.4-1.8-3 .2-.8-2.9-2.5-1.7 1-2.8-1-2.8 2.5-1.7.8-2.9 3 .2z"/><path d="m9 12 2 2 4-4"/>',
  router: '<rect x="3" y="13" width="18" height="7" rx="2"/><path d="M7 16.5h.01M11 16.5h.01M15 13V9M12.5 6.5a3.5 3.5 0 0 1 5 0M10 4a7 7 0 0 1 10 0"/>',
  nodes: '<circle cx="12" cy="5" r="2"/><circle cx="5" cy="19" r="2"/><circle cx="19" cy="19" r="2"/><path d="M12 7v5M12 12l-5.5 5.5M12 12l5.5 5.5"/>',
  sparkles: '<path d="M12 3v4M12 17v4M3 12h4M17 12h4M6 6l2.5 2.5M15.5 15.5 18 18M6 18l2.5-2.5M15.5 8.5 18 6"/>',
  videoOff: '<path d="M3 3l18 18"/><path d="M15 11v-1l6-4v12l-2-1.3M13 17H5a2 2 0 0 1-2-2V9a2 2 0 0 1 2-2h1"/>',
  keyboard: '<rect x="2" y="6" width="20" height="12" rx="2"/><path d="M6 10h.01M10 10h.01M14 10h.01M18 10h.01M7 14h10"/>',
  package: '<path d="M21 8 12 3 3 8v8l9 5 9-5z"/><path d="M3 8l9 5 9-5M12 13v8"/>',
  scale: '<path d="M12 3v18M6 21h12M6 7h12M6 7l-3 7a3 3 0 0 0 6 0zM18 7l-3 7a3 3 0 0 0 6 0z"/>',
  gauge: '<path d="M12 14l4-4"/><path d="M3.5 18a9 9 0 1 1 17 0z"/>',
  message: '<path d="M21 12a8 8 0 0 1-11.6 7.1L4 20l1-4.6A8 8 0 1 1 21 12z"/>',
  languages: '<path d="M4 5h9M8.5 3v2M11 5c-.8 4-3.3 7-6.5 8.5M6 9c1.3 2 3.2 3.4 5.5 4.2"/><path d="m13 21 4-9 4 9M14.5 18h5"/>',
  downloadCloud: '<path d="M7 18a4.5 4.5 0 0 1-.8-8.9A6 6 0 0 1 17.7 8 4.5 4.5 0 0 1 17 17"/><path d="M12 12v8M9 17l3 3 3-3"/>',
  seed: '<path d="M12 21v-8"/><path d="M12 13C12 8 8 5 3 5c0 5 4 8 9 8zM12 13c0-4 3-7 8-7 0 4-3 7-8 7z"/>',
  loader: '<path d="M12 3v3M12 18v3M4.2 7.5l2.6 1.5M17.2 15l2.6 1.5M4.2 16.5l2.6-1.5M17.2 9l2.6-1.5"/>',
  wifiOff: '<path d="M3 3l18 18"/><path d="M8.5 16a5 5 0 0 1 7 0M5 12.5a10 10 0 0 1 4.2-2.4M14.8 10.1A10 10 0 0 1 19 12.5M2 9a15 15 0 0 1 4.5-2.9M12 20h.01"/>',
};

let logoSeq = 0;

// Product mark: a shield holding a download arrow.
export function logoMarkup() {
  const id = `logo-grad-${++logoSeq}`;
  return `<svg class="brand-mark" viewBox="0 0 64 64" aria-hidden="true">
  <defs><linearGradient id="${id}" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#7d88ff"/><stop offset="1" stop-color="#5836d6"/></linearGradient></defs>
  <rect x="2" y="2" width="60" height="60" rx="16" fill="url(#${id})"/>
  <path d="M32 12 17 18v11c0 10 6.4 18.6 15 22 8.6-3.4 15-12 15-22V18z" fill="#ffffff" fill-opacity=".14" stroke="#ffffff" stroke-width="3.4" stroke-linejoin="round"/>
  <path d="M32 23.5v15.5M25.5 33 32 39.5 38.5 33" fill="none" stroke="#ffffff" stroke-width="3.6" stroke-linecap="round" stroke-linejoin="round"/>
</svg>`;
}

export const EMPTY_ART = `<svg class="empty-art" viewBox="0 0 132 132" aria-hidden="true">
  <circle cx="66" cy="66" r="62" fill="currentColor" fill-opacity=".06"/>
  <circle cx="66" cy="66" r="45" fill="currentColor" fill-opacity=".08"/>
  <path d="M66 29 41.5 38.8v17.4c0 15.8 10.4 30.4 24.5 35.8 14.1-5.4 24.5-20 24.5-35.8V38.8z" fill="currentColor" fill-opacity=".16" stroke="currentColor" stroke-width="3" stroke-linejoin="round"/>
  <path d="M66 47.5v25M56.3 63.3 66 73l9.7-9.7" fill="none" stroke="currentColor" stroke-width="3.6" stroke-linecap="round" stroke-linejoin="round"/>
  <circle cx="105" cy="29" r="4" fill="currentColor" fill-opacity=".35"/>
  <circle cx="23" cy="99" r="3" fill="currentColor" fill-opacity=".3"/>
  <circle cx="111" cy="94" r="2.5" fill="currentColor" fill-opacity=".25"/>
  <circle cx="20" cy="36" r="2" fill="currentColor" fill-opacity=".25"/>
</svg>`;

export const HEART_ART = `<svg class="support-heart" viewBox="0 0 96 96" aria-hidden="true">
  <circle cx="48" cy="48" r="46" fill="currentColor" fill-opacity=".10"/>
  <circle cx="48" cy="48" r="32" fill="currentColor" fill-opacity=".10"/>
  <path d="M48 70s-21-12.6-21-28.6A11.4 11.4 0 0 1 48 35.1a11.4 11.4 0 0 1 21 6.3C69 57.4 48 70 48 70z" fill="currentColor"/>
</svg>`;
