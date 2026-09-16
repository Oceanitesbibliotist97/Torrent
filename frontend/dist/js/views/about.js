// About page: version, privacy promise, data folder, shortcuts, credits.

import { h, icon, staticSvg } from '../dom.js';
import { t } from '../i18n.js';
import { store, on } from '../store.js';
import { api, errorText } from '../api.js';
import { toast, copyButton } from '../ui.js';
import { logoMarkup } from '../icons.js';

export const SHORTCUTS = [
  { keys: ['Ctrl', 'O'], label: 'shortcuts.openFile' },
  { keys: ['Ctrl', 'U'], label: 'shortcuts.addMagnet' },
  { keys: ['Ctrl', 'V'], label: 'shortcuts.paste' },
  { keys: ['Ctrl', 'A'], label: 'shortcuts.selectAll' },
  { keys: ['Space'], label: 'shortcuts.toggle' },
  { keys: ['Del'], label: 'shortcuts.remove' },
  { keys: ['Ctrl', 'F'], label: 'shortcuts.search' },
  { keys: ['Ctrl', ','], label: 'shortcuts.settings' },
  { keys: ['Esc'], label: 'shortcuts.escape' },
];

const CREDITS = [
  ['anacrolix/torrent', 'MPL-2.0'],
  ['Wails', 'MIT'],
  ['Go', 'BSD-3-Clause'],
  ['bbolt', 'MIT'],
  ['golang.org/x', 'BSD-3-Clause'],
];

export function createAboutView() {
  const inner = h('div', { class: 'page-inner' });
  const el = h('div', { class: 'page' }, inner);

  function render() {
    const boot = store.boot || {};
    inner.replaceChildren(
      h('section', { class: 'about-hero' },
        staticSvg(logoMarkup()),
        h('div', null,
          h('h1', { text: boot.appName || 'Torrent' }),
          h('p', { text: `${t('about.version', { version: boot.version || '' })} · ${t('app.tagline')}` }))),
      h('div', { class: 'about-grid', style: { 'margin-top': '20px' } },
        h('section', { class: 'card about-card' },
          h('h3', null, icon('shieldCheck'), t('about.promiseTitle')),
          h('ul', null, h('li', { text: t('about.promise1') }), h('li', { text: t('about.promise2') }), h('li', { text: t('about.promise3') }))),
        h('section', { class: 'card about-card' },
          h('h3', null, icon('hardDrive'), t('about.dataTitle')),
          h('div', { class: 'stack-sm' },
            h('span', { class: boot.portable ? 'badge badge-success' : 'badge badge-warn', text: boot.portable ? t('about.portable') : t('about.notPortable') }),
            h('div', { class: 'kv-value with-action' },
              h('span', { class: 'mono selectable truncate', title: boot.dataDir, text: boot.dataDir }),
              copyButton(() => boot.dataDir)),
            h('div', null, h('button', {
              class: 'btn btn-secondary btn-sm',
              type: 'button',
              onclick: () => api.openDataFolder().catch((err) => toast({ kind: 'error', title: errorText(err) })),
            }, icon('folder', 'sm'), t('about.openData'))))),
        h('section', { class: 'card about-card' },
          h('h3', null, icon('keyboard'), t('about.shortcutsTitle')),
          h('div', { class: 'shortcut-list' }, SHORTCUTS.map((s) => h('div', { class: 'shortcut' },
            h('span', { text: t(s.label) }),
            h('span', { class: 'keys' }, s.keys.map((k) => h('kbd', { text: k }))))))),
        h('section', { class: 'card about-card' },
          h('h3', null, icon('scale'), t('about.licenseTitle')),
          h('p', { class: 'legal-text', text: t('about.licenseText') }),
          h('h3', { style: { 'margin-top': '18px' } }, icon('package'), t('about.creditsTitle')),
          CREDITS.map(([name, license]) => h('div', { class: 'credit' }, h('span', { text: name }), h('span', { class: 'subtle', text: license }))),
          h('h3', { style: { 'margin-top': '18px' } }, icon('scale'), t('about.legalTitle')),
          h('p', { class: 'legal-text', text: t('about.legal') }))));
  }

  const off = on('lang', render);
  render();
  return { el, destroy: off };
}
