import { t, locale, onLanguageChange } from './i18n.js';

const formatters = new Map();
onLanguageChange(() => formatters.clear());

function nf(options) {
  const key = locale() + JSON.stringify(options);
  let f = formatters.get(key);
  if (!f) {
    f = new Intl.NumberFormat(locale(), options);
    formatters.set(key, f);
  }
  return f;
}

export function number(value, digits = 0) {
  return nf({ minimumFractionDigits: digits, maximumFractionDigits: digits }).format(value);
}

const UNITS = ['B', 'KB', 'MB', 'GB', 'TB'];

export function bytes(n) {
  if (typeof n !== 'number' || !Number.isFinite(n) || n < 0) return '—';
  let i = 0;
  let v = n;
  while (v >= 1024 && i < UNITS.length - 1) {
    v /= 1024;
    i++;
  }
  const digits = i === 0 ? 0 : v < 10 ? 2 : v < 100 ? 1 : 0;
  return `${number(v, digits)} ${t(`units.${UNITS[i]}`)}`;
}

export function speed(n) {
  if (!n || n < 1) return '';
  return t('units.perSecond', { value: bytes(n) });
}

export function percent(p) {
  const clamped = Math.max(0, Math.min(1, p || 0));
  if (clamped >= 1) return nf({ style: 'percent', maximumFractionDigits: 0 }).format(1);
  // Floor so 99.96% never shows as 100%.
  return nf({ style: 'percent', minimumFractionDigits: 1, maximumFractionDigits: 1 }).format(Math.floor(clamped * 1000) / 1000);
}

export function eta(seconds) {
  if (typeof seconds !== 'number' || seconds < 0) return '';
  if (seconds < 60) return t('units.seconds', { n: Math.max(1, Math.round(seconds)) });
  if (seconds < 3600) return t('units.minutes', { n: Math.round(seconds / 60) });
  if (seconds < 86400) {
    return t('units.hours', { h: Math.floor(seconds / 3600), m: Math.floor((seconds % 3600) / 60) });
  }
  const d = Math.floor(seconds / 86400);
  if (d > 365) return t('units.infinity');
  return t('units.days', { d, h: Math.floor((seconds % 86400) / 3600) });
}

export function ratio(r) {
  return number(r || 0, 2);
}

export function dateTime(ms) {
  if (!ms) return '—';
  return new Intl.DateTimeFormat(locale(), { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(ms));
}

export function shortDate(ms) {
  if (!ms) return '—';
  const d = new Date(ms);
  const now = new Date();
  if (d.toDateString() === now.toDateString()) {
    return new Intl.DateTimeFormat(locale(), { hour: '2-digit', minute: '2-digit' }).format(d);
  }
  const opts = { day: 'numeric', month: 'short' };
  if (d.getFullYear() !== now.getFullYear()) opts.year = 'numeric';
  return new Intl.DateTimeFormat(locale(), opts).format(d);
}
