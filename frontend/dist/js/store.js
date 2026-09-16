// Central UI state with a minimal publish/subscribe mechanism.

export const store = {
  boot: null,
  settings: null,
  torrents: [],
  byId: new Map(),
  global: null,
  view: 'transfers',
  filter: 'all',
  search: '',
  sort: { key: 'addedAt', dir: 'desc' },
  selection: new Set(),
  anchor: null,
  detailsTab: 'overview',
  detailsHeight: 300,
};

const topics = new Map();

export function on(topic, fn) {
  if (!topics.has(topic)) topics.set(topic, new Set());
  topics.get(topic).add(fn);
  return () => topics.get(topic).delete(fn);
}

export function emit(topic, payload) {
  const subs = topics.get(topic);
  if (subs) for (const fn of [...subs]) fn(payload);
}

export const FILTERS = {
  all: () => true,
  downloading: (t) => t.state === 'downloading' || t.state === 'stalled' || t.state === 'metadata' || t.state === 'checking' || (t.state === 'offline' && t.progress < 1),
  seeding: (t) => t.state === 'seeding',
  completed: (t) => t.hasMeta && t.progress >= 1,
  paused: (t) => t.state === 'paused',
  errors: (t) => t.state === 'error',
};

export function setState(state) {
  store.torrents = state.torrents || [];
  store.global = state.global || null;
  store.byId = new Map(store.torrents.map((t) => [t.id, t]));
  let changed = false;
  for (const id of store.selection) {
    if (!store.byId.has(id)) {
      store.selection.delete(id);
      changed = true;
    }
  }
  emit('state');
  if (changed) emit('selection');
}

export function visibleTorrents() {
  const filter = FILTERS[store.filter] || FILTERS.all;
  const query = store.search.trim().toLocaleLowerCase();
  const list = store.torrents.filter((t) => filter(t) && (!query || t.name.toLocaleLowerCase().includes(query)));
  const { key, dir } = store.sort;
  const sign = dir === 'asc' ? 1 : -1;
  list.sort((a, b) => {
    const av = a[key];
    const bv = b[key];
    let c = typeof av === 'string' ? av.localeCompare(bv, undefined, { numeric: true, sensitivity: 'base' }) : (av > bv) - (av < bv);
    if (key === 'eta') {
      // Unknown ETAs sort last in both directions.
      if (av < 0 && bv >= 0) return 1;
      if (bv < 0 && av >= 0) return -1;
    }
    if (c === 0) c = a.addedAt - b.addedAt;
    return c * sign;
  });
  return list;
}

export function selectedTorrents() {
  return [...store.selection].map((id) => store.byId.get(id)).filter(Boolean);
}

export function setSelection(ids, anchor) {
  store.selection = new Set(ids);
  if (anchor !== undefined) store.anchor = anchor;
  emit('selection');
}

export function setView(view) {
  if (store.view === view) return;
  store.view = view;
  emit('view');
}
