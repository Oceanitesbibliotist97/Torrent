// Shared user actions used by the list, context menu, details panel and shortcuts.

import { api, errorText, runtime } from './api.js';
import { setState } from './store.js';
import { toast } from './ui.js';
import { t } from './i18n.js';

export async function refreshState() {
  try {
    setState(await api.state());
  } catch {
    // The periodic state event will catch up.
  }
}

export async function run(promise) {
  try {
    return await promise;
  } catch (err) {
    toast({ kind: 'error', title: errorText(err), detail: err && err.detail ? err.detail : '' });
    return undefined;
  } finally {
    refreshState();
  }
}

const INACTIVE = new Set(['paused', 'completed', 'error']);

export function canPause(torrent) {
  return !INACTIVE.has(torrent.state);
}

export function pause(ids) {
  return ids.length ? run(api.pause(ids)) : undefined;
}

export function resume(ids) {
  return ids.length ? run(api.resume(ids)) : undefined;
}

// toggle pauses the selection if anything in it is active, otherwise resumes it.
export function toggle(torrents) {
  const active = torrents.filter(canPause);
  if (active.length) return pause(active.map((x) => x.id));
  return resume(torrents.map((x) => x.id));
}

export async function copyMagnet(id) {
  const link = await run(api.magnetLink(id));
  if (!link) return;
  await runtime.copy(link);
  toast({ kind: 'success', title: t('notice.copied'), timeout: 2000 });
}

export async function copyText(text) {
  await runtime.copy(text);
  toast({ kind: 'success', title: t('notice.copied'), timeout: 2000 });
}

export function openFolder(id) {
  return run(api.openFolder(id));
}

export function recheck(id) {
  return run(api.recheck(id));
}
