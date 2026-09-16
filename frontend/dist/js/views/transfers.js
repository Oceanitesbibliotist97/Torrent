// Transfers view: toolbar, sortable list with keyboard multi-select,
// context menu, empty states and the resizable details panel.

import { h, icon, setText, setAttr, staticSvg, debounce } from '../dom.js';
import { t } from '../i18n.js';
import { store, on, visibleTorrents, selectedTorrents, setSelection } from '../store.js';
import { errorText } from '../api.js';
import { bytes, speed, percent, eta, ratio, shortDate } from '../format.js';
import { openMenu } from '../ui.js';
import { EMPTY_ART } from '../icons.js';
import * as actions from '../actions.js';
import { openAddFiles, openAddMagnet, openRemoveDialog } from './add.js';
import { createDetails } from './details.js';

const STATE_ICON = {
  downloading: 'arrowDown',
  stalled: 'clock',
  metadata: 'magnet',
  seeding: 'arrowUp',
  completed: 'check',
  paused: 'pause',
  checking: 'refresh',
  error: 'alertCircle',
  offline: 'wifiOff',
};

const COLUMNS = [
  { cls: 'c-name', label: 'columns.name', sort: 'name' },
  { cls: 'c-size c-right', label: 'columns.size', sort: 'size' },
  { cls: 'c-progress', label: 'columns.progress', sort: 'progress' },
  { cls: 'c-down c-right', label: 'columns.down', sort: 'downRate' },
  { cls: 'c-up c-right', label: 'columns.up', sort: 'upRate' },
  { cls: 'c-eta c-right', label: 'columns.eta', sort: 'eta' },
  { cls: 'c-ratio c-right', label: 'columns.ratio', sort: 'ratio' },
  { cls: 'c-added c-right', label: 'columns.added', sort: 'addedAt' },
];

const DETAILS_KEY = 'ui.detailsHeight';

function loadDetailsHeight() {
  try {
    const v = Number(localStorage.getItem(DETAILS_KEY));
    return Number.isFinite(v) && v >= 160 ? v : 300;
  } catch {
    return 300;
  }
}

export function createTransfersView() {
  const rows = new Map();
  store.detailsHeight = loadDetailsHeight();

  /* ----------------------------------------------------------- Toolbar */

  const addBtn = h('button', { class: 'btn btn-primary', type: 'button', 'aria-haspopup': 'menu' },
    icon('plus', 'sm'), t('toolbar.add'), icon('chevronDown', 'sm'));
  addBtn.addEventListener('click', () => openMenu([
    { label: t('toolbar.addFile'), icon: 'file', shortcut: 'Ctrl+O', action: () => openAddFiles() },
    { label: t('toolbar.addMagnet'), icon: 'magnet', shortcut: 'Ctrl+U', action: () => openAddMagnet() },
  ], { anchor: addBtn }));

  const toolButton = (iconName, label, onClick) => h('button', {
    class: 'btn btn-ghost icon-btn',
    type: 'button',
    title: label,
    'aria-label': label,
    disabled: true,
    onclick: onClick,
  }, icon(iconName));
  const resumeBtn = toolButton('play', t('toolbar.resume'), () => actions.resume(selectedTorrents().filter((x) => !actions.canPause(x)).map((x) => x.id)));
  const pauseBtn = toolButton('pause', t('toolbar.pause'), () => actions.pause(selectedTorrents().filter(actions.canPause).map((x) => x.id)));
  const removeBtn = toolButton('trash', t('toolbar.remove'), () => openRemoveDialog(selectedTorrents()));
  const selectionInfo = h('span', { class: 'toolbar-info' });

  const searchInput = h('input', { class: 'input', type: 'search', placeholder: t('toolbar.search'), 'aria-label': t('toolbar.search'), value: store.search });
  searchInput.addEventListener('input', debounce(() => {
    store.search = searchInput.value;
    renderList();
  }, 80));
  searchInput.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && searchInput.value) {
      e.stopPropagation();
      searchInput.value = '';
      store.search = '';
      renderList();
    } else if (e.key === 'ArrowDown') {
      e.preventDefault();
      listBody.focus();
    }
  });

  const toolbar = h('header', { class: 'toolbar' },
    addBtn,
    h('span', { class: 'toolbar-sep' }),
    resumeBtn, pauseBtn, removeBtn,
    selectionInfo,
    h('span', { class: 'toolbar-spacer' }),
    h('div', { class: 'search' }, icon('search'), searchInput));

  /* -------------------------------------------------------------- List */

  const sortButtons = [];
  const listHead = h('div', { class: 'list-head', role: 'row' }, COLUMNS.map((col) => {
    const btn = h('button', { class: 'sort-btn', type: 'button', onclick: () => sortBy(col.sort) }, h('span', { text: t(col.label) }), icon('chevronDown'));
    sortButtons.push({ col, btn });
    return h('div', { class: `cell ${col.cls}`, role: 'columnheader' }, btn);
  }));
  const listBody = h('div', { class: 'list-body', role: 'rowgroup', tabIndex: 0, 'aria-label': t('list.label') });
  const empty = h('div', { class: 'empty', hidden: true });
  const list = h('div', { class: 'list', role: 'grid', 'aria-multiselectable': 'true', 'aria-label': t('list.label') }, listHead, listBody, empty);

  function sortBy(key) {
    if (store.sort.key === key) store.sort.dir = store.sort.dir === 'asc' ? 'desc' : 'asc';
    else store.sort = { key, dir: key === 'name' ? 'asc' : 'desc' };
    renderList();
  }

  function paintSort() {
    for (const { col, btn } of sortButtons) {
      const active = store.sort.key === col.sort;
      setAttr(btn, 'aria-sort', active ? (store.sort.dir === 'asc' ? 'ascending' : 'descending') : null);
      btn.lastElementChild.replaceWith(icon(active && store.sort.dir === 'asc' ? 'chevronUp' : 'chevronDown'));
    }
  }

  function createRow(id) {
    const r = {
      dot: h('span', { class: 'state-dot' }),
      name: h('span', { class: 'name-text' }),
      risky: h('span', { class: 'name-flag risky', title: t('details.riskyFile'), hidden: true }, icon('alertTriangle')),
      priv: h('span', { class: 'name-flag private', title: t('details.private'), hidden: true }, icon('lock')),
      stateLabel: h('span', { class: 'state-label' }),
      sub: h('span'),
      size: h('div', { class: 'cell c-size c-right', role: 'gridcell' }),
      fill: h('div', { class: 'bar-fill' }),
      pct: h('span', { class: 'pct num' }),
      down: h('div', { class: 'cell c-down c-right', role: 'gridcell' }),
      up: h('div', { class: 'cell c-up c-right', role: 'gridcell' }),
      eta: h('div', { class: 'cell c-eta c-right', role: 'gridcell' }),
      ratio: h('div', { class: 'cell c-ratio c-right', role: 'gridcell' }),
      added: h('div', { class: 'cell c-added c-right dim', role: 'gridcell' }),
      state: '',
      progress: -1,
    };
    r.el = h('div', { class: 'row', role: 'row', id: `row-${id}`, dataset: { id } },
      h('div', { class: 'cell c-name', role: 'gridcell' },
        r.dot,
        h('div', { class: 'name-block' },
          h('div', { class: 'name-line' }, r.name, r.risky, r.priv),
          h('div', { class: 'sub-line' }, r.stateLabel, r.sub))),
      r.size,
      h('div', { class: 'cell c-progress', role: 'gridcell' }, h('div', { class: 'bar' }, r.fill), r.pct),
      r.down, r.up, r.eta, r.ratio, r.added);
    return r;
  }

  function subline(tor) {
    const of = tor.hasMeta ? t('list.of', { done: bytes(tor.done), total: bytes(tor.size) }) : '';
    const peers = `${t('list.peers', { n: tor.peers })} (${t('list.seeds', { n: tor.seeds })})`;
    switch (tor.state) {
      case 'downloading':
      case 'stalled':
        return [of, peers].filter(Boolean).join(' · ');
      case 'metadata':
        return t('list.peers', { n: tor.peers });
      case 'seeding':
        return [bytes(tor.size), t('list.peers', { n: tor.peers })].join(' · ');
      case 'completed':
        return bytes(tor.size);
      case 'error':
        return errorText(tor.error) + (tor.error && tor.error.includes(': ') ? ` ${tor.error.slice(tor.error.indexOf(': ') + 2)}` : '');
      default:
        return of;
    }
  }

  function updateRow(r, tor) {
    if (r.state !== tor.state) {
      r.el.dataset.state = tor.state;
      r.dot.replaceChildren(icon(STATE_ICON[tor.state] || 'info'));
      r.state = tor.state;
    }
    setText(r.name, tor.name);
    setAttr(r.name, 'title', tor.name);
    r.risky.hidden = !(tor.risky && store.settings && store.settings.warnExecutables);
    r.priv.hidden = !tor.private;
    setText(r.stateLabel, t(`state.${tor.state}`));
    const sub = subline(tor);
    setText(r.sub, sub ? ` · ${sub}` : '');
    setText(r.size, tor.hasMeta ? bytes(tor.size) : '—');
    if (r.progress !== tor.progress) {
      r.fill.style.setProperty('--p', String(tor.progress));
      r.progress = tor.progress;
    }
    setText(r.pct, percent(tor.progress));
    setText(r.down, speed(tor.downRate));
    setText(r.up, speed(tor.upRate));
    setText(r.eta, eta(tor.eta));
    setText(r.ratio, ratio(tor.ratio));
    setText(r.added, shortDate(tor.addedAt));
    setAttr(r.el, 'aria-selected', store.selection.has(tor.id) ? 'true' : 'false');
  }

  let visible = [];

  function renderList() {
    visible = visibleTorrents();
    let prev = null;
    const shown = new Set();
    for (const tor of visible) {
      let r = rows.get(tor.id);
      if (!r) {
        r = createRow(tor.id);
        rows.set(tor.id, r);
      }
      updateRow(r, tor);
      shown.add(tor.id);
      const expected = prev ? prev.nextSibling : listBody.firstChild;
      if (expected !== r.el) listBody.insertBefore(r.el, expected);
      prev = r.el;
    }
    for (const [id, r] of rows) {
      if (!shown.has(id)) {
        r.el.remove();
        if (!store.byId.has(id)) rows.delete(id);
      }
    }
    listBody.hidden = visible.length === 0;
    listHead.hidden = store.torrents.length === 0;
    empty.hidden = visible.length > 0;
    if (!visible.length) renderEmpty();
    paintSort();
    paintToolbar();
  }

  let emptyKind = '';
  function renderEmpty() {
    const kind = store.torrents.length === 0 ? 'first' : 'filter';
    if (kind === emptyKind && empty.childElementCount) return;
    emptyKind = kind;
    if (kind === 'first') {
      empty.className = 'empty';
      empty.replaceChildren(
        staticSvg(EMPTY_ART),
        h('h2', { text: t('empty.title') }),
        h('p', { text: t('empty.text') }),
        h('div', { class: 'empty-actions' },
          h('button', { class: 'btn btn-primary btn-lg', type: 'button', onclick: () => openAddFiles() }, icon('file'), t('empty.openFile')),
          h('button', { class: 'btn btn-secondary btn-lg', type: 'button', onclick: () => openAddMagnet() }, icon('magnet'), t('empty.pasteMagnet'))),
        h('div', { class: 'empty-note' }, icon('shieldCheck', 'sm'), t('empty.privacyNote')));
    } else {
      empty.className = 'empty compact';
      empty.replaceChildren(
        staticSvg(EMPTY_ART),
        h('h2', { text: t('empty.noMatchesTitle') }),
        h('p', { text: t('empty.noMatchesText') }));
    }
  }

  function paintToolbar() {
    const sel = selectedTorrents();
    resumeBtn.disabled = !sel.some((x) => !actions.canPause(x));
    pauseBtn.disabled = !sel.some(actions.canPause);
    removeBtn.disabled = sel.length === 0;
    setText(selectionInfo, sel.length > 1 ? t('toolbar.selected', { n: sel.length }) : '');
  }

  /* --------------------------------------------------------- Selection */

  function select(id, e) {
    const ids = visible.map((x) => x.id);
    if (e && e.shiftKey && store.anchor && ids.includes(store.anchor)) {
      const a = ids.indexOf(store.anchor);
      const b = ids.indexOf(id);
      const range = ids.slice(Math.min(a, b), Math.max(a, b) + 1);
      setSelection(e.ctrlKey ? [...store.selection, ...range] : range);
    } else if (e && (e.ctrlKey || e.metaKey)) {
      const next = new Set(store.selection);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      setSelection([...next], id);
    } else {
      setSelection([id], id);
    }
    const r = rows.get(id);
    if (r) {
      setAttr(listBody, 'aria-activedescendant', r.el.id);
      r.el.scrollIntoView({ block: 'nearest' });
    }
  }

  listBody.addEventListener('mousedown', (e) => {
    const rowEl = e.target.closest('.row');
    if (!rowEl) return;
    if (e.button === 2 && store.selection.has(rowEl.dataset.id)) return;
    if (e.button === 0 || e.button === 2) select(rowEl.dataset.id, e.button === 0 ? e : null);
  });
  listBody.addEventListener('dblclick', (e) => {
    const rowEl = e.target.closest('.row');
    if (rowEl) actions.openFolder(rowEl.dataset.id);
  });
  listBody.addEventListener('contextmenu', (e) => {
    e.preventDefault();
    if (e.target.closest('.row')) contextMenu(e.clientX, e.clientY);
  });

  function moveFocus(delta, e) {
    if (!visible.length) return;
    const ids = visible.map((x) => x.id);
    const current = store.anchor && ids.includes(store.anchor) ? store.anchor : null;
    let index = current ? ids.indexOf(current) + delta : delta > 0 ? 0 : ids.length - 1;
    index = Math.max(0, Math.min(ids.length - 1, index));
    if (e.shiftKey) {
      const anchor = store.anchor;
      select(ids[index], { shiftKey: true, ctrlKey: false });
      store.anchor = anchor;
      // Keep extending from the original anchor while the cursor moves.
      store.cursor = ids[index];
    } else {
      select(ids[index], null);
    }
  }

  listBody.addEventListener('keydown', (e) => {
    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault();
        moveFocus(1, e);
        break;
      case 'ArrowUp':
        e.preventDefault();
        moveFocus(-1, e);
        break;
      case 'Home':
        e.preventDefault();
        if (visible.length) select(visible[0].id, null);
        break;
      case 'End':
        e.preventDefault();
        if (visible.length) select(visible[visible.length - 1].id, null);
        break;
      case 'ContextMenu': {
        e.preventDefault();
        const r = store.anchor && rows.get(store.anchor);
        const box = (r ? r.el : listBody).getBoundingClientRect();
        contextMenu(box.left + 40, box.top + box.height / 2);
        break;
      }
      case 'Enter':
        if (store.selection.size === 1) {
          e.preventDefault();
          actions.openFolder([...store.selection][0]);
        }
        break;
      default:
    }
  });

  function contextMenu(x, y) {
    const sel = selectedTorrents();
    if (!sel.length) return;
    const single = sel.length === 1 ? sel[0] : null;
    const inactive = sel.filter((s) => !actions.canPause(s));
    const active = sel.filter(actions.canPause);
    openMenu([
      { label: t('menu.resume'), icon: 'play', disabled: !inactive.length, action: () => actions.resume(inactive.map((s) => s.id)) },
      { label: t('menu.pause'), icon: 'pause', shortcut: 'Space', disabled: !active.length, action: () => actions.pause(active.map((s) => s.id)) },
      'sep',
      { label: t('menu.openFolder'), icon: 'folder', disabled: !single, action: () => actions.openFolder(single.id) },
      { label: t('menu.copyMagnet'), icon: 'magnet', disabled: !single, action: () => actions.copyMagnet(single.id) },
      { label: t('menu.copyHash'), icon: 'copy', disabled: !single, action: () => actions.copyText(single.id) },
      { label: t('menu.recheck'), icon: 'refresh', disabled: !single || !single.hasMeta || !actions.canPause(single), action: () => actions.recheck(single.id) },
      'sep',
      { label: t('menu.remove'), icon: 'trash', shortcut: 'Del', action: () => openRemoveDialog(sel) },
      { label: t('menu.removeWithFiles'), icon: 'trash', danger: true, shortcut: 'Shift+Del', action: () => openRemoveDialog(sel, true) },
    ], { x, y });
  }

  /* ----------------------------------------------------------- Details */

  const splitter = h('div', { class: 'splitter', role: 'separator', 'aria-orientation': 'horizontal', hidden: true });
  const details = createDetails({ onClose: () => setSelection([]) });
  details.el.hidden = true;
  details.el.style.height = `${store.detailsHeight}px`;

  splitter.addEventListener('pointerdown', (e) => {
    e.preventDefault();
    splitter.setPointerCapture(e.pointerId);
    splitter.classList.add('dragging');
    const startY = e.clientY;
    const startH = details.el.getBoundingClientRect().height;
    const max = el.getBoundingClientRect().height - 180;
    const onMove = (ev) => {
      const next = Math.max(160, Math.min(max, startH + (startY - ev.clientY)));
      details.el.style.height = `${next}px`;
      store.detailsHeight = next;
    };
    const onUp = () => {
      splitter.classList.remove('dragging');
      splitter.removeEventListener('pointermove', onMove);
      splitter.removeEventListener('pointerup', onUp);
      try {
        localStorage.setItem(DETAILS_KEY, String(Math.round(store.detailsHeight)));
      } catch {
        // Storage can be unavailable; the size simply is not remembered.
      }
    };
    splitter.addEventListener('pointermove', onMove);
    splitter.addEventListener('pointerup', onUp);
  });

  function paintDetails() {
    const show = store.selection.size === 1;
    details.el.hidden = !show;
    splitter.hidden = !show;
    details.setTorrent(show ? [...store.selection][0] : null);
  }

  const el = h('div', { class: 'transfers' }, toolbar, list, splitter, details.el);

  /* ----------------------------------------------------------- Wiring */

  const offs = [
    on('state', () => {
      renderList();
      details.onState();
    }),
    on('selection', () => {
      for (const [id, r] of rows) setAttr(r.el, 'aria-selected', store.selection.has(id) ? 'true' : 'false');
      paintToolbar();
      paintDetails();
    }),
    on('filter', () => {
      emptyKind = '';
      renderList();
    }),
  ];

  renderList();
  paintDetails();

  return {
    el,
    destroy() {
      offs.forEach((off) => off());
      details.destroy();
    },
    focusSearch() {
      searchInput.focus();
      searchInput.select();
    },
    // Shortcuts that apply to the list when focus is not in a text field.
    handleKey(e) {
      const sel = selectedTorrents();
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'a') {
        e.preventDefault();
        setSelection(visible.map((x) => x.id), visible.length ? visible[0].id : null);
        return true;
      }
      if (e.key === 'Delete' && sel.length) {
        e.preventDefault();
        openRemoveDialog(sel, e.shiftKey);
        return true;
      }
      if (e.key === ' ' && sel.length) {
        e.preventDefault();
        actions.toggle(sel);
        return true;
      }
      if (e.key === 'Escape' && store.selection.size) {
        setSelection([]);
        return true;
      }
      return false;
    },
  };
}
