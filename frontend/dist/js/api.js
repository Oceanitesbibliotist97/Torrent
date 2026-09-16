// Bridge to the Go backend (Wails bindings) with uniform error handling.
// All calls go through the in-process IPC channel; there is no local HTTP
// server another program or website could talk to.

import { t, has } from './i18n.js';

export class AppError extends Error {
  constructor(code, detail = '') {
    super(detail || code);
    this.code = code;
    this.detail = detail;
  }
}

export function toAppError(err) {
  if (err instanceof AppError) return err;
  const message = typeof err === 'string' ? err : err && err.message ? err.message : String(err);
  const m = /^([a-zA-Z]+)(?::\s([\s\S]*))?$/.exec(message);
  if (m && has(`errors.${m[1]}`)) return new AppError(m[1], m[2] || '');
  return new AppError('unknown', message);
}

export function errorText(err) {
  const e = toAppError(err);
  return t(`errors.${e.code}`);
}

function call(method, ...args) {
  const backend = window.go && window.go.main && window.go.main.App;
  if (!backend || typeof backend[method] !== 'function') {
    return Promise.reject(new AppError('internal', 'backend unavailable'));
  }
  return backend[method](...args).catch((err) => {
    throw toAppError(err);
  });
}

export const api = {
  bootstrap: () => call('GetBootstrap'),
  state: () => call('GetState'),
  saveSettings: (settings) => call('SaveSettings', settings),
  interfaces: () => call('ListInterfaces'),
  pickTorrentFiles: (title) => call('PickTorrentFiles', title),
  pickFolder: (title, current) => call('PickFolder', title, current || ''),
  pickBlocklist: (title) => call('PickBlocklist', title),
  freeSpace: (path) => call('FreeSpace', path),
  clipboardMagnet: () => call('ClipboardMagnet'),
  previewTorrent: (path) => call('PreviewTorrent', path),
  addTorrentFile: (path, opts) => call('AddTorrentFile', path, opts),
  previewMagnet: (uri) => call('PreviewMagnet', uri),
  addMagnet: (uri, opts) => call('AddMagnet', uri, opts),
  pause: (ids) => call('Pause', ids),
  resume: (ids) => call('Resume', ids),
  remove: (ids, deleteFiles) => call('Remove', ids, deleteFiles),
  recheck: (id) => call('Recheck', id),
  setFilePriority: (id, indices, priority) => call('SetFilePriority', id, indices, priority),
  details: (id, section) => call('GetDetails', id, section),
  openFolder: (id) => call('OpenFolder', id),
  magnetLink: (id) => call('GetMagnetLink', id),
  reconnect: () => call('Reconnect'),
  openLink: (id) => call('OpenLink', id),
  openDataFolder: () => call('OpenDataFolder'),
};

function rt() {
  return window.runtime || {};
}

export const runtime = {
  on(event, fn) {
    const off = rt().EventsOn ? rt().EventsOn(event, fn) : null;
    return typeof off === 'function' ? off : () => {};
  },
  onFileDrop(fn) {
    if (rt().OnFileDrop) rt().OnFileDrop((x, y, paths) => fn(paths || []), false);
  },
  copy(text) {
    if (rt().ClipboardSetText) return rt().ClipboardSetText(text);
    return navigator.clipboard ? navigator.clipboard.writeText(text) : Promise.resolve();
  },
  windowTheme(theme) {
    const r = rt();
    if (theme === 'dark' && r.WindowSetDarkTheme) r.WindowSetDarkTheme();
    else if (theme === 'light' && r.WindowSetLightTheme) r.WindowSetLightTheme();
    else if (r.WindowSetSystemDefaultTheme) r.WindowSetSystemDefaultTheme();
  },
};
