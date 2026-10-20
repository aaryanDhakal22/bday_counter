// The toys: hug meter, the runaway-No question, the reasons jar and the
// polaroid wall. They show before, during and after the birthday.

import { h, svg, rand, shuffle, store, replay, center } from './util.js';
import { guard } from './spam.js';
import { track } from './api.js';
import { burst, burstFrom, confetti, heartConfetti, vibrate } from './effects.js';
import { card } from './ui.js';

const HEART = 'M50 88 C 22 68 4 50 4 30 C 4 15 15 5 28 5 C 38 5 46 11 50 19 C 54 11 62 5 72 5 C 85 5 96 15 96 30 C 96 50 78 68 50 88 Z';

// ---------- hug meter ----------

export function hugMeter(site) {
    const milestones = (site.milestones || []).slice().sort((a, b) => a.count - b.count);
    let count = store.get('hugs', 0);
    const clipId = 'hug-clip-' + Math.random().toString(36).slice(2, 8);

    const fill = svg('rect', { class: 'fill', x: 0, y: 0, width: 100, height: 92, fill: '#f1b4af' });
    const heartSvg = svg('svg', { viewBox: '0 0 100 92', 'aria-hidden': 'true' },
        svg('defs', null, svg('clipPath', { id: clipId }, svg('path', { d: HEART }))),
        svg('path', { d: HEART, fill: '#fff4f2' }),
        svg('g', { 'clip-path': `url(#${clipId})` }, fill),
        svg('path', { d: HEART, fill: 'none', stroke: '#e0736b', 'stroke-width': 3 }));
    const countEl = h('div', { class: 'hug-count' });
    const heart = h('div', { class: 'hug-heart' }, heartSvg, countEl);
    const level = h('div', { class: 'hug-level' });
    const msg = h('p', { class: 'hug-msg', role: 'status', 'aria-live': 'polite', text: count ? '' : 'Thich na, euta hug deu 🤗' });
    const btn = h('button', { class: 'btn hug-btn', type: 'button', text: site.text['hug.button'] });

    // Milestones from /config, then a level every 500 hugs after the last one.
    function progress() {
        let prev = 0;
        let lvl = 1;
        for (const m of milestones) {
            if (count < m.count) return { prev, next: m.count, lvl };
            prev = m.count;
            lvl++;
        }
        const extra = Math.floor((count - prev) / 500);
        return { prev: prev + extra * 500, next: prev + (extra + 1) * 500, lvl: lvl + extra };
    }

    function draw() {
        const p = progress();
        const pct = (count - p.prev) / (p.next - p.prev);
        countEl.textContent = count;
        fill.style.transform = `translateY(${((1 - pct) * 92).toFixed(1)}px)`;
        level.textContent = `Level ${p.lvl} · arko ${p.next} ma`;
    }

    btn.addEventListener('click', (e) => {
        if (!guard('hug', btn)) return;
        const before = progress();
        count++;
        store.set('hugs', count);
        track('hug');
        replay(heart, 'pulse');
        const plus = h('span', { class: 'plus-one', text: '+1', style: { left: rand(30, 60) + '%', top: '20%' } });
        plus.addEventListener('animationend', () => plus.remove());
        heart.append(plus);
        vibrate(12);
        burst(e.clientX || center(btn).x, e.clientY || center(btn).y, { n: 4, spread: 60 });

        const hit = milestones.find((m) => m.count === count);
        if (hit || count === before.next) {
            msg.textContent = hit ? hit.text : `${count} hugs! Level up 🎉`;
            replay(msg, 'pop');
            burstFrom(heart, { n: 18, spread: 150 });
            const c = center(heart);
            confetti({ particleCount: 70, origin: { x: c.x / innerWidth, y: c.y / innerHeight } });
            vibrate([30, 50, 30, 50, 60]);
        }
        draw();
    });

    draw();
    return card(site.text['hug.title'], heart, level, msg, btn);
}

// ---------- will you be mine forever? ----------

export function question(site) {
    const pleas = site.lists['question.pleas'] || [];
    const yes = h('button', { class: 'btn q-yes', type: 'button', text: site.text['question.yes'] });
    const no = h('button', { class: 'btn q-no', type: 'button', text: site.text['question.no'] });
    const arena = h('div', { class: 'q-arena' }, yes, no);
    const after = h('p', { class: 'q-after', role: 'status', 'aria-live': 'polite' });
    let dodges = 0;
    let noScale = 1;
    let yesScale = 1;
    let lastDodge = 0;

    function dodge(e) {
        e?.preventDefault?.();
        const t = performance.now();
        if (t - lastDodge < 350) return; // a tap fires pointerdown and click
        lastDodge = t;
        dodges++;
        track('no');

        const a = arena.getBoundingClientRect();
        const n = no.getBoundingClientRect();
        const y = yes.getBoundingClientRect();
        let spot;
        for (let i = 0; i < 20; i++) {
            const px = rand(16, 84);
            const py = rand(14, 86);
            const x = a.left + (a.width * px) / 100;
            const yy = a.top + (a.height * py) / 100;
            const farFromHere = Math.hypot(x - (n.left + n.width / 2), yy - (n.top + n.height / 2)) > 90;
            const clearOfYes = Math.abs(x - (y.left + y.width / 2)) > (y.width + n.width) / 2 + 8
                || Math.abs(yy - (y.top + y.height / 2)) > (y.height + n.height) / 2 + 8;
            spot = [px, py];
            if (farFromHere && clearOfYes) break;
        }
        no.style.left = spot[0] + '%';
        no.style.top = spot[1] + '%';
        noScale = Math.max(0.45, noScale * 0.9);
        yesScale = Math.min(1.7, yesScale + 0.07);
        no.style.transform = `scale(${noScale})`;
        yes.style.transform = `scale(${yesScale})`;
        if (pleas.length) no.textContent = pleas[(dodges - 1) % pleas.length];
    }

    no.addEventListener('pointerdown', dodge);
    no.addEventListener('pointerenter', (e) => { if (e.pointerType === 'mouse') dodge(e); });
    no.addEventListener('click', dodge);

    yes.addEventListener('click', () => {
        if (!guard('yes', yes)) return;
        burstFrom(yes, { n: 16, spread: 140 });
        const c = center(yes);
        heartConfetti({ x: c.x / innerWidth, y: c.y / innerHeight });
        vibrate([20, 30, 20]);
        after.textContent = site.text['question.after'];
        replay(after, 'pop');
        track('yes');
    });

    return card(null, h('p', { class: 'q-text', text: site.text['question.text'] }), arena, after);
}

// ---------- reasons jar ----------

const NOTE_COLORS = ['#fff1c9', '#fde7e4', '#cdeee0', '#ece2fb', '#dff0ff'];

export function reasonsJar(site) {
    const reasons = site.lists.reasons || [];
    if (!reasons.length) return null;

    const notes = Array.from({ length: 14 }, (_, i) => h('span', {
        class: 'jar-note',
        style: {
            left: rand(4, 64) + '%',
            top: rand(38, 82) + '%',
            background: NOTE_COLORS[i % NOTE_COLORS.length],
            transform: `rotate(${rand(-40, 40)}deg)`,
        },
    }));
    const jar = h('button', { class: 'jar', type: 'button', 'aria-label': 'Take a note from the jar' },
        h('div', { class: 'jar-lid' }),
        h('div', { class: 'jar-body' }, notes, h('span', { class: 'jar-heart-label', text: '💗 maya' })));
    const paper = h('div', { class: 'jar-paper', role: 'status', 'aria-live': 'polite', hidden: true });
    const countEl = h('p', { class: 'jar-count' });

    // A shuffled bag, so she reads every reason before any repeats.
    let order = store.get('jar.order', null);
    let pos = store.get('jar.pos', 0);
    if (!Array.isArray(order) || order.length !== reasons.length) {
        order = shuffle(reasons.map((_, i) => i));
        pos = 0;
    }
    const showCount = () => { countEl.textContent = `${Math.min(pos, reasons.length)} / ${reasons.length} padhyau`; };

    jar.addEventListener('click', () => {
        if (!guard('jar', jar)) return;
        replay(jar, 'wiggle');
        let text;
        if (pos >= order.length) {
            text = site.text['jar.empty'];
            order = shuffle(order);
            pos = 0;
        } else {
            text = reasons[order[pos++]];
        }
        store.set('jar.order', order);
        store.set('jar.pos', pos);
        showCount();
        flyNote(jar);
        setTimeout(() => {
            paper.textContent = text;
            paper.hidden = false;
            replay(paper, 'unfold');
        }, 380);
        track('jar');
    });

    showCount();
    return card(site.text['jar.title'], h('p', { class: 'hint', text: site.text['jar.hint'] }), jar, paper, countEl);
}

function flyNote(jar) {
    const r = jar.getBoundingClientRect();
    const note = h('span', { class: 'flying-note', style: { left: r.left + r.width / 2 - 18 + 'px', top: r.top + 10 + 'px' } });
    document.body.append(note);
    const anim = note.animate([
        { transform: 'translate(0, 0) rotate(0) scale(1)', opacity: 1 },
        { transform: `translate(${rand(-30, 30)}px, -90px) rotate(${rand(-200, 200)}deg) scale(1.3)`, opacity: 1, offset: 0.6 },
        { transform: 'translate(0, 160px) rotate(0) scale(2)', opacity: 0 },
    ], { duration: 520, easing: 'ease-out' });
    anim.onfinish = () => note.remove();
}

// ---------- polaroid wall ----------

export function polaroidWall(site) {
    const photos = site.photos || [];
    if (!site.flags.photos || !photos.length) return null;
    const grid = h('div', { class: 'polaroids' }, photos.map((p, i) => {
        const b = h('button', { class: 'polaroid', type: 'button', 'aria-label': p.caption || `Photo ${i + 1}` },
            h('img', { src: p.src, alt: '', loading: 'lazy' }),
            h('span', { text: p.caption }));
        b.style.setProperty('--tilt', rand(-4, 4).toFixed(1) + 'deg');
        b.addEventListener('click', () => {
            if (!guard('photos', b)) return;
            openLightbox(photos, i);
            track('photo');
        });
        return b;
    }));
    return card(site.text['photos.title'], grid);
}

export function openLightbox(photos, start) {
    let i = start;
    const img = h('img', { alt: '' });
    const cap = h('figcaption');
    const prev = h('button', { class: 'lb-btn', type: 'button', 'aria-label': 'Previous photo', text: '←' });
    const next = h('button', { class: 'lb-btn', type: 'button', 'aria-label': 'Next photo', text: '→' });
    const close = h('button', { class: 'lb-btn lb-close', type: 'button', 'aria-label': 'Close', text: '✕' });
    const box = h('div', { class: 'lightbox', role: 'dialog', 'aria-modal': 'true', 'aria-label': 'Photos' },
        close, h('figure', null, img, cap), h('div', { class: 'lb-nav' }, prev, next));

    const show = () => {
        img.src = photos[i].src;
        cap.textContent = photos[i].caption || '';
    };
    const go = (d) => {
        i = (i + d + photos.length) % photos.length;
        show();
    };
    const onKey = (e) => {
        if (e.key === 'Escape') shut();
        else if (e.key === 'ArrowLeft') go(-1);
        else if (e.key === 'ArrowRight') go(1);
    };
    function shut() {
        box.remove();
        removeEventListener('keydown', onKey);
        document.documentElement.style.overflow = '';
    }

    prev.addEventListener('click', () => go(-1));
    next.addEventListener('click', () => go(1));
    close.addEventListener('click', shut);
    box.addEventListener('click', (e) => { if (e.target === box) shut(); });
    let startX = null;
    box.addEventListener('pointerdown', (e) => { startX = e.clientX; });
    box.addEventListener('pointerup', (e) => {
        if (startX != null && Math.abs(e.clientX - startX) > 50) go(e.clientX < startX ? 1 : -1);
        startX = null;
    });
    addEventListener('keydown', onKey);
    if (photos.length < 2) prev.hidden = next.hidden = true;

    show();
    document.documentElement.style.overflow = 'hidden';
    document.body.append(box);
    close.focus();
}

// toys bundles everything that's switched on.
export function toys(site, heading) {
    const parts = [
        site.flags.hug && hugMeter(site),
        site.flags.question && question(site),
        site.flags.jar && reasonsJar(site),
        polaroidWall(site),
    ].filter(Boolean);
    if (!parts.length) return null;
    return h('div', { id: 'toys' }, heading && h('h2', { class: 'section-title', text: heading }), parts);
}
