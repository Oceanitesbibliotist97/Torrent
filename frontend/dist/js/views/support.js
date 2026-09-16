// Support page with the three allowlisted donation links.

import { h, icon, staticSvg } from '../dom.js';
import { t } from '../i18n.js';
import { store, on } from '../store.js';
import { api, errorText } from '../api.js';
import { toast } from '../ui.js';
import { HEART_ART } from '../icons.js';

const BRAND = {
  buymeacoffee: { icon: 'coffee', text: 'support.bmcText' },
  kofi: { icon: 'cup', text: 'support.kofiText' },
  donationalerts: { icon: 'gift', text: 'support.daText' },
};

export function createSupportView() {
  const inner = h('div', { class: 'page-inner' });
  const el = h('div', { class: 'page' }, inner);

  function render() {
    const links = (store.boot && store.boot.links) || [];
    inner.replaceChildren(
      h('section', { class: 'support-hero' },
        h('div', null,
          h('h1', { text: t('support.title') }),
          h('p', { text: t('support.lead') })),
        staticSvg(HEART_ART)),
      h('div', { class: 'donate-grid' }, links.map((link) => {
        const brand = BRAND[link.id] || { icon: 'heart', text: '' };
        return h('article', { class: 'donate-card', dataset: { brand: link.id } },
          h('div', { class: 'donate-logo' }, icon(brand.icon)),
          h('h3', { text: link.name }),
          h('p', { text: brand.text ? t(brand.text) : '' }),
          h('div', { class: 'donate-url', text: link.url.replace(/^https:\/\//, '') }),
          h('button', {
            class: 'btn btn-primary',
            type: 'button',
            onclick: () => api.openLink(link.id).catch((err) => toast({ kind: 'error', title: errorText(err) })),
          }, t('support.open', { name: link.name }), icon('external', 'sm')));
      })),
      h('div', { class: 'callout callout-info', style: { 'margin-top': '14px' } },
        icon('info'),
        h('div', { text: t('support.note') })),
      h('h2', { class: 'section-title', text: t('support.otherTitle') }),
      h('ul', { class: 'help-list' },
        h('li', null, icon('message'), t('support.other1')),
        h('li', null, icon('alertCircle'), t('support.other2')),
        h('li', null, icon('languages'), t('support.other3'))),
      h('p', { class: 'subtle', style: { 'margin-top': '22px' }, text: t('support.thanks') }));
  }

  const off = on('lang', render);
  render();
  return { el, destroy: off };
}
