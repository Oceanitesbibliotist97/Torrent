// Small DOM helpers. Untrusted strings (torrent names, file paths, peer
// client names) only ever reach the page through textContent or attribute
// values, never through innerHTML, so a malicious torrent cannot inject markup.

import { ICONS } from './icons.js';

const PROPS = new Set(['value', 'checked', 'disabled', 'selected', 'hidden', 'tabIndex', 'indeterminate', 'readOnly', 'multiple']);

export function h(tag, props, ...children) {
  const el = document.createElement(tag);
  if (props) {
    for (const [key, value] of Object.entries(props)) {
      if (value === undefined || value === null || value === false) continue;
      if (key === 'class') el.className = value;
      else if (key === 'text') el.textContent = value;
      else if (key === 'style') for (const [p, v] of Object.entries(value)) el.style.setProperty(p, v);
      else if (key === 'dataset') Object.assign(el.dataset, value);
      else if (key.startsWith('on') && typeof value === 'function') el.addEventListener(key.slice(2).toLowerCase(), value);
      else if (PROPS.has(key)) el[key] = value;
      else el.setAttribute(key, value === true ? '' : String(value));
    }
  }
  append(el, children);
  return el;
}

export function append(el, children) {
  for (const child of children) {
    if (child === null || child === undefined || child === false) continue;
    if (Array.isArray(child)) append(el, child);
    else if (child instanceof Node) el.appendChild(child);
    else el.appendChild(document.createTextNode(String(child)));
  }
  return el;
}

const SVG_NS = 'http://www.w3.org/2000/svg';

export function icon(name, cls = '') {
  const svg = document.createElementNS(SVG_NS, 'svg');
  svg.setAttribute('class', cls ? `icon ${cls}` : 'icon');
  svg.setAttribute('viewBox', '0 0 24 24');
  svg.setAttribute('aria-hidden', 'true');
  svg.innerHTML = ICONS[name] || '';
  return svg;
}

// Parses a trusted, static SVG string from icons.js.
export function staticSvg(markup) {
  const tpl = document.createElement('template');
  tpl.innerHTML = markup.trim();
  return tpl.content.firstElementChild;
}

export function setText(el, text) {
  const value = text === undefined || text === null ? '' : String(text);
  if (el.textContent !== value) el.textContent = value;
}

export function setAttr(el, name, value) {
  if (value === null || value === undefined || value === false) {
    if (el.hasAttribute(name)) el.removeAttribute(name);
  } else {
    const v = value === true ? '' : String(value);
    if (el.getAttribute(name) !== v) el.setAttribute(name, v);
  }
}

export function debounce(fn, ms) {
  let timer;
  return (...args) => {
    clearTimeout(timer);
    timer = setTimeout(() => fn(...args), ms);
  };
}

export function isEditable(target) {
  return target instanceof HTMLElement && (target.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName));
}
