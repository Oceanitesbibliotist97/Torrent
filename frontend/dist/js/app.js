// Application shell: boot, sidebar, status bar, navigation, shortcuts, drag and drop.

import { h, icon, staticSvg, setText, setAttr, isEditable } from './dom.js';
import { t, loadLanguage, detectLanguage, onLanguageChange } from './i18n.js';
import { store, on, emit, setState, FILTERS } from './store.js';
import { api, runtime, errorText } from './api.js';
import { toast, openModal, modalOpen, menuOpen, closeMenu } from './ui.js';
import { bytes, number } from './format.js';
import { logoMarkup } from './icons.js';
import { createTransfersView } from './views/transfers.js';
import { createPrivacyView, protection } from './views/privacy.js';
import { createSettingsView } from './views/settings.js';
import { createSupportView } from './views/support.js';
import { createAboutView } from './views/about.js';
import { openAddFiles, openAddMagnet, handleOpenArgs } from './views/add.js';

const VIEWS = {
  transfers: createTransfersView,
  privacy: createPrivacyView,
  settings: createSettingsView,
  support: createSupportView,
  about: createAboutView,
};

const FILTER_NAV = [
  { id: 'all', icon: 'list' },
  { id: 'downloading', icon: 'arrowDown' },
  { id: 'seeding', icon: 'arrowUp' },
  { id: 'completed', icon: 'checkCircle' },
  { id: 'paused', icon: 'pause' },
  { id: 'errors', icon: 'alertCircle' },
];

const APP_NAV = [
  { id: 'privacy', icon: 'shield' },
  { id: 'settings', icon: 'sliders' },
  { id: 'support', icon: 'heart' },
  { id: 'about', icon: 'info' },
];

const appEl = document.getElementById('app');
const darkQuery = window.matchMedia('(prefers-color-scheme: dark)');

let current = null;
let mainEl = null;
let sidebar = null;
let status = null;
let dropOverlay = null;

/* ------------------------------------------------------------------ Theme */

function applyTheme(theme) {
  const effective = theme === 'system' ? (darkQuery.matches ? 'dark' : 'light') : theme;
  document.documentElement.dataset.theme = effective;
  runtime.windowTheme(theme);
}

darkQuery.addEventListener('change', () => {
  if (store.settings && store.settings.theme === 'system') applyTheme('system');
});

/* ---------------------------------------------------------------- Sidebar */

function buildSidebar() {
  const counts = new Map();
  const filterButtons = FILTER_NAV.map((item) => {
    const count = h('span', { class: 'nav-count' });
    counts.set(item.id, count);
    return h('button', {
      class: 'nav-item',
      type: 'button',
      dataset: { filter: item.id },
      onclick: () => {
        store.filter = item.id;
        navigate('transfers');
        emit('filter');
        paintNav();
      },
    }, icon(item.icon), h('span', { class: 'nav-text', text: t(`nav.${item.id}`) }), count);
  });
  const appButtons = APP_NAV.map((item) => h('button', {
    class: 'nav-item',
    type: 'button',
    dataset: { view: item.id },
    onclick: () => navigate(item.id),
  }, icon(item.icon, item.id === 'support' ? 'nav-heart' : ''), h('span', { class: 'nav-text', text: t(`nav.${item.id}`) })));

  const shieldIcon = h('div', { class: 'shield-icon' });
  const shieldTitle = h('div', { class: 'shield-title' });
  const shieldSub = h('div', { class: 'shield-sub truncate' });
  const shield = h('button', { class: 'shield-card', type: 'button', onclick: () => navigate('privacy') },
    shieldIcon, h('div', { class: 'grow' }, shieldTitle, shieldSub));

  sidebar = { counts, filterButtons, appButtons, shield, shieldIcon, shieldTitle, shieldSub, iconName: '' };
  return h('aside', { class: 'sidebar' },
    h('div', { class: 'brand' },
      staticSvg(logoMarkup()),
      h('div', null,
        h('div', { class: 'brand-name', text: store.boot.appName }),
        h('div', { class: 'brand-sub', text: t('app.tagline') }))),
    h('nav', { class: 'nav', 'aria-label': t('nav.transfers') }, h('div', { class: 'nav-label', text: t('nav.transfers') }), filterButtons),
    h('nav', { class: 'nav', 'aria-label': t('nav.app') }, h('div', { class: 'nav-label', text: t('nav.app') }), appButtons),
    h('div', { class: 'sidebar-spacer' }),
    shield);
}

function paintNav() {
  for (const b of sidebar.filterButtons) {
    setAttr(b, 'aria-current', store.view === 'transfers' && store.filter === b.dataset.filter ? 'page' : null);
  }
  for (const b of sidebar.appButtons) setAttr(b, 'aria-current', store.view === b.dataset.view ? 'page' : null);
}

function paintCounts() {
  for (const [id, el] of sidebar.counts) {
    const n = store.torrents.filter(FILTERS[id]).length;
    setText(el, n ? number(n) : '');
  }
}

function paintShield() {
  const p = protection(store.global, store.settings);
  const net = (store.global && store.global.network) || {};
  sidebar.shield.dataset.level = p.level;
  if (sidebar.iconName !== p.icon) {
    sidebar.shieldIcon.replaceChildren(icon(p.icon, 'lg'));
    sidebar.iconName = p.icon;
  }
  setText(sidebar.shieldTitle, t(`status.${p.status}`));
  let sub;
  if (p.status === 'vpn') sub = net.interface;
  else if (p.status === 'proxy') sub = net.proxy;
  else if (p.status === 'direct') sub = t('privacy.hero.setup');
  else sub = t('privacy.hero.reconnect');
  setText(sidebar.shieldSub, sub || '');
}

/* ------------------------------------------------------------- Status bar */

function buildStatusbar() {
  status = {
    shield: h('button', { class: 'status-shield', type: 'button', onclick: () => navigate('privacy') }),
    down: h('span', { class: 'status-item down num' }),
    up: h('span', { class: 'status-item up num' }),
    dht: h('span', { class: 'status-item num' }),
    port: h('span', { class: 'status-item num' }),
    free: h('span', { class: 'status-item num' }),
    signature: '',
  };
  return h('footer', { class: 'statusbar' },
    status.shield, status.down, status.up,
    h('span', { class: 'status-spacer' }),
    status.dht, status.port, status.free);
}

function paintStatus() {
  const g = store.global;
  if (!g) return;
  const p = protection(g, store.settings);
  const signature = `${p.level}|${p.status}`;
  if (signature !== status.signature) {
    status.signature = signature;
    status.shield.dataset.level = p.level;
    status.shield.replaceChildren(icon(p.icon), h('span', { text: t(`status.${p.status}`) }));
  }
  status.down.replaceChildren(icon('arrowDown'), t('units.perSecond', { value: bytes(g.downRate) }));
  status.up.replaceChildren(icon('arrowUp'), t('units.perSecond', { value: bytes(g.upRate) }));
  setText(status.dht, g.network.dht ? t('status.dht', { n: number(g.dhtNodes) }) : t('status.dhtOff'));
  setText(status.port, g.network.listenPort ? t('status.port', { port: g.network.listenPort }) : '');
  setText(status.free, g.freeSpace >= 0 ? t('status.free', { size: bytes(g.freeSpace) }) : '');
}

/* ------------------------------------------------------------- Navigation */

function confirmDiscard() {
  return new Promise((resolve) => {
    const stay = h('button', { class: 'btn btn-secondary', type: 'button', text: t('common.cancel'), 'data-autofocus': '' });
    const discard = h('button', { class: 'btn btn-danger', type: 'button', text: t('settings.discard') });
    const modal = openModal({
      title: t('settings.unsaved'),
      body: h('p', { class: 'muted', style: { margin: '0' }, text: t('settings.networkRestart') }),
      footer: [stay, discard],
      onClose: (result) => resolve(result === true),
    });
    stay.addEventListener('click', () => modal.close(false));
    discard.addEventListener('click', () => modal.close(true));
  });
}

async function navigate(view, opts = {}) {
  if (current && store.view === view && !opts.force && !opts.section) {
    paintNav();
    return;
  }
  if (current && current.isDirty && current.isDirty() && store.view !== view && !(await confirmDiscard())) return;
  closeMenu();
  if (current && current.destroy) current.destroy();
  store.view = view;
  current = VIEWS[view]({ navigate, applyTheme, section: opts.section });
  mainEl.replaceChildren(current.el);
  paintNav();
  emit('view');
}

/* ---------------------------------------------------------------- Shell */

function buildShell() {
  mainEl = h('main', { class: 'main' });
  dropOverlay = h('div', { class: 'drop-overlay', hidden: true },
    h('div', { class: 'drop-zone' }, icon('downloadCloud'), h('strong', { text: t('drop.title') }), h('span', { text: t('drop.text') })));
  appEl.replaceChildren(buildSidebar(), mainEl, buildStatusbar(), dropOverlay);
  paintCounts();
  paintShield();
  paintStatus();
}

function rebuildForLanguage() {
  const view = store.view;
  if (current && current.destroy && view === 'transfers') current.destroy();
  const keep = view !== 'transfers' ? current : null;
  buildShell();
  if (keep) {
    mainEl.replaceChildren(keep.el);
    current = keep;
    paintNav();
  } else {
    current = null;
    navigate(view, { force: true });
  }
  emit('lang');
}

/* ------------------------------------------------------------ Shortcuts */

async function pasteMagnet() {
  const magnet = await api.clipboardMagnet().catch(() => '');
  openAddMagnet(magnet);
}

function onKeyDown(e) {
  const mod = e.ctrlKey || e.metaKey;
  const key = e.key.length === 1 ? e.key.toLowerCase() : e.key;
  // Browser behaviours that make no sense in a desktop app.
  if (key === 'F5' || (mod && (key === 'r' || key === 'p' || key === 's'))) {
    e.preventDefault();
    return;
  }
  if (modalOpen() || menuOpen()) return;
  if (mod && key === 'o') {
    e.preventDefault();
    openAddFiles();
  } else if (mod && key === 'u') {
    e.preventDefault();
    openAddMagnet();
  } else if (mod && key === ',') {
    e.preventDefault();
    navigate('settings');
  } else if (mod && key === 'f') {
    e.preventDefault();
    navigate('transfers').then(() => current.focusSearch && current.focusSearch());
  } else if (isEditable(e.target)) {
    // Let text fields handle everything else.
  } else if (mod && key === 'v') {
    e.preventDefault();
    pasteMagnet();
  } else if (store.view === 'transfers' && current && current.handleKey) {
    current.handleKey(e);
  }
}

/* ---------------------------------------------------------- Drag & drop */

let dragDepth = 0;

function draggingFiles(e) {
  return !!e.dataTransfer && [...e.dataTransfer.types].some((type) => type === 'Files' || type === 'text/uri-list' || type === 'text/plain');
}

function hideDrop() {
  dragDepth = 0;
  dropOverlay.hidden = true;
}

function wireDragAndDrop() {
  window.addEventListener('dragenter', (e) => {
    if (!draggingFiles(e) || modalOpen()) return;
    dragDepth++;
    dropOverlay.hidden = false;
  });
  window.addEventListener('dragleave', () => {
    dragDepth = Math.max(0, dragDepth - 1);
    if (!dragDepth) dropOverlay.hidden = true;
  });
  window.addEventListener('dragover', (e) => {
    if (draggingFiles(e)) e.preventDefault();
  });
  window.addEventListener('drop', (e) => {
    e.preventDefault();
    hideDrop();
    const text = e.dataTransfer ? e.dataTransfer.getData('text/plain').trim() : '';
    if (/^magnet:\?/i.test(text)) openAddMagnet(text);
  });
  runtime.onFileDrop((paths) => {
    hideDrop();
    const torrents = paths.filter((p) => /\.torrent$/i.test(p));
    if (torrents.length) openAddFiles(torrents);
    else if (paths.length) toast({ kind: 'warning', title: t('drop.text') });
  });
}

/* ------------------------------------------------------------------ Boot */

async function boot() {
  appEl.replaceChildren(h('div', { class: 'boot' }, h('div', { class: 'spinner' })));
  let bootData;
  try {
    bootData = await api.bootstrap();
  } catch (err) {
    appEl.replaceChildren(h('div', { class: 'boot' }, h('p', { text: `Backend unavailable: ${err.message}` })));
    return;
  }
  store.boot = bootData;
  store.settings = bootData.settings;
  applyTheme(store.settings.theme);

  let language = store.settings.language;
  if (!language) {
    language = detectLanguage();
    try {
      store.settings = await api.saveSettings({ ...store.settings, language });
    } catch {
      // Still usable with the detected language for this session.
    }
  }
  await loadLanguage(language);
  document.title = bootData.appName;

  buildShell();
  await navigate('transfers');
  appEl.removeAttribute('aria-busy');

  on('state', () => {
    paintCounts();
    paintShield();
    paintStatus();
  });
  on('settings', () => {
    paintShield();
    status.signature = '';
    paintStatus();
  });
  onLanguageChange(rebuildForLanguage);

  try {
    setState(await api.state());
  } catch {
    // The engine may be offline; its error is reported below.
  }
  runtime.on('state', (s) => setState(s));
  runtime.on('notice', (n) => toast({ kind: n.kind, title: t(n.key), detail: n.detail || '' }));
  runtime.on('open-args', (args) => handleOpenArgs(args));
  wireDragAndDrop();
  document.addEventListener('keydown', onKeyDown);
  document.addEventListener('dragstart', (e) => {
    if (!isEditable(e.target)) e.preventDefault();
  });

  if (bootData.engineError) toast({ kind: 'error', title: errorText(bootData.engineError), timeout: 0 });
  if (bootData.settingsError) toast({ kind: 'warning', title: errorText(bootData.settingsError) });
  if (bootData.args && bootData.args.length) handleOpenArgs(bootData.args);
}

boot();
