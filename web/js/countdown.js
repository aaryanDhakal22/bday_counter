// Flip-card countdown: din / ghanta / minet / sekend.

import { h } from './util.js';

const UNITS = [['d', 'din'], ['h', 'ghanta'], ['m', 'minet'], ['s', 'sekend']];

export function createCountdown() {
    const cards = {};
    const last = {};
    const sr = h('span', { class: 'sr-only' });
    const el = h('div', { class: 'countdown', role: 'timer' },
        UNITS.map(([key, label]) => {
            cards[key] = h('div', { class: 'flip-num', text: '00', 'aria-hidden': 'true' });
            return h('div', { class: 'flip' }, cards[key], h('div', { class: 'flip-label', text: label, 'aria-hidden': 'true' }));
        }),
        sr);

    function set(ms) {
        const total = Math.floor(Math.max(0, ms) / 1000);
        const v = {
            d: Math.floor(total / 86400),
            h: Math.floor(total / 3600) % 24,
            m: Math.floor(total / 60) % 60,
            s: total % 60,
        };
        for (const [key] of UNITS) {
            const text = String(v[key]).padStart(2, '0');
            if (last[key] === text) continue;
            const card = cards[key];
            if (last[key] === undefined) {
                card.textContent = text;
            } else {
                card.classList.remove('flipping');
                void card.offsetWidth;
                card.classList.add('flipping');
                setTimeout(() => { card.textContent = text; }, 250);
            }
            last[key] = text;
        }
        sr.textContent = `${v.d} din ${v.h} ghanta ${v.m} minet`;
    }

    return { el, set };
}
