// Privacy center: an honest, live report of what is protected.

import { h, icon } from '../dom.js';
import { t } from '../i18n.js';
import { store, on } from '../store.js';
import { api, errorText } from '../api.js';
import { toast } from '../ui.js';

// protection summarises the effective network protection for the sidebar,
// status bar and this page.
export function protection(global, settings) {
  const net = global && global.network;
  const mode = (net && net.mode) || (settings && settings.networkMode) || 'direct';
  if (!net) return { level: 'warn', status: 'direct', icon: 'shieldAlert' };
  if (mode === 'proxy' && net.online && !net.proxyReachable) return { level: 'danger', status: 'proxyUnreachable', icon: 'shieldOff' };
  if (!net.online) return { level: 'danger', status: net.killSwitch ? 'killSwitch' : 'offline', icon: mode === 'direct' ? 'wifiOff' : 'shieldOff' };
  if (mode === 'vpn') return { level: 'safe', status: 'vpn', icon: 'shieldCheck' };
  if (mode === 'proxy') return { level: 'safe', status: 'proxy', icon: 'shieldCheck' };
  return { level: 'warn', status: 'direct', icon: 'shieldAlert' };
}

const BADGE_CLASS = { safe: 'badge badge-success', warn: 'badge badge-warn', danger: 'badge badge-danger', neutral: 'badge' };

function check({ iconName, title, text, level, badge }) {
  return h('div', { class: 'check', dataset: { level } },
    h('div', { class: 'check-icon' }, icon(iconName)),
    h('div', null,
      h('div', { class: 'check-head' },
        h('span', { class: 'check-title', text: title }),
        badge ? h('span', { class: BADGE_CLASS[level], text: t(`privacy.badge.${badge}`) }) : null),
      h('div', { class: 'check-text', text })));
}

export function createPrivacyView({ navigate }) {
  const inner = h('div', { class: 'page-inner' });
  const el = h('div', { class: 'page' }, inner);
  let signature = '';

  function heroSection() {
    const net = (store.global && store.global.network) || {};
    const p = protection(store.global, store.settings);
    const goSettings = () => navigate('settings', { section: 'privacy' });
    let title;
    let text;
    const actions = [];
    switch (p.status) {
      case 'vpn':
        title = t('privacy.hero.vpnTitle');
        text = t('privacy.hero.vpnText', { iface: net.interface });
        actions.push(h('button', { class: 'btn btn-secondary', type: 'button', onclick: goSettings }, icon('sliders'), t('privacy.hero.change')));
        break;
      case 'proxy':
        title = t('privacy.hero.proxyTitle');
        text = t('privacy.hero.proxyText', { proxy: net.proxy });
        actions.push(h('button', { class: 'btn btn-secondary', type: 'button', onclick: goSettings }, icon('sliders'), t('privacy.hero.change')));
        break;
      case 'direct':
        title = t('privacy.hero.directTitle');
        text = t('privacy.hero.directText');
        actions.push(h('button', { class: 'btn btn-primary', type: 'button', onclick: goSettings }, icon('shieldCheck'), t('privacy.hero.setup')));
        break;
      default:
        title = p.status === 'proxyUnreachable' ? t('status.proxyUnreachable') : t('privacy.hero.offlineTitle');
        text = t('privacy.hero.offlineText');
        if (net.error) text += ` ${errorText(net.error)}`;
        actions.push(
          h('button', {
            class: 'btn btn-primary',
            type: 'button',
            onclick: async (e) => {
              e.currentTarget.disabled = true;
              try {
                await api.reconnect();
              } catch (err) {
                toast({ kind: 'error', title: errorText(err) });
              }
            },
          }, icon('refresh'), t('privacy.hero.reconnect')),
          h('button', { class: 'btn btn-secondary', type: 'button', onclick: goSettings }, icon('sliders'), t('privacy.hero.change')));
    }
    return h('section', { class: 'hero', dataset: { level: p.level } },
      h('div', { class: 'hero-icon' }, icon(p.icon)),
      h('div', { class: 'hero-text' },
        h('h2', { class: 'hero-title', text: title }),
        h('p', { class: 'hero-desc', text }),
        h('div', { class: 'hero-actions' }, actions)));
  }

  function checks() {
    const s = store.settings || {};
    const net = (store.global && store.global.network) || {};
    const mode = s.networkMode || 'direct';
    const boot = store.boot || {};
    const c = (key) => t(`privacy.check.${key}`);
    const items = [
      { iconName: 'eyeOff', title: c('telemetryTitle'), text: c('telemetryText'), level: 'safe', badge: 'always' },
      { iconName: 'hardDrive', title: c('portableTitle'), text: boot.portable ? c('portableText') : c('appdataText'), level: boot.portable ? 'safe' : 'neutral', badge: boot.portable ? 'on' : null },
      {
        iconName: 'globe',
        title: c('ipTitle'),
        text: mode === 'vpn' ? t('privacy.check.ipVpn', { iface: s.vpnInterface }) : mode === 'proxy' ? t('privacy.check.ipProxy', { proxy: net.proxy || s.proxy.host }) : c('ipDirect'),
        level: mode === 'direct' ? 'warn' : 'safe',
        badge: mode === 'direct' ? 'off' : 'on',
      },
      { iconName: 'zap', title: c('killTitle'), text: mode === 'vpn' ? c('killText') : c('killNa'), level: mode === 'vpn' ? 'safe' : 'neutral', badge: mode === 'vpn' ? 'always' : 'notUsed' },
      { iconName: 'server', title: c('dnsTitle'), text: mode === 'vpn' ? c('dnsVpn') : mode === 'proxy' ? c('dnsProxy') : c('dnsDirect'), level: mode === 'direct' ? 'neutral' : 'safe', badge: mode === 'direct' ? null : 'on' },
      { iconName: 'lock', title: c('encryptionTitle'), text: s.encryption === 'require' ? c('encryptionRequired') : c('encryptionPreferred'), level: s.encryption === 'require' ? 'safe' : 'warn', badge: s.encryption === 'require' ? 'required' : 'preferred' },
      { iconName: 'userX', title: c('fingerprintTitle'), text: s.anonymousMode ? c('fingerprintOn') : c('fingerprintOff'), level: s.anonymousMode ? 'safe' : 'warn', badge: s.anonymousMode ? 'on' : 'off' },
      { iconName: 'videoOff', title: c('webrtcTitle'), text: c('webrtcText'), level: 'safe', badge: 'off' },
      { iconName: 'router', title: c('upnpTitle'), text: net.upnp ? c('upnpOn') : c('upnpOff'), level: net.upnp ? 'warn' : 'safe', badge: net.upnp ? 'on' : 'off' },
      { iconName: 'nodes', title: c('dhtTitle'), text: net.dht ? c('dhtOn') : c('dhtOff'), level: net.dht ? 'neutral' : 'safe', badge: net.dht ? 'on' : 'off' },
      { iconName: 'history', title: c('historyTitle'), text: s.rememberTorrents ? c('historyOn') : c('historyOff'), level: s.rememberTorrents ? 'neutral' : 'safe', badge: s.rememberTorrents ? 'local' : 'none' },
      { iconName: 'sparkles', title: c('linksTitle'), text: c('linksText'), level: 'safe', badge: 'always' },
      { iconName: 'fileWarning', title: c('execTitle'), text: s.warnExecutables ? c('execOn') : c('execOff'), level: s.warnExecutables ? 'safe' : 'warn', badge: s.warnExecutables ? 'on' : 'off' },
      { iconName: 'stamp', title: c('motwTitle'), text: s.markOfTheWeb ? c('motwOn') : c('motwOff'), level: s.markOfTheWeb ? 'safe' : 'warn', badge: s.markOfTheWeb ? 'on' : 'off' },
      {
        iconName: 'ban',
        title: c('blocklistTitle'),
        text: net.blocklistRules > 0 ? t('privacy.check.blocklistOn', { n: net.blocklistRules }) : c('blocklistOff'),
        level: net.blocklistRules > 0 ? 'safe' : 'neutral',
        badge: net.blocklistRules > 0 ? 'on' : 'off',
      },
    ];
    return h('div', { class: 'check-grid' }, items.map(check));
  }

  function limits() {
    return h('section', { class: 'limits' },
      h('h2', { class: 'section-title', text: t('privacy.limits.title') }),
      h('ul', null, [1, 2, 3, 4].map((i) => h('li', { text: t(`privacy.limits.item${i}`) }))));
  }

  function render() {
    inner.replaceChildren(
      h('header', { class: 'page-header' },
        h('h1', { text: t('privacy.title') }),
        h('p', { text: t('privacy.subtitle') })),
      heroSection(),
      h('div', { class: 'section-title', text: '' }),
      checks(),
      limits());
  }

  function currentSignature() {
    const net = (store.global && store.global.network) || {};
    const s = store.settings || {};
    return JSON.stringify([net.mode, net.online, net.killSwitch, net.proxyReachable, net.interface, net.proxy, net.dht, net.upnp, net.blocklistRules, net.error,
      s.networkMode, s.vpnInterface, s.encryption, s.anonymousMode, s.rememberTorrents, s.warnExecutables, s.markOfTheWeb]);
  }

  const offs = [
    on('state', () => {
      const sig = currentSignature();
      if (sig !== signature) {
        signature = sig;
        render();
      }
    }),
    on('settings', render),
    on('lang', render),
  ];
  signature = currentSignature();
  render();
  return { el, destroy: () => offs.forEach((off) => off()) };
}
