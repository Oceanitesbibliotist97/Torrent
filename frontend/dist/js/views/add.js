// Add-torrent, add-magnet and remove dialogs.

import { h, icon, debounce } from '../dom.js';
import { t } from '../i18n.js';
import { store } from '../store.js';
import { api, errorText } from '../api.js';
import { openModal, toast } from '../ui.js';
import { bytes } from '../format.js';
import { refreshState } from '../actions.js';

const MAX_ROWS = 1000;

function basename(path) {
  return path.split(/[\\/]/).pop();
}

function savePathField(initial) {
  let value = initial;
  const input = h('input', { class: 'input mono', value, readOnly: true, 'aria-label': t('add.saveTo'), title: value });
  const free = h('div', { class: 'field-hint' });
  let freeBytes = -1;
  const listeners = new Set();

  async function refreshFree() {
    freeBytes = await api.freeSpace(value).catch(() => -1);
    free.textContent = freeBytes >= 0 ? t('add.freeSpace', { size: bytes(freeBytes) }) : '';
    listeners.forEach((fn) => fn());
  }

  const browse = h('button', {
    class: 'btn btn-secondary',
    type: 'button',
    onclick: async () => {
      const dir = await api.pickFolder(t('add.saveTo'), value).catch(() => '');
      if (dir) {
        value = dir;
        input.value = dir;
        input.title = dir;
        refreshFree();
      }
    },
  }, icon('folder', 'sm'), t('add.browse'));

  refreshFree();
  return {
    el: h('div', { class: 'field' },
      h('span', { class: 'field-label', text: t('add.saveTo') }),
      h('div', { class: 'path-field' }, input, browse),
      free),
    get value() {
      return value;
    },
    get freeBytes() {
      return freeBytes;
    },
    hint: free,
    onFreeChange: (fn) => listeners.add(fn),
  };
}

function startCheckbox() {
  const input = h('input', { type: 'checkbox', class: 'check-box', checked: !!(store.settings && store.settings.startImmediately) });
  return { el: h('label', { class: 'inline-check' }, input, h('span', { text: t('add.start') })), input };
}

export async function openAddFiles(paths) {
  let list = paths;
  if (!list) {
    try {
      list = await api.pickTorrentFiles(t('toolbar.addFile'));
    } catch (err) {
      toast({ kind: 'error', title: errorText(err) });
      return;
    }
  }
  for (const path of list || []) {
    await addTorrentDialog(path);
  }
}

async function addTorrentDialog(path) {
  let preview;
  try {
    preview = await api.previewTorrent(path);
  } catch (err) {
    toast({ kind: 'error', title: errorText(err), detail: basename(path) });
    return;
  }
  if (preview.exists) {
    toast({ kind: 'info', title: t('add.duplicate'), detail: preview.name });
    return;
  }

  const settings = store.settings || {};
  const warn = settings.warnExecutables !== false;
  const multi = preview.files.length > 1;
  const selected = new Set(preview.files.map((f) => f.index));

  await new Promise((resolve) => {
    const saveField = savePathField(settings.downloadDir);
    const start = startCheckbox();
    const selectedInfo = h('span', { class: 'footer-info num' });
    const addBtn = h('button', { class: 'btn btn-primary', type: 'button', 'data-autofocus': '' }, icon('plus', 'sm'), t('add.confirm'));
    const cancelBtn = h('button', { class: 'btn btn-secondary', type: 'button', text: t('add.cancel') });

    const body = [
      h('div', { class: 'summary' },
        h('div', { class: 'summary-icon' }, icon('file', 'lg')),
        h('div', { class: 'summary-main' },
          h('div', { class: 'summary-name selectable', text: preview.name }),
          h('div', { class: 'summary-meta num', text: `${bytes(preview.totalSize)} · ${t('add.files', { n: preview.files.length })}` }))),
    ];
    if (warn && preview.riskyFiles > 0) {
      body.push(h('div', { class: 'callout callout-warn', role: 'alert' },
        icon('alertTriangle'),
        h('div', null, h('strong', { text: t('add.riskyTitle', { n: preview.riskyFiles }) }), h('span', { text: t('add.riskyText') }))));
    }
    if (preview.private) {
      body.push(h('div', { class: 'callout callout-info' }, icon('lock'), h('div', { text: t('add.private') })));
    }

    let rows = [];
    let master = null;
    if (multi) {
      master = h('input', { type: 'checkbox', class: 'check-box', checked: true, 'aria-label': t('add.selectAll') });
      const filter = h('input', { class: 'input', type: 'search', placeholder: t('add.filterFiles'), 'aria-label': t('add.filterFiles') });
      const listEl = h('div', { class: 'file-picker-list', role: 'list' });
      rows = preview.files.map((f) => {
        const risky = warn && f.risky;
        const cb = h('input', { type: 'checkbox', class: 'check-box', checked: true, 'aria-label': f.path });
        cb.addEventListener('change', () => {
          if (cb.checked) selected.add(f.index);
          else selected.delete(f.index);
          update();
        });
        const row = h('label', { class: risky ? 'file-row risky' : 'file-row', role: 'listitem', title: risky ? t('details.riskyFile') : f.path },
          cb,
          h('span', { class: 'file-name' }, icon(risky ? 'fileWarning' : 'file', 'sm'), h('span', { text: f.path })),
          h('span', { class: 'file-size', text: bytes(f.size) }));
        return { f, cb, row };
      });
      const renderRows = () => {
        const q = filter.value.trim().toLocaleLowerCase();
        listEl.replaceChildren(...rows.filter((r) => !q || r.f.path.toLocaleLowerCase().includes(q)).slice(0, MAX_ROWS).map((r) => r.row));
      };
      master.addEventListener('change', () => {
        for (const r of rows) {
          r.cb.checked = master.checked;
          if (master.checked) selected.add(r.f.index);
          else selected.delete(r.f.index);
        }
        update();
      });
      filter.addEventListener('input', debounce(renderRows, 120));
      renderRows();
      body.push(h('div', { class: 'file-picker' }, h('div', { class: 'file-picker-head' }, master, filter), listEl));
    }
    body.push(saveField.el, start.el);

    function update() {
      let size = 0;
      for (const f of preview.files) if (selected.has(f.index)) size += f.size;
      selectedInfo.textContent = multi ? t('add.selectedSize', { selected: bytes(size), total: bytes(preview.totalSize) }) : '';
      if (master) {
        master.checked = selected.size === preview.files.length;
        master.indeterminate = selected.size > 0 && selected.size < preview.files.length;
      }
      const lowSpace = saveField.freeBytes >= 0 && size > saveField.freeBytes;
      saveField.hint.classList.toggle('warn-text', lowSpace);
      addBtn.disabled = selected.size === 0;
    }
    saveField.onFreeChange(update);
    update();

    const modal = openModal({
      title: t('add.titleFile'),
      subtitle: basename(path),
      wide: multi,
      body,
      footer: [selectedInfo, cancelBtn, addBtn],
      onClose: resolve,
    });
    cancelBtn.addEventListener('click', () => modal.close());
    addBtn.addEventListener('click', async () => {
      addBtn.disabled = true;
      let priorities = null;
      if (multi) {
        priorities = new Array(preview.fileCount).fill(1);
        for (const f of preview.files) if (!selected.has(f.index)) priorities[f.index] = 0;
      }
      try {
        await api.addTorrentFile(path, { savePath: saveField.value, start: start.input.checked, priorities });
        modal.close(true);
        toast({ kind: 'success', title: t('notice.added'), detail: preview.name });
        refreshState();
      } catch (err) {
        addBtn.disabled = false;
        toast({ kind: 'error', title: errorText(err) });
      }
    });
  });
}

export function openAddMagnet(prefill = '') {
  const settings = store.settings || {};
  const saveField = savePathField(settings.downloadDir);
  const start = startCheckbox();
  const textarea = h('textarea', {
    class: 'textarea mono',
    placeholder: t('add.magnetPlaceholder'),
    'aria-label': t('add.magnetLabel'),
    spellcheck: 'false',
    'data-autofocus': '',
    rows: '3',
  });
  textarea.value = prefill;
  const status = h('div', { class: 'field-status', 'aria-live': 'polite' });
  const addBtn = h('button', { class: 'btn btn-primary', type: 'button', disabled: true }, icon('plus', 'sm'), t('add.confirm'));
  const cancelBtn = h('button', { class: 'btn btn-secondary', type: 'button', text: t('add.cancel') });
  let seq = 0;

  async function validate() {
    const mine = ++seq;
    const value = textarea.value.trim();
    addBtn.disabled = true;
    if (!value) {
      status.className = 'field-status';
      status.replaceChildren();
      textarea.removeAttribute('aria-invalid');
      return;
    }
    try {
      const p = await api.previewMagnet(value);
      if (mine !== seq) return;
      const parts = [p.name || t('add.magnetNoName'), t('add.magnetTrackers', { n: p.trackers })];
      if (p.skipped) parts.push(t('add.magnetSkipped', { n: p.skipped }));
      status.className = p.exists ? 'field-status bad' : 'field-status ok';
      status.replaceChildren(icon(p.exists ? 'alertCircle' : 'checkCircle', 'sm'), h('span', { class: 'truncate', text: p.exists ? t('add.duplicate') : parts.join(' · ') }));
      textarea.removeAttribute('aria-invalid');
      addBtn.disabled = p.exists;
    } catch {
      if (mine !== seq) return;
      status.className = 'field-status bad';
      status.replaceChildren(icon('alertCircle', 'sm'), h('span', { text: t('add.magnetInvalid') }));
      textarea.setAttribute('aria-invalid', 'true');
    }
  }
  textarea.addEventListener('input', debounce(validate, 150));

  const modal = openModal({
    title: t('add.titleMagnet'),
    body: [
      h('div', { class: 'field' }, h('span', { class: 'field-label', text: t('add.magnetLabel') }), textarea, status),
      saveField.el,
      start.el,
    ],
    footer: [cancelBtn, addBtn],
  });
  cancelBtn.addEventListener('click', () => modal.close());

  async function submit() {
    if (addBtn.disabled) return;
    addBtn.disabled = true;
    try {
      await api.addMagnet(textarea.value.trim(), { savePath: saveField.value, start: start.input.checked, priorities: null });
      modal.close(true);
      toast({ kind: 'success', title: t('notice.added') });
      refreshState();
    } catch (err) {
      addBtn.disabled = false;
      toast({ kind: 'error', title: errorText(err) });
    }
  }
  addBtn.addEventListener('click', submit);
  textarea.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      submit();
    }
  });
  if (prefill) validate();
}

export function openRemoveDialog(torrents, withFiles = false) {
  if (!torrents.length) return;
  const n = torrents.length;
  const ids = torrents.map((x) => x.id);
  const deleteBox = h('input', { type: 'checkbox', class: 'check-box', checked: withFiles });
  const hint = h('div', { class: 'field-hint danger-text', text: t('remove.deleteFilesHint') });
  const label = h('span');
  const confirmBtn = h('button', { class: 'btn btn-danger', type: 'button' }, icon('trash', 'sm'), label);
  const cancelBtn = h('button', { class: 'btn btn-secondary', type: 'button', text: t('common.cancel'), 'data-autofocus': '' });

  function sync() {
    hint.hidden = !deleteBox.checked;
    label.textContent = deleteBox.checked ? t('remove.confirmDelete') : t('remove.confirm');
  }
  deleteBox.addEventListener('change', sync);
  sync();

  const modal = openModal({
    title: t('remove.title', { n }),
    body: [
      h('ul', { class: 'remove-list' },
        torrents.slice(0, 5).map((x) => h('li', { text: x.name, title: x.name })),
        n > 5 ? h('li', { class: 'subtle', text: t('remove.andMore', { n: n - 5 }) }) : null),
      h('label', { class: 'inline-check' }, deleteBox, h('span', { text: t('remove.deleteFiles') })),
      hint,
    ],
    footer: [cancelBtn, confirmBtn],
  });
  cancelBtn.addEventListener('click', () => modal.close());
  confirmBtn.addEventListener('click', async () => {
    confirmBtn.disabled = true;
    try {
      await api.remove(ids, deleteBox.checked);
      modal.close(true);
      toast({ kind: 'success', title: t('notice.removed', { n }) });
    } catch (err) {
      modal.close(false);
      toast({ kind: 'error', title: errorText(err), detail: err.detail || '' });
    } finally {
      refreshState();
    }
  });
}

// handleOpenArgs processes magnet links and .torrent paths passed to the .exe.
export async function handleOpenArgs(args) {
  for (const arg of args || []) {
    if (/^magnet:\?/i.test(arg)) openAddMagnet(arg);
    else await openAddFiles([arg]);
  }
}
