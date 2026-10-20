// Hearts, bursts, confetti and buzzes.

import { h, pick, rand, reducedMotion } from './util.js';
import { track } from './api.js';

const HEARTS = ['💗', '💕', '💖', '🩷', '💞'];
let flags = {};

export function initEffects(site) {
    flags = site.flags;
    if (flags.floatingHearts && !reducedMotion) floatingHearts();
    if (flags.tapHearts) tapHearts();
}

function floatingHearts() {
    const root = document.getElementById('bg-hearts');
    for (let i = 0; i < 12; i++) {
        root.append(h('span', {
            text: pick(['💗', '🩷', '✨', '💕', '🌸']),
            style: {
                left: rand(0, 96) + 'vw',
                fontSize: rand(0.9, 1.9) + 'rem',
                animationDuration: rand(14, 26) + 's',
                animationDelay: -rand(0, 26) + 's',
            },
        }));
    }
}

function tapHearts() {
    let last = 0;
    document.addEventListener('pointerdown', (e) => {
        if (e.target.closest('input, textarea, select, .lightbox')) return;
        const t = performance.now();
        if (t - last < 70) return;
        last = t;
        heartAt(e.clientX, e.clientY);
        track('hearts');
    }, { passive: true });
}

function heartAt(x, y, emoji = pick(HEARTS)) {
    const el = h('span', { class: 'tap-heart', text: emoji, style: { left: x + 'px', top: y + 'px' } });
    el.addEventListener('animationend', () => el.remove());
    document.body.append(el);
}

// burst throws a ring of emoji out from a point.
export function burst(x, y, { n = 12, emojis = HEARTS, spread = 110 } = {}) {
    if (reducedMotion) n = Math.min(n, 4);
    for (let i = 0; i < n; i++) {
        const a = (i / n) * Math.PI * 2 + rand(-0.3, 0.3);
        const d = rand(spread * 0.5, spread);
        const el = h('span', {
            class: 'burst-bit',
            text: pick(emojis),
            style: { left: x + 'px', top: y + 'px', fontSize: rand(1, 1.8) + 'rem' },
        });
        el.style.setProperty('--dx', Math.cos(a) * d + 'px');
        el.style.setProperty('--dy', Math.sin(a) * d + 'px');
        el.style.setProperty('--rot', rand(-90, 90) + 'deg');
        el.addEventListener('animationend', () => el.remove());
        document.body.append(el);
    }
}

export function burstFrom(el, opts) {
    const r = el.getBoundingClientRect();
    burst(r.left + r.width / 2, r.top + r.height / 2, opts);
}

export function vibrate(pattern) {
    if (flags.vibrate && navigator.vibrate) {
        try { navigator.vibrate(pattern); } catch { /* not allowed here */ }
    }
}

// canvas-confetti is loaded lazily so a CDN hiccup never breaks the page.
let confettiP;
function loadConfetti() {
    confettiP ||= import('https://cdn.jsdelivr.net/npm/canvas-confetti@1.9.4/dist/confetti.module.mjs')
        .then((m) => m.default)
        .catch(() => null);
    return confettiP;
}

const PASTEL = ['#f1b4af', '#e0736b', '#fff1c9', '#cdeee0', '#ece2fb', '#ffffff', '#ffd27f'];

export async function confetti(opts = {}) {
    const c = await loadConfetti();
    if (!c) return;
    c({ particleCount: 120, spread: 90, origin: { y: 0.6 }, colors: PASTEL, disableForReducedMotion: true, ...opts });
}

export async function heartConfetti(origin) {
    const c = await loadConfetti();
    if (!c) return;
    const heart = c.shapeFromText ? c.shapeFromText({ text: '💗', scalar: 2 }) : undefined;
    c({
        particleCount: 40, spread: 100, startVelocity: 35, scalar: 2, origin,
        shapes: heart ? [heart] : undefined, disableForReducedMotion: true,
    });
}

// fireworks: confetti from both sides for a few seconds.
export async function fireworks(ms = 3000) {
    const c = await loadConfetti();
    if (!c || reducedMotion) return;
    const end = Date.now() + ms;
    (function frame() {
        c({ particleCount: 6, angle: 60, spread: 70, origin: { x: 0, y: 0.7 }, colors: PASTEL });
        c({ particleCount: 6, angle: 120, spread: 70, origin: { x: 1, y: 0.7 }, colors: PASTEL });
        if (Date.now() < end) requestAnimationFrame(frame);
    })();
}
