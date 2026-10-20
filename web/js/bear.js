// The Bubu bear: tap it to pet it. It squishes, talks and swaps GIFs.
// It also runs the grumpy toast that pops up when she spams a toy.

import { h, pick, pickNew, replay } from './util.js';
import { guard } from './spam.js';
import { track } from './api.js';
import { burst, vibrate } from './effects.js';

let site;
export function initBear(s) {
    site = s;
}

const GRUMPY_GIF = '/assets/pre/1.gif';
const FORGIVE_GIF = '/assets/pre/3.gif';

export const gifsFor = (state) => [1, 2, 3].map((n) => `/assets/${state}/${n}.gif`);

export function createBear({ state = site.state, gif = null, small = false } = {}) {
    const list = gifsFor(state);
    let idx = Math.floor(Math.random() * list.length);
    const interactive = site.flags.bear;
    const img = h('img', { src: gif || list[idx], alt: '', width: 200, height: 200, draggable: 'false' });
    const plate = interactive
        ? h('button', { class: 'bear-plate', type: 'button', 'aria-label': 'Pet the bear' }, img)
        : h('div', { class: 'bear-plate' }, img);
    const bubble = h('div', { class: 'bubble', role: 'status', 'aria-live': 'polite' });
    const el = h('div', { class: 'bear' + (small ? ' small' : '') }, bubble, plate);
    let hideTimer;
    let lastLine;

    function say(text, ms = 2600) {
        if (!text) return;
        bubble.textContent = text;
        bubble.classList.add('show');
        clearTimeout(hideTimer);
        hideTimer = setTimeout(() => bubble.classList.remove('show'), ms);
    }

    function swap() {
        if (gif || list.length < 2) return;
        idx = (idx + 1 + Math.floor(Math.random() * (list.length - 1))) % list.length;
        img.src = list[idx];
    }

    if (interactive) {
        plate.addEventListener('click', (e) => {
            if (!guard('bear', plate)) return;
            replay(plate, 'squish');
            lastLine = pickNew(site.lists['bear.lines'], lastLine);
            say(lastLine);
            swap();
            const r = plate.getBoundingClientRect();
            burst(e.clientX || r.left + r.width / 2, e.clientY || r.top + r.height / 2, { n: 6, spread: 70 });
            vibrate(12);
            track('bear');
        });
    }

    const every = site.nums['bear.autoSwap'];
    const timer = every > 0 && !gif ? setInterval(swap, every * 1000) : null;

    return {
        el, img, plate, say, swap,
        destroy() {
            clearInterval(timer);
            clearTimeout(hideTimer);
        },
    };
}

let toast;
let toastTimer;

// grumpy shows the cross bear for lockMs, then it forgives her.
export function grumpy(lockMs) {
    if (!toast) {
        toast = h('div', { class: 'toast-bear', role: 'status', 'aria-live': 'polite' },
            h('div', { class: 'msg' }),
            h('img', { alt: '', src: GRUMPY_GIF, width: 88, height: 88 }));
        document.body.append(toast);
    }
    const [msg, img] = toast.children;
    img.src = GRUMPY_GIF;
    msg.textContent = pick(site.lists['bear.grumpy'] || []) || 'Basa na! 😤';
    toast.classList.add('show');
    vibrate([60, 40, 60]);
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => {
        img.src = FORGIVE_GIF;
        msg.textContent = pick(site.lists['bear.forgive'] || []) || '🥺💗';
        toastTimer = setTimeout(() => toast.classList.remove('show'), 2400);
    }, lockMs);
}
