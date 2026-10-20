// Small shared building blocks for the three pages.

import { h } from './util.js';

const segmenter = typeof Intl !== 'undefined' && Intl.Segmenter ? new Intl.Segmenter(undefined, { granularity: 'grapheme' }) : null;

export function graphemes(text) {
    return segmenter ? Array.from(segmenter.segment(text), (s) => s.segment) : Array.from(text);
}

// bouncyTitle makes a title whose letters hop one after another.
export function bouncyTitle(text, tag = 'h1') {
    let i = 0;
    const words = text.split(/(\s+)/).map((part) => {
        if (/^\s+$/.test(part) || !part) return part;
        return h('span', { style: { whiteSpace: 'nowrap', display: 'inline-block' } },
            graphemes(part).map((ch) => {
                const s = h('span', { class: 'ch', text: ch });
                s.style.setProperty('--i', i++);
                return s;
            }));
    });
    return h(tag, { class: 'title', 'aria-label': text }, words);
}

export function card(title, ...children) {
    return h('section', { class: 'card reveal' }, title && h('h2', { text: title }), children);
}

export function footer(site) {
    return h('footer', { class: 'footer', text: site.text.footer });
}

export function datePill(site) {
    return h('span', { class: 'pill' }, `🎂 ${site.bsDate} · ${site.adDate}`);
}
