// Talks to the Go server: site data, clock sync and fun-stat events.

import { params } from './util.js';

let offset = 0; // server time minus device time, so a wrong phone clock can't break the countdown
let current = null;
let pending = {};
let flushTimer = null;

// now() is the site's clock (server time, or the preview time when previewing).
export const now = () => Date.now() + offset;

export async function loadSite({ at } = {}) {
    const q = new URLSearchParams();
    for (const k of ['state', 'at']) if (params.get(k)) q.set(k, params.get(k));
    if (at) {
        q.delete('state');
        q.set('at', String(at));
    }
    const t0 = Date.now();
    const res = await fetch('/api/site' + (q.size ? '?' + q : ''), { cache: 'no-store' });
    if (!res.ok) throw new Error('site: ' + res.status);
    const data = await res.json();
    offset = data.now - (t0 + Date.now()) / 2;
    current = data;
    return data;
}

// track counts things like hugs and bear pets for the /config stats page.
export function track(key, n = 1) {
    if (current?.admin) return; // the server ignores your own clicks anyway
    pending[key] = (pending[key] || 0) + n;
    flushTimer ||= setTimeout(flush, 3000);
}

export function flush() {
    clearTimeout(flushTimer);
    flushTimer = null;
    if (!Object.keys(pending).length) return;
    const body = JSON.stringify({ counts: pending });
    pending = {};
    if (!(navigator.sendBeacon && navigator.sendBeacon('/api/event', body))) {
        fetch('/api/event', { method: 'POST', body, keepalive: true }).catch(() => {});
    }
}

addEventListener('pagehide', flush);
document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'hidden') flush();
});

export async function postJSON(path, data) {
    const res = await fetch(path, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
    });
    if (!res.ok) throw new Error(await res.text());
}
