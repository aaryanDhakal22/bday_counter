// The daily envelope: a sealed note she taps open, once per day.

import { h, params, store } from './util.js';
import { track } from './api.js';

function noteFor(site) {
    if (site.state === 'during') return site.text['envelope.birthday'];
    const post = site.lists['envelope.post'];
    const list = site.state === 'post' && post?.length ? post : site.lists['envelope.notes'];
    return list?.length ? list[site.dayIndex % list.length] : '';
}

let active = null; // the envelope on screen, if any

// maybeShowEnvelope resolves once the envelope is closed (or right away if
// she has already opened today's). A new one replaces any still open, e.g.
// when the page flips to her birthday at midnight.
export function maybeShowEnvelope(site) {
    if (active) {
        active.overlay.remove();
        active.resolve();
        active = null;
        document.documentElement.style.overflow = '';
    }
    return new Promise((resolve) => {
        const key = `envelope.${site.state}.${site.day}`;
        const note = noteFor(site);
        if (!site.flags.envelope || !note || (store.get(key) && params.get('envelope') !== 'again')) {
            resolve();
            return;
        }

        const letter = h('div', { class: 'env-letter' },
            h('p', { class: 'env-note', text: note }),
            h('span', { class: 'env-sign', text: site.from ? `— ${site.from} 💗` : '💗' }));
        const envelope = h('button', { class: 'envelope', type: 'button', 'aria-label': 'Open the envelope' },
            h('div', { class: 'env-back' }), letter, h('div', { class: 'env-front' }),
            h('div', { class: 'env-flap' }), h('div', { class: 'env-seal', text: '💗' }));
        const hint = h('p', { class: 'env-hint', text: site.text['envelope.hint'] });
        const go = h('button', { class: 'btn env-go', type: 'button', text: site.text['envelope.button'], hidden: true });
        const overlay = h('div', { class: 'env-overlay', role: 'dialog', 'aria-modal': 'true', 'aria-label': site.text['envelope.to'] },
            h('div', { class: 'env-wrap' },
                h('div', { class: 'env-to', text: site.text['envelope.to'] }),
                envelope, hint, go));

        envelope.addEventListener('click', () => {
            if (envelope.classList.contains('open')) return;
            envelope.classList.add('open');
            hint.style.visibility = 'hidden';
            go.hidden = false;
            requestAnimationFrame(() => go.classList.add('show'));
            setTimeout(() => go.focus({ preventScroll: true }), 1300);
            track('envelope');
        });
        go.addEventListener('click', () => {
            store.set(key, 1);
            active = null;
            overlay.classList.add('gone');
            document.documentElement.style.overflow = '';
            setTimeout(() => overlay.remove(), 600);
            resolve();
        });

        active = { overlay, resolve };
        document.documentElement.style.overflow = 'hidden';
        document.body.append(overlay);
    });
}
