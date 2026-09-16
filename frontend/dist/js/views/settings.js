// Settings page. Language and theme apply instantly; everything else is
// collected in a draft and saved explicitly, because network changes
// reconnect every transfer.

import { h, icon } from '../dom.js';
import { t, LANGUAGES, loadLanguage } from '../i18n.js';
import { store, on, emit } from '../store.js';
import { api, errorText } from '../api.js';
import { toast, switchControl, segmented, selectControl, settingRow } from '../ui.js';

const SECTIONS = [
  { id: 'general', icon: 'sliders' },
  { id: 'downloads', icon: 'folder' },
  { id: 'speed', icon: 'gauge' },
  { id: 'connection', icon: 'wifi' },
  { id: 'privacy', icon: 'shieldCheck' },
];

const NETWORK_KEYS = ['listenPort', 'enableDHT', 'enablePEX', 'enableUTP', 'enableIPv6', 'enableUPnP', 'enableWebSeeds',
  'encryption', 'anonymousMode', 'networkMode', 'vpnInterface', 'proxy', 'blocklistPath', 'clearProxyPassword'];

const clone = (v) => JSON.parse(JSON.stringify(v));

export function createSettingsView({ section, applyTheme }) {
  let saved = clone(store.settings);
  let draft = clone(store.settings);
  let interfaces = null;

  const content = h('div', { class: 'settings-content' });
  const nav = h('nav', { class: 'settings-nav', 'aria-label': t('settings.title') });
  const inner = h('div', { class: 'page-inner' });
  const el = h('div', { class: 'page' }, inner);

  const netNote = h('span', { class: 'savebar-note', text: t('settings.networkRestart') });
  const saveBtn = h('button', { class: 'btn btn-primary', type: 'button', onclick: save }, icon('check', 'sm'), t('settings.save'));
  const discardBtn = h('button', { class: 'btn btn-ghost', type: 'button', onclick: discard, text: t('settings.discard') });
  const savebar = h('div', { class: 'savebar', hidden: true, role: 'region', 'aria-label': t('settings.unsaved') },
    h('div', { class: 'savebar-text' }, icon('alertCircle'), h('span', { text: t('settings.unsaved') }), netNote),
    discardBtn, saveBtn);

  function markDirty() {
    const dirty = JSON.stringify(draft) !== JSON.stringify(saved);
    savebar.hidden = !dirty;
    netNote.hidden = !NETWORK_KEYS.some((k) => JSON.stringify(draft[k]) !== JSON.stringify(saved[k]));
  }

  async function save() {
    saveBtn.disabled = true;
    try {
      const result = await api.saveSettings(draft);
      store.settings = result;
      saved = clone(result);
      draft = clone(result);
      emit('settings');
      toast({ kind: 'success', title: t('settings.saved'), timeout: 2500 });
      render();
    } catch (err) {
      toast({ kind: 'error', title: errorText(err) });
    } finally {
      saveBtn.disabled = false;
    }
  }

  function discard() {
    draft = clone(saved);
    render();
  }

  async function quickSave(patch) {
    try {
      const result = await api.saveSettings({ ...saved, ...patch, clearProxyPassword: false });
      store.settings = result;
      saved = clone(result);
      Object.assign(draft, patch);
      emit('settings');
      if (patch.theme) applyTheme(patch.theme);
      if (patch.language) await loadLanguage(patch.language);
    } catch (err) {
      toast({ kind: 'error', title: errorText(err) });
    }
  }

  const bindSwitch = (key, title, hint) => settingRow({
    title: t(title),
    hint: hint ? t(hint) : '',
    control: switchControl({ checked: draft[key], label: t(title), onChange: (v) => { draft[key] = v; markDirty(); } }),
  });

  function numberInput(value, { min, max, step = 1, integer = true, onChange, label }) {
    const input = h('input', { class: 'input num-input num', type: 'number', min: String(min), max: String(max), step: String(step), 'aria-label': label });
    input.value = String(value);
    input.addEventListener('input', () => {
      const v = Number(input.value);
      if (input.value !== '' && Number.isFinite(v)) {
        onChange(integer ? Math.round(v) : v);
        markDirty();
      }
    });
    return input;
  }

  function field(label, control, extra) {
    return h('label', { class: 'field' }, h('span', { class: 'field-label', text: label }), control, extra || null);
  }

  function card(...rows) {
    return h('div', { class: 'card' }, rows);
  }

  function sectionEl(id, iconName, ...children) {
    return h('section', { class: 'settings-section', id: `settings-${id}` },
      h('h2', null, icon(iconName), t(`settings.section.${id}`)), children);
  }

  function general() {
    return sectionEl('general', 'sliders', card(
      settingRow({
        title: t('settings.language'),
        control: segmented({
          label: t('settings.language'),
          value: draft.language || 'en',
          options: LANGUAGES.map((l) => ({ value: l.code, label: l.name })),
          onChange: (v) => quickSave({ language: v }),
        }),
      }),
      settingRow({
        title: t('settings.theme'),
        control: segmented({
          label: t('settings.theme'),
          value: draft.theme,
          options: [
            { value: 'system', label: t('settings.themeSystem'), icon: 'monitor' },
            { value: 'dark', label: t('settings.themeDark'), icon: 'moon' },
            { value: 'light', label: t('settings.themeLight'), icon: 'sun' },
          ],
          onChange: (v) => quickSave({ theme: v }),
        }),
      })));
  }

  function downloads() {
    const dirInput = h('input', { class: 'input mono', value: draft.downloadDir, readOnly: true, title: draft.downloadDir, 'aria-label': t('settings.downloadDir') });
    const browse = h('button', {
      class: 'btn btn-secondary',
      type: 'button',
      onclick: async () => {
        const dir = await api.pickFolder(t('settings.downloadDir'), draft.downloadDir).catch(() => '');
        if (dir) {
          draft.downloadDir = dir;
          dirInput.value = dir;
          dirInput.title = dir;
          markDirty();
        }
      },
    }, icon('folder', 'sm'), t('add.browse'));
    return sectionEl('downloads', 'folder', card(
      h('div', { class: 'setting-row stack' },
        h('div', { class: 'setting-text' }, h('div', { class: 'setting-title', text: t('settings.downloadDir') })),
        h('div', { class: 'path-field' }, dirInput, browse)),
      bindSwitch('startImmediately', 'settings.startImmediately', 'settings.startImmediatelyHint'),
      bindSwitch('warnExecutables', 'settings.warnExecutables', 'settings.warnExecutablesHint'),
      bindSwitch('markOfTheWeb', 'settings.markOfTheWeb', 'settings.markOfTheWebHint'),
      bindSwitch('rememberTorrents', 'settings.rememberTorrents', 'settings.rememberTorrentsHint')));
  }

  function speed() {
    const limit = (key, title) => settingRow({
      title: t(title),
      hint: t('settings.limitHint'),
      control: numberInput(draft[key], { min: 0, max: 10000000, label: t(title), onChange: (v) => { draft[key] = Math.max(0, v); } }),
    });
    return sectionEl('speed', 'gauge', card(
      limit('downloadLimitKiB', 'settings.downloadLimit'),
      limit('uploadLimitKiB', 'settings.uploadLimit'),
      bindSwitch('seedAfterComplete', 'settings.seedAfterComplete', 'settings.seedAfterCompleteHint'),
      settingRow({
        title: t('settings.seedRatioLimit'),
        hint: t('settings.seedRatioLimitHint'),
        control: numberInput(draft.seedRatioLimit, { min: 0, max: 1000, step: 0.1, integer: false, label: t('settings.seedRatioLimit'), onChange: (v) => { draft.seedRatioLimit = Math.max(0, v); } }),
      }),
      settingRow({
        title: t('settings.maxPeers'),
        control: numberInput(draft.maxPeersPerTorrent, { min: 5, max: 500, label: t('settings.maxPeers'), onChange: (v) => { draft.maxPeersPerTorrent = v; } }),
      })));
  }

  function connection() {
    const port = numberInput(draft.listenPort, { min: 1024, max: 65535, label: t('settings.listenPort'), onChange: (v) => { draft.listenPort = v; } });
    const random = h('button', {
      class: 'btn btn-secondary btn-sm',
      type: 'button',
      onclick: () => {
        draft.listenPort = 20000 + Math.floor(Math.random() * 40000);
        port.value = String(draft.listenPort);
        markDirty();
      },
    }, icon('refresh', 'sm'), t('settings.randomPort'));
    return sectionEl('connection', 'wifi', card(
      settingRow({ title: t('settings.listenPort'), hint: t('settings.listenPortHint'), control: h('div', { class: 'field-row' }, port, random) }),
      bindSwitch('randomizePort', 'settings.randomizePort'),
      bindSwitch('enableDHT', 'settings.dht', 'settings.dhtHint'),
      bindSwitch('enablePEX', 'settings.pex', 'settings.pexHint'),
      bindSwitch('enableUTP', 'settings.utp', 'settings.utpHint'),
      bindSwitch('enableIPv6', 'settings.ipv6', 'settings.ipv6Hint'),
      bindSwitch('enableUPnP', 'settings.upnp', 'settings.upnpHint'),
      bindSwitch('enableWebSeeds', 'settings.webSeeds', 'settings.webSeedsHint')));
  }

  function vpnPanel() {
    const selectHost = h('div', { class: 'grow' });
    function renderSelect() {
      const options = (interfaces || []).map((i) => ({
        value: i.name,
        label: `${i.name}${i.likelyVPN ? ` · ${t('settings.likelyVpn')}` : ''} — ${i.addresses.slice(0, 2).join(', ')}`,
      }));
      if (draft.vpnInterface && !options.some((o) => o.value === draft.vpnInterface)) {
        options.unshift({ value: draft.vpnInterface, label: draft.vpnInterface });
      }
      selectHost.replaceChildren(interfaces && !options.length
        ? h('div', { class: 'field-hint', text: t('settings.noInterfaces') })
        : selectControl({
          options,
          value: draft.vpnInterface,
          placeholder: t('settings.vpnInterfacePlaceholder'),
          label: t('settings.vpnInterface'),
          onChange: (v) => { draft.vpnInterface = v; markDirty(); },
        }));
    }
    async function load() {
      interfaces = await api.interfaces().catch(() => []);
      renderSelect();
    }
    if (interfaces) renderSelect();
    else load();
    return h('div', { class: 'mode-extra' },
      h('div', { class: 'field' },
        h('span', { class: 'field-label', text: t('settings.vpnInterface') }),
        h('div', { class: 'path-field' }, selectHost,
          h('button', { class: 'btn btn-secondary', type: 'button', onclick: load }, icon('refresh', 'sm'), t('settings.refresh'))),
        h('div', { class: 'field-hint', text: t('settings.vpnInterfaceHint') })),
      bindSwitch('autoReconnect', 'settings.autoReconnect', 'settings.autoReconnectHint'));
  }

  function proxyPanel() {
    const host = h('input', { class: 'input mono', placeholder: '127.0.0.1', spellcheck: 'false', autocomplete: 'off' });
    host.value = draft.proxy.host;
    host.addEventListener('input', () => { draft.proxy.host = host.value.trim(); markDirty(); });
    const port = numberInput(draft.proxy.port, { min: 1, max: 65535, label: t('settings.proxyPort'), onChange: (v) => { draft.proxy.port = v; } });
    port.classList.remove('num-input');
    const user = h('input', { class: 'input', placeholder: t('settings.proxyOptional'), spellcheck: 'false', autocomplete: 'off' });
    user.value = draft.proxy.username;
    user.addEventListener('input', () => { draft.proxy.username = user.value; markDirty(); });
    const hasSaved = saved.proxyPasswordSet && !draft.clearProxyPassword;
    const pass = h('input', { class: 'input', type: 'password', autocomplete: 'new-password', placeholder: hasSaved ? t('settings.proxyPassSaved') : t('settings.proxyOptional') });
    pass.addEventListener('input', () => {
      draft.proxy.password = pass.value;
      draft.clearProxyPassword = false;
      markDirty();
    });
    const clear = hasSaved ? h('button', {
      class: 'link-btn',
      type: 'button',
      text: t('settings.proxyPassClear'),
      onclick: (e) => {
        draft.clearProxyPassword = true;
        draft.proxy.password = '';
        pass.value = '';
        pass.placeholder = t('settings.proxyOptional');
        e.currentTarget.remove();
        markDirty();
      },
    }) : null;
    return h('div', { class: 'mode-extra' },
      h('div', { class: 'grid-2' }, field(t('settings.proxyHost'), host), field(t('settings.proxyPort'), port)),
      h('div', { class: 'grid-2-even' }, field(t('settings.proxyUser'), user), field(t('settings.proxyPass'), pass, clear)),
      store.boot && store.boot.secretStorage ? h('div', { class: 'field-hint', text: t('settings.proxyPassNote') }) : null);
  }

  function modeCards() {
    const host = h('div', { class: 'mode-cards', role: 'radiogroup', 'aria-label': t('settings.networkMode') });
    const modes = [
      { id: 'direct', icon: 'globe' },
      { id: 'vpn', icon: 'shieldCheck', recommended: true },
      { id: 'proxy', icon: 'server' },
    ];
    const cap = (s) => s[0].toUpperCase() + s.slice(1);
    function paint(focusId) {
      host.replaceChildren();
      for (const m of modes) {
        const checked = draft.networkMode === m.id;
        const cardEl = h('button', {
          type: 'button',
          class: 'mode-card',
          role: 'radio',
          'aria-checked': String(checked),
          onclick: () => {
            if (draft.networkMode === m.id) return;
            draft.networkMode = m.id;
            markDirty();
            paint(m.id);
          },
        },
        h('span', { class: 'mode-radio' }),
        h('span', null,
          h('div', { class: 'mode-title' }, t(`settings.mode${cap(m.id)}`),
            m.recommended ? h('span', { class: 'badge badge-success recommended', text: t('settings.recommended') }) : null),
          h('div', { class: 'mode-hint', text: t(`settings.mode${cap(m.id)}Hint`) })),
        icon(m.icon));
        host.appendChild(cardEl);
        if (checked && m.id === 'vpn') host.appendChild(vpnPanel());
        if (checked && m.id === 'proxy') host.appendChild(proxyPanel());
        if (focusId === m.id) requestAnimationFrame(() => cardEl.focus());
      }
    }
    paint();
    return host;
  }

  function privacy() {
    const encHint = h('div', { class: 'setting-hint' });
    const syncEnc = () => { encHint.textContent = t(draft.encryption === 'require' ? 'settings.encryptionRequireHint' : 'settings.encryptionPreferHint'); };
    syncEnc();

    const listPath = h('input', { class: 'input mono', readOnly: true, placeholder: t('settings.blocklistNone'), 'aria-label': t('settings.blocklist') });
    listPath.value = draft.blocklistPath;
    const clearList = h('button', {
      class: 'btn btn-ghost',
      type: 'button',
      text: t('settings.clear'),
      disabled: !draft.blocklistPath,
      onclick: () => {
        draft.blocklistPath = '';
        listPath.value = '';
        clearList.disabled = true;
        markDirty();
      },
    });
    const chooseList = h('button', {
      class: 'btn btn-secondary',
      type: 'button',
      onclick: async () => {
        const path = await api.pickBlocklist(t('settings.blocklist')).catch(() => '');
        if (path) {
          draft.blocklistPath = path;
          listPath.value = path;
          clearList.disabled = false;
          markDirty();
        }
      },
    }, icon('file', 'sm'), t('settings.choose'));

    return sectionEl('privacy', 'shieldCheck', card(
      h('div', { class: 'setting-row' },
        h('div', { class: 'setting-text' }, h('div', { class: 'setting-title', text: t('settings.encryption') }), encHint),
        h('div', { class: 'setting-control' }, segmented({
          label: t('settings.encryption'),
          value: draft.encryption,
          options: [
            { value: 'prefer', label: t('settings.encryptionPrefer') },
            { value: 'require', label: t('settings.encryptionRequire'), icon: 'lock' },
          ],
          onChange: (v) => { draft.encryption = v; syncEnc(); markDirty(); },
        }))),
      bindSwitch('anonymousMode', 'settings.anonymousMode', 'settings.anonymousModeHint'),
      h('div', { class: 'setting-row stack' },
        h('div', { class: 'setting-text' }, h('div', { class: 'setting-title', text: t('settings.networkMode') })),
        modeCards()),
      h('div', { class: 'setting-row stack' },
        h('div', { class: 'setting-text' },
          h('div', { class: 'setting-title', text: t('settings.blocklist') }),
          h('div', { class: 'setting-hint', text: t('settings.blocklistHint') })),
        h('div', { class: 'path-field' }, listPath, chooseList, clearList))));
  }

  function render() {
    const scrollTop = el.scrollTop;
    nav.replaceChildren(...SECTIONS.map((s) => h('button', {
      type: 'button',
      dataset: { section: s.id },
      onclick: () => {
        const target = content.querySelector(`#settings-${s.id}`);
        if (target) target.scrollIntoView({ behavior: 'smooth', block: 'start' });
      },
    }, icon(s.icon), t(`settings.section.${s.id}`))));
    content.replaceChildren(general(), downloads(), speed(), connection(), privacy(), savebar);
    inner.replaceChildren(
      h('header', { class: 'page-header' }, h('h1', { text: t('settings.title') })),
      h('div', { class: 'settings-layout' }, nav, content));
    netNote.textContent = t('settings.networkRestart');
    markDirty();
    el.scrollTop = scrollTop;
    updateNav();
  }

  function updateNav() {
    const top = el.getBoundingClientRect().top;
    let current = SECTIONS[0].id;
    for (const s of SECTIONS) {
      const node = content.querySelector(`#settings-${s.id}`);
      if (node && node.getBoundingClientRect().top - top <= 90) current = s.id;
    }
    for (const b of nav.children) b.setAttribute('aria-current', String(b.dataset.section === current));
  }
  el.addEventListener('scroll', updateNav, { passive: true });

  const offs = [on('lang', render)];
  render();
  if (section) {
    requestAnimationFrame(() => {
      const target = content.querySelector(`#settings-${section}`);
      if (target) target.scrollIntoView({ block: 'start' });
    });
  }
  return {
    el,
    destroy: () => offs.forEach((off) => off()),
    isDirty: () => !savebar.hidden,
  };
}
