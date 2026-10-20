// The teasing week after her birthday: "SAKYO BIRTHDAY", a party-hangover
// scene to sweep up, the Budhi meter, leftover cake and a rating.

import { h, svg, rand, pick, clamp, store, replay } from '../util.js';
import { postJSON, track } from '../api.js';
import { createCountdown } from '../countdown.js';
import { createBear } from '../bear.js';
import { guard } from '../spam.js';
import { burst, burstFrom, confetti, vibrate } from '../effects.js';
import { toys } from '../toys.js';
import { bouncyTitle, card, datePill, footer } from '../ui.js';

export function renderPost(app, site) {
    const countdown = createCountdown();
    const scene = hangover(site);

    app.append(h('div', { class: 'page' },
        h('section', { class: 'hero' },
            datePill(site),
            bouncyTitle(site.text['post.title']),
            countdown.el,
            h('p', { class: 'countdown-ago', text: site.text['post.ago'] }),
            scene.el,
            h('p', { class: 'sub', text: site.text['post.sub'] }),
            h('a', { class: 'scroll-hint', href: '#after', text: site.text['pre.scroll'] })),
        h('div', { id: 'after' }, budhiMeter(site), leftoverCake(site), rating(site)),
        site.flags.toysOnBirthday && toys(site, site.text['toys.title']),
        footer(site)));

    return {
        tick(now) {
            countdown.set(now - site.end);
        },
        destroy() {
            scene.destroy();
        },
    };
}

// ---------- hangover scene ----------

const BIT_COLORS = ['#f1b4af', '#e0736b', '#fff1c9', '#cdeee0', '#ece2fb', '#9fd3f5', '#f7b733'];

function hangover(site) {
    const bear = createBear({ state: 'post' });
    if (!site.flags.hangover) return { el: bear.el, destroy: bear.destroy };

    const key = `swept.${site.bsYear}`;
    const done = store.get(key, false);
    const bits = done ? [] : Array.from({ length: 26 }, () => h('span', {
        class: 'bit',
        style: {
            left: rand(2, 95) + '%',
            bottom: rand(0.25, 2.1) + 'rem',
            background: pick(BIT_COLORS),
            transform: `rotate(${rand(0, 180)}deg)`,
        },
    }));
    const zzz = h('div', { class: 'zzz', 'aria-hidden': 'true', hidden: done }, h('span', { text: 'z' }), h('span', { text: 'z' }), h('span', { text: 'Z' }));
    const hint = h('p', { class: 'hint', role: 'status', 'aria-live': 'polite', text: done ? site.text['hangover.done'] : site.text['hangover.hint'] });
    const scene = h('div', { class: 'hangover', style: { touchAction: 'pan-y' } },
        h('div', { class: 'floor' }),
        h('span', { class: 'balloon b1' }), h('span', { class: 'balloon b2' }), h('span', { class: 'balloon b3' }),
        bits, bear.el, zzz);

    let left = bits.length;
    function sweep(e) {
        if (!left) return;
        for (const b of bits) {
            if (b.classList.contains('swept')) continue;
            const r = b.getBoundingClientRect();
            if (Math.hypot(r.left + r.width / 2 - e.clientX, r.top + r.height / 2 - e.clientY) > 36) continue;
            b.classList.add('swept');
            b.style.transform = `translate(${rand(-70, 70)}px, ${rand(-150, -60)}px) rotate(${rand(-360, 360)}deg)`;
            left--;
        }
        if (left === 0) {
            hint.textContent = site.text['hangover.done'];
            zzz.hidden = true;
            bear.say(site.text['hangover.done'], 3200);
            burstFrom(scene, { n: 14, emojis: ['✨', '🧹', '💗'], spread: 160 });
            vibrate([30, 40, 30]);
            store.set(key, true);
            track('sweep');
        }
    }
    scene.addEventListener('pointermove', sweep);
    scene.addEventListener('pointerdown', sweep);

    return { el: h('div', { style: { width: '100%' } }, scene, hint), destroy: bear.destroy };
}

// ---------- budhi meter ----------

const ACCESSORIES = [
    { e: '👓', top: '42%', left: '50%', size: '3rem' },
    { e: '☁️', top: '9%', left: '50%', size: '3.6rem' },
    { e: '🦯', top: '62%', left: '92%', size: '2.8rem' },
    { e: '🍵', top: '84%', left: '12%', size: '2.4rem' },
    { e: '🧣', top: '72%', left: '50%', size: '2.8rem' },
    { e: '📻', top: '86%', left: '88%', size: '2.2rem' },
];

function budhiMeter(site) {
    if (!site.flags.budhi) return null;
    const jokes = site.lists['budhi.jokes'] || [];
    const steps = Math.max(jokes.length, 3);
    let step = 0;

    const img = h('img', { src: '/assets/during/1.gif', alt: '', width: 192, height: 192 });
    const stage = h('div', { class: 'budhi-stage' }, img);
    const bar = h('i');
    const pct = h('span', { class: 'budhi-pct' });
    const joke = h('p', { class: 'budhi-joke', role: 'status', 'aria-live': 'polite' });
    const btn = h('button', { class: 'btn', type: 'button', text: site.text['budhi.button'] });

    function paint() {
        const p = step / steps;
        bar.style.width = Math.round(p * 100) + '%';
        pct.textContent = `${Math.round(p * 100)}% budhi`;
        img.style.filter = `sepia(${(p * 0.75).toFixed(2)}) grayscale(${(p * 0.5).toFixed(2)})`;
    }

    btn.addEventListener('click', () => {
        if (step >= steps) {
            step = 0;
            stage.querySelectorAll('.acc').forEach((a) => a.remove());
            joke.textContent = 'Feri jawan! ✨';
            btn.textContent = site.text['budhi.button'];
            burstFrom(stage, { n: 12, emojis: ['✨', '🌸', '💗'] });
            paint();
            return;
        }
        if (!guard('budhi', btn)) return;
        const a = ACCESSORIES[step % ACCESSORIES.length];
        stage.append(h('span', { class: 'acc', text: a.e, style: { top: a.top, left: a.left, fontSize: a.size } }));
        joke.textContent = jokes[step] || '👵';
        replay(joke, 'pop');
        step++;
        paint();
        vibrate(10);
        track('budhi');
        if (step >= steps) {
            joke.textContent = site.text['budhi.done'];
            btn.textContent = 'Feri jawan banau ✨';
            confetti({ particleCount: 80, spread: 70, origin: { y: 0.6 } });
        }
    });

    paint();
    return card(site.text['budhi.title'],
        site.age > 0 && h('p', { class: 'hint', text: `${site.age} barsa pugisakyau 😌` }),
        stage, h('div', { class: 'budhi-bar' }, bar), pct, joke, btn);
}

// ---------- leftover cake ----------

function leftoverCake(site) {
    if (!site.flags.leftover) return null;
    const lines = site.lists['leftover.slices'] || [];
    const n = clamp(lines.length || 6, 3, 10);
    const key = `slices.${site.bsYear}`;
    let eaten = clamp(store.get(key, 0), 0, n);
    const total = n + 2; // two slices already went at the party
    const C = 100;
    const R = 90;
    const at = (r, a) => `${(C + r * Math.cos(a)).toFixed(2)},${(C + r * Math.sin(a)).toFixed(2)}`;
    const msg = h('p', { class: 'leftover-msg', role: 'status', 'aria-live': 'polite' });
    const again = h('button', { class: 'btn btn-soft', type: 'button', text: 'Arko cake? 🎂', hidden: eaten < n });

    const slices = Array.from({ length: n }, (_, k) => {
        const a0 = ((k + 2) / total) * Math.PI * 2 - Math.PI / 2;
        const a1 = ((k + 3) / total) * Math.PI * 2 - Math.PI / 2;
        const mid = (a0 + a1) / 2;
        const g = svg('g', { class: 'slice', role: 'button', tabindex: 0, 'aria-label': `Cake slice ${k + 1}` },
            svg('path', { d: `M${C},${C} L${at(R, a0)} A${R},${R} 0 0 1 ${at(R, a1)} Z`, fill: '#fde2df', stroke: '#fff8f3', 'stroke-width': 3 }),
            svg('path', { d: `M${at(R, a0)} A${R},${R} 0 0 1 ${at(R, a1)} L${at(70, a1)} A70,70 0 0 0 ${at(70, a0)} Z`, fill: '#f1b4af' }),
            svg('circle', { cx: C + 80 * Math.cos(mid), cy: C + 80 * Math.sin(mid), r: 7, fill: '#e0566b' }),
            svg('circle', { cx: C + 45 * Math.cos(mid), cy: C + 45 * Math.sin(mid), r: 3, fill: '#fff' }));
        if (k < eaten) g.classList.add('eaten');
        g.addEventListener('click', (e) => eat(g, e));
        g.addEventListener('keydown', (e) => {
            if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault();
                eat(g, e);
            }
        });
        return g;
    });

    function eat(g, e) {
        if (g.classList.contains('eaten')) return;
        g.classList.add('eaten');
        const line = lines[eaten] || pick(['Yum! 😋', 'Mmm 🍰']);
        eaten++;
        store.set(key, eaten);
        msg.textContent = line;
        replay(msg, 'pop');
        const r = g.getBoundingClientRect();
        burst(e.clientX || r.left + r.width / 2, e.clientY || r.top + r.height / 2, { n: 6, emojis: ['😋', '🍰', '✨', '🤎'], spread: 70 });
        vibrate(10);
        track('slice');
        if (eaten >= n) again.hidden = false;
    }

    again.addEventListener('click', () => {
        eaten = 0;
        store.set(key, 0);
        slices.forEach((g) => g.classList.remove('eaten'));
        msg.textContent = 'Naya cake aayo! 🎂';
        again.hidden = true;
    });

    const plate = svg('svg', { viewBox: '-4 -4 208 208', 'aria-label': 'Leftover cake' },
        svg('circle', { cx: C, cy: C, r: 100, fill: '#fff', stroke: '#f1b4af', 'stroke-width': 3 }),
        svg('circle', { cx: 70, cy: 52, r: 2.5, fill: '#e8c39c' }),
        svg('circle', { cx: 84, cy: 40, r: 1.8, fill: '#e8c39c' }),
        svg('circle', { cx: 60, cy: 66, r: 1.5, fill: '#e8c39c' }),
        slices);

    const el = card(site.text['leftover.title'], h('p', { class: 'hint', text: site.text['leftover.hint'] }), plate, msg, again);
    el.classList.add('leftover');
    return el;
}

// ---------- rate your birthday ----------

function rating(site) {
    if (!site.flags.rating) return null;
    const key = `rated.${site.bsYear}`;
    const prev = store.get(key, null);
    let stars = prev?.stars || 0;
    let nope = 0;

    const buttons = [1, 2, 3, 4, 5].map((n) => h('button', { class: 'star', type: 'button', 'aria-label': `${n} star${n > 1 ? 's' : ''}`, text: '★' }));
    const row = h('div', { class: 'stars', role: 'group', 'aria-label': 'Rating' }, buttons);
    const note = h('p', { class: 'rate-note', role: 'status', 'aria-live': 'polite' });
    const review = h('textarea', { class: 'field', rows: 3, maxlength: 300, placeholder: site.text['rate.placeholder'], 'aria-label': 'Review' });
    const send = h('button', { class: 'btn', type: 'button', text: site.text['rate.button'] });
    const thanks = h('p', { class: 'hint', role: 'status', hidden: true, text: site.text['rate.thanks'], style: { marginTop: '.75rem' } });

    const paint = () => buttons.forEach((b, i) => {
        b.classList.toggle('on', i < stars);
        b.setAttribute('aria-pressed', String(i < stars));
    });

    buttons.forEach((b, i) => b.addEventListener('click', () => {
        const n = i + 1;
        // Low ratings get "corrected" twice before the bear gives up.
        if (n < 4 && nope < 2) {
            nope++;
            stars = 5;
            replay(row, 'nope');
            note.textContent = nope === 1 ? 'Galti le thichyau hola, 5 nai ho 😌' : 'Pakka? 🥺 Feri soch ta...';
        } else {
            stars = n;
            note.textContent = n === 5 ? 'Yayyy! 🥰' : n === 4 ? '4 matra? 🥺' : 'Hmm... thik chha 🥲';
        }
        paint();
    }));

    send.addEventListener('click', async () => {
        if (!stars) {
            replay(row, 'nope');
            note.textContent = 'Paila star deu na ⭐';
            return;
        }
        send.disabled = true;
        try {
            await postJSON('/api/rating', { stars, review: review.value.trim() });
            store.set(key, { stars });
            finish();
            confetti({ particleCount: 100, spread: 90, origin: { y: 0.7 } });
        } catch {
            note.textContent = 'Pathauna sakena 🥺 Feri try gara';
        } finally {
            send.disabled = false;
        }
    });

    function finish() {
        review.hidden = true;
        send.hidden = true;
        thanks.hidden = false;
        buttons.forEach((b) => { b.disabled = true; });
    }

    paint();
    if (prev) finish();
    return card(site.text['rate.title'], row, note, review, h('div', { style: { marginTop: '.75rem' } }, send), thanks);
}
