// Reusable UI building blocks: toasts, modals, menus and form controls.

import { h, icon } from './dom.js';
import { t } from './i18n.js';
import { runtime } from './api.js';

/* ---------------------------------------------------------------- Toasts */

const TOAST_ICONS = { success: 'checkCircle', info: 'info', warning: 'alertTriangle', error: 'alertCircle' };

export function toast({ kind = 'info', title, detail = '', timeout = 5000 }) {
  const root = document.getElementById('toasts');
  let el;
  const close = () => {
    if (!el.isConnected) return;
    el.classList.add('leaving');
    setTimeout(() => el.remove(), 200);
  };
  el = h('div', { class: 'toast', dataset: { kind }, role: kind === 'error' ? 'alert' : 'status' },
    h('span', { class: 'toast-icon' }, icon(TOAST_ICONS[kind] || 'info')),
    h('div', { class: 'toast-body' },
      h('div', { class: 'toast-title', text: title }),
      detail ? h('div', { class: 'toast-detail', text: detail }) : null),
    h('button', { class: 'btn btn-ghost icon-btn btn-sm', type: 'button', 'aria-label': t('common.close'), onclick: close }, icon('x', 'sm')));
  root.appendChild(el);
  while (root.children.length > 4) root.firstElementChild.remove();
  if (timeout) setTimeout(close, kind === 'error' ? timeout * 1.6 : timeout);
  return close;
}

/* ---------------------------------------------------------------- Modals */

const modals = [];

export function modalOpen() {
  return modals.length > 0;
}

function focusables(container) {
  return [...container.querySelectorAll('button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])')]
    .filter((el) => !el.disabled && el.getClientRects().length > 0);
}

export function openModal({ title, subtitle, body, footer, wide = false, onClose }) {
  const root = document.getElementById('overlay-root');
  const restoreFocus = document.activeElement;
  const titleId = `modal-title-${modals.length + 1}`;
  let closed = false;

  const dialog = h('div', { class: wide ? 'modal wide' : 'modal', role: 'dialog', 'aria-modal': 'true', 'aria-labelledby': titleId },
    h('div', { class: 'modal-header' },
      h('div', { class: 'modal-heading' },
        h('h2', { class: 'modal-title', id: titleId, text: title }),
        subtitle ? h('p', { class: 'modal-subtitle', text: subtitle }) : null),
      h('button', { class: 'btn btn-ghost icon-btn btn-sm', type: 'button', 'aria-label': t('common.close'), onclick: () => close() }, icon('x'))),
    h('div', { class: 'modal-body' }, body),
    footer ? h('div', { class: 'modal-footer' }, footer) : null);

  const backdrop = h('div', { class: 'modal-backdrop' }, dialog);
  backdrop.addEventListener('mousedown', (e) => {
    if (e.target === backdrop) close();
  });
  backdrop.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
      e.preventDefault();
      e.stopPropagation();
      close();
    } else if (e.key === 'Tab') {
      const items = focusables(dialog);
      if (!items.length) return;
      const first = items[0];
      const last = items[items.length - 1];
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    }
  });

  const handle = { dialog, close };
  function close(result) {
    if (closed) return;
    closed = true;
    backdrop.remove();
    modals.splice(modals.indexOf(handle), 1);
    if (onClose) onClose(result);
    if (restoreFocus && restoreFocus.isConnected && restoreFocus.focus) restoreFocus.focus();
  }

  root.appendChild(backdrop);
  modals.push(handle);
  requestAnimationFrame(() => {
    const preferred = dialog.querySelector('[data-autofocus]') || focusables(dialog.querySelector('.modal-body'))[0] || dialog.querySelector('.btn-primary');
    if (preferred) preferred.focus();
  });
  return handle;
}

/* ----------------------------------------------------------------- Menus */

let activeMenu = null;

export function menuOpen() {
  return activeMenu !== null;
}

export function closeMenu() {
  if (!activeMenu) return;
  activeMenu.cleanup();
  activeMenu.el.remove();
  const { restoreFocus } = activeMenu;
  activeMenu = null;
  if (restoreFocus && restoreFocus.isConnected) restoreFocus.focus();
}

// items: [{ label, icon, shortcut, danger, disabled, action } | 'sep']
export function openMenu(items, { x = 0, y = 0, anchor = null } = {}) {
  if (activeMenu && anchor && activeMenu.anchor === anchor) {
    closeMenu();
    return;
  }
  closeMenu();
  const menu = h('div', { class: 'menu', role: 'menu' });
  for (const item of items) {
    if (item === 'sep') {
      menu.appendChild(h('div', { class: 'menu-sep', role: 'separator' }));
      continue;
    }
    menu.appendChild(h('button', {
      class: item.danger ? 'menu-item danger' : 'menu-item',
      type: 'button',
      role: 'menuitem',
      disabled: !!item.disabled,
      onclick: () => {
        closeMenu();
        item.action();
      },
    }, item.icon ? icon(item.icon) : null, h('span', { class: 'menu-text', text: item.label }), item.shortcut ? h('kbd', { text: item.shortcut }) : null));
  }
  document.getElementById('overlay-root').appendChild(menu);

  let left = x;
  let top = y;
  if (anchor) {
    const r = anchor.getBoundingClientRect();
    left = r.left;
    top = r.bottom + 4;
  }
  const box = menu.getBoundingClientRect();
  if (top + box.height > window.innerHeight - 8) top = Math.max(8, (anchor ? anchor.getBoundingClientRect().top - 4 : y) - box.height);
  left = Math.max(8, Math.min(left, window.innerWidth - box.width - 8));
  menu.style.left = `${left}px`;
  menu.style.top = `${Math.max(8, top)}px`;

  const buttons = [...menu.querySelectorAll('.menu-item:not(:disabled)')];
  menu.addEventListener('keydown', (e) => {
    const i = buttons.indexOf(document.activeElement);
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      buttons[(i + 1) % buttons.length].focus();
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      buttons[(i - 1 + buttons.length) % buttons.length].focus();
    } else if (e.key === 'Escape' || e.key === 'Tab') {
      e.preventDefault();
      e.stopPropagation();
      closeMenu();
    }
  });
  const onPointer = (e) => {
    if (menu.contains(e.target) || (anchor && anchor.contains(e.target))) return;
    closeMenu();
  };
  const onBlur = () => closeMenu();
  document.addEventListener('mousedown', onPointer, true);
  window.addEventListener('blur', onBlur);
  window.addEventListener('resize', onBlur);
  activeMenu = {
    el: menu,
    anchor,
    restoreFocus: anchor || document.activeElement,
    cleanup() {
      document.removeEventListener('mousedown', onPointer, true);
      window.removeEventListener('blur', onBlur);
      window.removeEventListener('resize', onBlur);
    },
  };
  if (buttons.length) buttons[0].focus();
}

/* -------------------------------------------------------------- Controls */

export function switchControl({ checked, onChange, label, disabled = false }) {
  const input = h('input', { type: 'checkbox', role: 'switch', checked: !!checked, disabled, 'aria-label': label });
  input.addEventListener('change', () => onChange(input.checked));
  return h('label', { class: 'switch' }, input, h('span', { class: 'switch-track' }));
}

export function segmented({ options, value, onChange, label }) {
  const group = h('div', { class: 'segmented', role: 'radiogroup', 'aria-label': label });
  let current = value;
  const buttons = options.map((opt) => h('button', {
    type: 'button',
    role: 'radio',
    'aria-checked': String(opt.value === value),
    tabIndex: opt.value === value ? 0 : -1,
    onclick: () => select(opt.value, false),
  }, opt.icon ? icon(opt.icon, 'sm') : null, opt.label));
  function select(v, focus) {
    buttons.forEach((b, i) => {
      const on = options[i].value === v;
      b.setAttribute('aria-checked', String(on));
      b.tabIndex = on ? 0 : -1;
      if (on && focus) b.focus();
    });
    if (v !== current) {
      current = v;
      onChange(v);
    }
  }
  group.addEventListener('keydown', (e) => {
    const i = options.findIndex((o) => o.value === current);
    if (e.key === 'ArrowRight' || e.key === 'ArrowDown') {
      e.preventDefault();
      select(options[(i + 1) % options.length].value, true);
    } else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') {
      e.preventDefault();
      select(options[(i - 1 + options.length) % options.length].value, true);
    }
  });
  buttons.forEach((b) => group.appendChild(b));
  return group;
}

export function selectControl({ options, value, onChange, label, placeholder }) {
  const select = h('select', { class: 'select', 'aria-label': label });
  if (placeholder) select.appendChild(h('option', { value: '', text: placeholder, disabled: true, selected: !value }));
  for (const opt of options) select.appendChild(h('option', { value: opt.value, text: opt.label, selected: opt.value === value }));
  select.addEventListener('change', () => onChange(select.value));
  return h('div', { class: 'select-wrap' }, select, icon('chevronDown', 'sm'));
}

export function settingRow({ title, hint, control, stack = false }) {
  return h('div', { class: stack ? 'setting-row stack' : 'setting-row' },
    h('div', { class: 'setting-text' },
      h('div', { class: 'setting-title', text: title }),
      hint ? h('div', { class: 'setting-hint', text: hint }) : null),
    control ? h('div', { class: 'setting-control' }, control) : null);
}

export function copyButton(getText, label) {
  return h('button', {
    class: 'btn btn-ghost icon-btn btn-sm',
    type: 'button',
    title: label || t('common.copy'),
    'aria-label': label || t('common.copy'),
    onclick: async () => {
      await runtime.copy(getText());
      toast({ kind: 'success', title: t('notice.copied'), timeout: 2000 });
    },
  }, icon('copy', 'sm'));
}
