// Localization with plural forms (Intl.PluralRules), so Russian gets
// "1 файл, 2 файла, 5 файлов" right.

let fallback = {};
let dict = {};
let lang = 'en';
let rules = new Intl.PluralRules('en-US');
const listeners = new Set();

export const LANGUAGES = [
  { code: 'en', name: 'English' },
  { code: 'ru', name: 'Русский' },
];

async function fetchDict(code) {
  const res = await fetch(`i18n/${code}.json`);
  if (!res.ok) throw new Error(`missing dictionary ${code}`);
  return res.json();
}

export async function loadLanguage(code) {
  const next = code === 'ru' ? 'ru' : 'en';
  if (!fallback.nav) fallback = await fetchDict('en');
  dict = next === 'en' ? fallback : await fetchDict(next);
  lang = next;
  rules = new Intl.PluralRules(locale());
  document.documentElement.lang = next;
  for (const fn of listeners) fn(next);
}

// Only the browser's preferred language list is consulted; nothing leaves the machine.
export function detectLanguage() {
  const prefs = navigator.languages && navigator.languages.length ? navigator.languages : [navigator.language || 'en'];
  return prefs.some((l) => /^(ru|be)\b/i.test(l)) ? 'ru' : 'en';
}

function lookup(obj, key) {
  let cur = obj;
  for (const part of key.split('.')) {
    if (cur === null || typeof cur !== 'object') return undefined;
    cur = cur[part];
  }
  return cur;
}

export function has(key) {
  return lookup(dict, key) !== undefined || lookup(fallback, key) !== undefined;
}

export function t(key, params) {
  let value = lookup(dict, key);
  if (value === undefined) value = lookup(fallback, key);
  if (value === undefined) return key;
  if (typeof value === 'object') {
    const n = params && typeof params.n === 'number' ? params.n : 0;
    value = value[rules.select(n)] ?? value.other ?? '';
  }
  if (!params) return value;
  return value.replace(/\{(\w+)\}/g, (match, name) => (name in params ? String(params[name]) : match));
}

export function getLanguage() {
  return lang;
}

export function locale() {
  return lang === 'ru' ? 'ru-RU' : 'en-US';
}

export function onLanguageChange(fn) {
  listeners.add(fn);
  return () => listeners.delete(fn);
}
