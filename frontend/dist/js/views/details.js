// Details panel for the selected transfer: overview, files, peers, trackers.

import { h, icon, setText } from '../dom.js';
import { t } from '../i18n.js';
import { store } from '../store.js';
import { api, errorText } from '../api.js';
import { toast } from '../ui.js';
import { bytes, speed, percent, eta, ratio, dateTime, number } from '../format.js';
import * as actions from '../actions.js';

const TABS = ['overview', 'files', 'peers', 'trackers'];
const REFRESH_MS = 2000;

export function createDetails({ onClose }) {
  let id = null;
  let tab = store.detailsTab;
  let timer = null;
  let seq = 0;
  let staticInfo = null;
  let overviewRefs = null;
  let fileRows = new Map();

  const title = h('div', { class: 'details-title' });
  const tabButtons = TABS.map((k) => h('button', {
    class: 'tab',
    type: 'button',
    role: 'tab',
    'aria-selected': String(k === tab),
    onclick: () => setTab(k),
  }, t(`details.${k}`)));
  const folderBtn = h('button', {
    class: 'btn btn-ghost icon-btn btn-sm',
    type: 'button',
    title: t('details.openFolder'),
    'aria-label': t('details.openFolder'),
    onclick: () => id && actions.openFolder(id),
  }, icon('folder'));
  const magnetBtn = h('button', {
    class: 'btn btn-ghost icon-btn btn-sm',
    type: 'button',
    title: t('menu.copyMagnet'),
    'aria-label': t('menu.copyMagnet'),
    onclick: () => id && actions.copyMagnet(id),
  }, icon('magnet'));
  const closeBtn = h('button', {
    class: 'btn btn-ghost icon-btn btn-sm',
    type: 'button',
    title: t('details.close'),
    'aria-label': t('details.close'),
    onclick: onClose,
  }, icon('x'));
  const body = h('div', { class: 'details-body', role: 'tabpanel' });
  const el = h('section', { class: 'details' },
    h('div', { class: 'details-head' },
      h('div', { class: 'tabs', role: 'tablist' }, tabButtons),
      title,
      h('div', { class: 'row-actions' }, folderBtn, magnetBtn, closeBtn)),
    body);

  function setTorrent(next) {
    if (next === id) return;
    id = next;
    staticInfo = null;
    render();
  }

  function setTab(k) {
    tab = k;
    store.detailsTab = k;
    tabButtons.forEach((b, i) => b.setAttribute('aria-selected', String(TABS[i] === k)));
    render();
  }

  function render() {
    clearTimeout(timer);
    overviewRefs = null;
    fileRows = new Map();
    body.replaceChildren();
    const tor = id && store.byId.get(id);
    setText(title, tor ? tor.name : '');
    if (!tor) return;
    if (tab === 'overview') {
      body.appendChild(buildOverview());
      updateOverview();
    }
    load();
  }

  async function load() {
    clearTimeout(timer);
    if (!id) return;
    const mine = ++seq;
    const section = tab;
    const forId = id;
    try {
      const d = await api.details(forId, section);
      if (mine !== seq || forId !== id || section !== tab) return;
      if (section === 'overview') {
        staticInfo = d;
        paintStatic();
      } else if (section === 'files') {
        paintFiles(d.files);
      } else if (section === 'peers') {
        paintPeers(d.peers);
      } else {
        paintTrackers(d.trackers);
      }
    } catch (err) {
      if (mine === seq) body.replaceChildren(h('div', { class: 'details-empty', text: errorText(err) }));
    }
    if (mine === seq && (section === 'files' || section === 'peers')) {
      timer = setTimeout(load, REFRESH_MS);
    }
  }

  /* ---------------------------------------------------------- Overview */

  function kv(label, value, { wide = false, action = null, mono = false } = {}) {
    const valueEl = h('span', { class: mono ? 'mono selectable' : 'selectable' });
    if (value !== undefined) setText(valueEl, value);
    const node = h('div', { class: wide ? 'kv wide' : 'kv' },
      h('div', { class: 'kv-label', text: label }),
      action ? h('div', { class: 'kv-value with-action' }, valueEl, action) : h('div', { class: 'kv-value' }, valueEl));
    return { node, valueEl };
  }

  function buildOverview() {
    const refs = {};
    refs.pct = h('div', { class: 'big-pct' });
    refs.fill = h('div', { class: 'bar-fill' });
    refs.state = h('div', { class: 'state-label' });
    refs.doneOf = h('div', { class: 'muted num' });
    refs.progressBox = h('div', { class: 'overview-progress' }, refs.pct, h('div', { class: 'bar lg' }, refs.fill), refs.state, refs.doneOf);

    const live = [
      ['downloaded', 'details.downloaded'], ['uploaded', 'details.uploaded'], ['ratio', 'details.ratio'],
      ['down', 'details.downSpeed'], ['up', 'details.upSpeed'], ['eta', 'details.eta'],
      ['peers', 'details.connections'], ['size', 'details.size'], ['total', 'details.totalSize'],
      ['added', 'details.added'], ['completed', 'details.completedAt'],
    ];
    const grid = h('div', { class: 'kv-grid' });
    for (const [key, label] of live) {
      const item = kv(t(label));
      refs[key] = item.valueEl;
      grid.appendChild(item.node);
    }
    refs.pieces = kv(t('details.pieces'));
    grid.appendChild(refs.pieces.node);
    const tor = store.byId.get(id);
    const path = kv(t('details.savePath'), tor.savePath, {
      wide: true,
      action: h('button', { class: 'btn btn-ghost icon-btn btn-sm', type: 'button', title: t('details.openFolder'), 'aria-label': t('details.openFolder'), onclick: () => actions.openFolder(id) }, icon('folder', 'sm')),
    });
    const hash = kv(t('details.infoHash'), tor.id, {
      wide: true,
      mono: true,
      action: h('button', { class: 'btn btn-ghost icon-btn btn-sm', type: 'button', title: t('details.copy'), 'aria-label': t('details.copy'), onclick: () => actions.copyText(tor.id) }, icon('copy', 'sm')),
    });
    grid.append(path.node, hash.node);
    refs.staticHost = h('div', { class: 'kv-grid', style: { 'grid-column': '1 / -1' } });
    grid.appendChild(refs.staticHost);
    overviewRefs = refs;
    return h('div', { class: 'overview' }, refs.progressBox, grid);
  }

  function paintStatic() {
    if (!overviewRefs || !staticInfo) return;
    const d = staticInfo;
    setText(overviewRefs.pieces.valueEl, d.pieces ? t('details.piecesValue', { count: number(d.pieces), size: bytes(d.pieceLength) }) : t('details.unknown'));
    const items = [];
    if (d.infoHashV2) items.push(kv(t('details.infoHashV2'), d.infoHashV2, { wide: true, mono: true }).node);
    if (d.private) items.push(h('div', { class: 'kv wide' }, h('div', { class: 'callout' }, icon('lock'), h('div', null, h('strong', { text: t('details.private') }), h('span', { text: t('details.privateText') })))));
    if (d.createdBy) items.push(kv(t('details.createdBy'), d.createdBy).node);
    if (d.createdAt) items.push(kv(t('details.createdAt'), dateTime(d.createdAt * 1000)).node);
    if (d.comment) {
      const c = kv(t('details.comment'), undefined, { wide: true });
      c.valueEl.className = 'selectable comment-text';
      setText(c.valueEl, d.comment);
      items.push(c.node);
    }
    overviewRefs.staticHost.replaceChildren(...items);
  }

  function updateOverview() {
    const tor = id && store.byId.get(id);
    if (!overviewRefs || !tor) return;
    const r = overviewRefs;
    r.progressBox.dataset.state = tor.state;
    setText(r.pct, percent(tor.progress));
    r.fill.style.setProperty('--p', String(tor.progress));
    setText(r.state, t(`state.${tor.state}`) + (tor.state === 'error' && tor.error ? ` — ${errorText(tor.error)}` : ''));
    setText(r.doneOf, tor.hasMeta ? t('list.of', { done: bytes(tor.done), total: bytes(tor.size) }) : '');
    setText(r.downloaded, bytes(tor.downloaded));
    setText(r.uploaded, bytes(tor.uploaded));
    setText(r.ratio, ratio(tor.ratio));
    setText(r.down, speed(tor.downRate) || '—');
    setText(r.up, speed(tor.upRate) || '—');
    setText(r.eta, eta(tor.eta) || '—');
    setText(r.peers, `${number(tor.peers)} / ${number(tor.seeds)} (${number(tor.knownPeers)})`);
    setText(r.size, tor.hasMeta ? bytes(tor.size) : '—');
    setText(r.total, tor.hasMeta ? bytes(tor.totalSize) : '—');
    setText(r.added, dateTime(tor.addedAt));
    setText(r.completed, tor.completedAt ? dateTime(tor.completedAt) : '—');
  }

  /* ------------------------------------------------------------- Files */

  function paintFiles(files) {
    if (!files.length) {
      body.replaceChildren(h('div', { class: 'details-empty', text: t('details.noFiles') }));
      return;
    }
    let tbody = body.querySelector('tbody');
    if (!tbody) {
      tbody = h('tbody');
      body.replaceChildren(h('table', { class: 'dtable' },
        h('thead', null, h('tr', null,
          h('th', { text: t('details.fileName') }),
          h('th', { class: 't-right', text: t('columns.size') }),
          h('th', { text: t('columns.progress') }),
          h('th', { text: t('details.priority') }))),
        tbody));
    }
    const warn = store.settings && store.settings.warnExecutables !== false;
    for (const f of files) {
      let row = fileRows.get(f.index);
      if (!row) {
        const select = h('select', { class: 'select', 'aria-label': t('details.priority') },
          [0, 1, 2].map((p) => h('option', { value: String(p), text: t(`priority.${p}`) })));
        select.addEventListener('change', async () => {
          try {
            await api.setFilePriority(id, [f.index], Number(select.value));
            load();
          } catch (err) {
            toast({ kind: 'error', title: errorText(err) });
          }
        });
        row = {
          tr: h('tr'),
          fill: h('div', { class: 'bar-fill' }),
          pct: h('span', { class: 'pct num' }),
          select,
        };
        const risky = warn && f.risky;
        row.tr.append(
          h('td', { class: 't-name' }, h('div', { class: 'file-cell', title: risky ? t('details.riskyFile') : f.path },
            risky ? h('span', { class: 'risky-icon' }, icon('fileWarning', 'sm')) : icon('file', 'sm'),
            h('span', { class: 'selectable', text: f.path }))),
          h('td', { class: 't-right num', text: bytes(f.size) }),
          h('td', { class: 't-progress' }, h('div', { class: 'c-progress' }, h('div', { class: 'bar' }, row.fill), row.pct)),
          h('td', null, h('div', { class: 'select-wrap' }, select, icon('chevronDown', 'sm'))));
        fileRows.set(f.index, row);
        tbody.appendChild(row.tr);
      }
      const p = f.done < 0 || f.size === 0 ? (f.done < 0 ? 0 : 1) : f.done / f.size;
      row.fill.style.setProperty('--p', String(p));
      setText(row.pct, f.done < 0 ? t('details.unknown') : percent(p));
      if (document.activeElement !== row.select) row.select.value = String(f.priority);
      row.tr.classList.toggle('skip', f.priority === 0);
    }
  }

  /* ------------------------------------------------------------- Peers */

  function paintPeers(peers) {
    const tor = store.byId.get(id);
    if (!peers.length) {
      const inactive = tor && (tor.state === 'paused' || tor.state === 'completed' || tor.state === 'error');
      body.replaceChildren(h('div', { class: 'details-empty', text: t(inactive ? 'details.noPeersInactive' : 'details.noPeers') }));
      return;
    }
    body.replaceChildren(h('table', { class: 'dtable' },
      h('thead', null, h('tr', null,
        h('th', { text: t('details.address') }),
        h('th', { text: t('details.client') }),
        h('th', { text: t('details.connection') }),
        h('th', { text: t('details.source') }),
        h('th', { text: t('columns.progress') }),
        h('th', { class: 't-right', text: t('columns.down') }),
        h('th', { class: 't-right', text: t('columns.up') }))),
      h('tbody', null, peers.map((p) => h('tr', null,
        h('td', { class: 'mono selectable', text: p.address }),
        h('td', { class: 'truncate', text: p.client || t('details.unknown') }),
        h('td', { text: /udp|utp/i.test(p.network) ? 'uTP' : 'TCP' }),
        h('td', { text: t(`source.${p.source}`).startsWith('source.') ? p.source : t(`source.${p.source}`) }),
        h('td', { class: 't-progress' }, h('div', { class: 'c-progress' }, h('div', { class: 'bar' }, h('div', { class: 'bar-fill', style: { '--p': String(p.progress) } })), h('span', { class: 'pct num', text: percent(p.progress) }))),
        h('td', { class: 't-right num', text: speed(p.downRate) }),
        h('td', { class: 't-right num', text: speed(p.upRate) }))))));
  }

  /* ---------------------------------------------------------- Trackers */

  function paintTrackers(trackers) {
    if (!trackers.length) {
      body.replaceChildren(h('div', { class: 'details-empty', text: t('details.noTrackers') }));
      return;
    }
    body.replaceChildren(h('table', { class: 'dtable' },
      h('thead', null, h('tr', null,
        h('th', { text: t('details.tier') }),
        h('th', { text: 'URL' }),
        h('th', { text: '' }))),
      h('tbody', null, trackers.map((tr) => h('tr', null,
        h('td', { class: 'num', text: String(tr.tier + 1) }),
        h('td', { class: 't-name mono selectable' }, h('div', { class: 'file-cell' }, h('span', { text: tr.url }))),
        h('td', null, tr.skipped ? h('span', { class: 'badge badge-warn', title: t('details.trackerSkippedHint') }, icon('shield', 'sm'), t('details.trackerSkipped')) : null))))));
  }

  return {
    el,
    setTorrent,
    onState() {
      const tor = id && store.byId.get(id);
      if (!tor) return;
      setText(title, tor.name);
      if (tab === 'overview') updateOverview();
    },
    destroy() {
      clearTimeout(timer);
      seq++;
    },
  };
}
