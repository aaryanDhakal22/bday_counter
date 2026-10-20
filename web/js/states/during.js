// Her birthday. The lights go down, she blows out the candles (mic or tap),
// the lights come up with confetti and a music box, and the surprises
// unlock: wish lantern, gift box, letter, slideshow.

import { h, rand, pick, store, replay, onVisible, revealOnScroll } from '../util.js';
import { postJSON, track } from '../api.js';
import { createBear } from '../bear.js';
import { createCake, micSupported } from '../cake.js';
import { playBirthdaySong, stopSong } from '../music.js';
import { burstFrom, confetti, fireworks, vibrate } from '../effects.js';
import { toys } from '../toys.js';
import { bouncyTitle, card, datePill, footer, graphemes } from '../ui.js';

const BUNTING = ['#f1b4af', '#fff1c9', '#cdeee0', '#ece2fb', '#dff0ff', '#e0736b'];

export function renderDuring(app, site) {
    const blownKey = `during.${site.bsYear}.blown`;
    let blown = store.get(blownKey, false);

    const bear = createBear({ state: 'during', small: true });
    const titleSlot = h('div');
    const age = site.age > 0 && site.text['during.age'] ? h('p', { class: 'age-line', text: site.text['during.age'] }) : null;
    const sub = h('p', { class: 'sub', text: site.text['during.sub'] });
    const hint = h('p', { class: 'hint' });
    const meter = h('div', { class: 'mic-meter', hidden: true }, h('i'));
    const micBtn = site.flags.mic && micSupported()
        ? h('button', { class: 'btn btn-soft', type: 'button', text: site.text['cake.mic'] })
        : null;
    const relight = h('button', { class: 'btn btn-soft', type: 'button', text: site.text['cake.again'], hidden: true });
    const scrollHint = h('a', { class: 'scroll-hint', href: '#surprises', text: '👇 Surprise haru tala chhan', hidden: true });
    const musicBtn = site.flags.music ? h('button', { class: 'music-btn', type: 'button', 'aria-label': 'Play the birthday song', text: '🎵', hidden: true }) : null;
    const cake = createCake(site, { onAllOut: celebrate });
    const bunting = h('div', { class: 'bunting', 'aria-hidden': 'true' }, Array.from({ length: 18 }, (_, i) => {
        const s = h('span', { style: { background: BUNTING[i % BUNTING.length] } });
        s.style.setProperty('--i', i);
        return s;
    }));

    const slides = slideshow(site);
    const rest = h('div', { id: 'surprises', hidden: !blown },
        wishCard(site),
        giftCard(site),
        letterCard(site),
        slides?.el,
        site.flags.toysOnBirthday && toys(site, site.text['toys.title']),
        footer(site));

    app.append(h('div', { class: 'page' },
        h('section', { class: 'party' },
            h('div', { class: 'party-sky', 'aria-hidden': 'true' }),
            bunting,
            datePill(site),
            titleSlot,
            age,
            cake.el,
            hint,
            micBtn,
            meter,
            bear.el,
            sub,
            relight,
            scrollHint),
        rest));
    if (musicBtn) document.body.append(musicBtn);

    function setTitle(text) {
        titleSlot.replaceChildren(bouncyTitle(text));
    }

    function lightsOff() {
        document.body.classList.add('lights-off');
        setTitle(site.text['cake.title']);
        hint.textContent = site.text['cake.hint'];
        for (const el of [age, sub, bear.el, relight, scrollHint]) if (el) el.hidden = true;
        if (micBtn) {
            micBtn.hidden = false;
            micBtn.disabled = false;
            micBtn.textContent = site.text['cake.mic'];
        }
    }

    function lightsOn() {
        document.body.classList.remove('lights-off');
        setTitle(site.text['during.title']);
        hint.textContent = '';
        meter.hidden = true;
        if (micBtn) micBtn.hidden = true;
        for (const el of [age, sub, bear.el, relight]) if (el) el.hidden = false;
        rest.hidden = false;
        revealOnScroll(rest);
        if (musicBtn) musicBtn.hidden = false;
    }

    function setMusic(on) {
        if (!musicBtn) return;
        musicBtn.classList.toggle('playing', on);
        musicBtn.setAttribute('aria-label', on ? 'Stop the music' : 'Play the birthday song');
    }

    function playSong() {
        if (site.flags.music && playBirthdaySong(() => setMusic(false))) setMusic(true);
    }

    function celebrate() {
        lightsOn();
        fireworks(3500);
        confetti({ particleCount: 220, spread: 130, startVelocity: 45, origin: { y: 0.55 } });
        vibrate([60, 60, 120]);
        playSong();
        setTimeout(() => bear.say(pick(['Happy birthday! 🎂', `${site.name}! 💗`, 'Yayyy 🎉'])), 900);
        if (!blown) {
            blown = true;
            store.set(blownKey, true);
            scrollHint.hidden = false;
        }
    }

    musicBtn?.addEventListener('click', () => {
        if (musicBtn.classList.contains('playing')) {
            stopSong();
            setMusic(false);
        } else {
            playSong();
        }
    });

    micBtn?.addEventListener('click', async () => {
        micBtn.disabled = true;
        try {
            await cake.startMic(meter.firstChild);
            micBtn.textContent = '🎤 Sunirachhu... phuu gara!';
            meter.hidden = false;
        } catch {
            micBtn.hidden = true;
            hint.textContent = 'Mic chalena 🙈 Candle ma tap gara';
        }
    });

    relight.addEventListener('click', () => {
        stopSong();
        setMusic(false);
        cake.relight();
        lightsOff();
        relight.scrollIntoView({ block: 'center', behavior: 'smooth' });
    });

    if (blown) {
        cake.putOut();
        lightsOn();
    } else {
        lightsOff();
    }

    return {
        start() {
            if (!blown) confetti({ particleCount: 80, spread: 100, origin: { y: 0.3 } });
        },
        destroy() {
            document.body.classList.remove('lights-off');
            cake.stopMic();
            stopSong();
            bear.destroy();
            slides?.destroy();
            musicBtn?.remove();
        },
    };
}

// ---------- make a wish ----------

function wishCard(site) {
    if (!site.flags.wish) return null;
    const input = h('textarea', { class: 'field', rows: 3, maxlength: 500, placeholder: site.text['wish.placeholder'], 'aria-label': site.text['wish.title'] });
    const send = h('button', { class: 'btn', type: 'button', text: site.text['wish.button'] });
    const note = h('p', { class: 'hint', role: 'status', 'aria-live': 'polite', style: { marginTop: '.75rem' } });

    send.addEventListener('click', async () => {
        const text = input.value.trim();
        if (!text) {
            replay(input, 'shake');
            input.focus();
            return;
        }
        send.disabled = true;
        try {
            await postJSON('/api/wish', { text });
            lantern(send, text);
            input.value = '';
            note.textContent = site.text['wish.done'];
        } catch {
            note.textContent = 'Internet le dhoka diyo 🥺 Feri try gara';
        } finally {
            send.disabled = false;
        }
    });

    const el = card(site.text['wish.title'], input, h('div', { style: { marginTop: '.75rem' } }, send), note);
    el.classList.add('wish');
    return el;
}

function lantern(from, text) {
    const r = from.getBoundingClientRect();
    const el = h('div', { class: 'lantern', style: { left: r.left + r.width / 2 - 44 + 'px', top: r.top - 60 + 'px' } },
        h('div', { class: 'glow', text: '🏮' }),
        h('small', { text }));
    el.addEventListener('animationend', () => el.remove());
    document.body.append(el);
}

// ---------- gift box ----------

const GIFT_TAPS = 5;

function giftCard(site) {
    if (!site.flags.gift) return null;
    const key = `gift.${site.bsYear}`;
    let taps = store.get(key, false) ? GIFT_TAPS : 0;

    const dots = h('div', { class: 'gift-progress', 'aria-hidden': 'true' }, Array.from({ length: GIFT_TAPS }, () => h('i')));
    const box = h('button', { class: 'giftbox', type: 'button', 'aria-label': 'Unwrap the gift' },
        h('div', { class: 'gift-rays' }),
        h('div', { class: 'gift-body' }),
        h('div', { class: 'gift-lid' }, h('span', { class: 'bow', text: '🎀' })));
    const reveal = h('div', { class: 'gift-card', role: 'status', 'aria-live': 'polite', hidden: true, text: site.text['gift.message'] });

    const paint = () => [...dots.children].forEach((d, i) => d.classList.toggle('on', i < taps));

    function open(celebrate) {
        box.classList.add('opened');
        box.setAttribute('aria-label', 'Gift opened');
        reveal.hidden = false;
        if (!celebrate) return;
        burstFrom(box, { n: 20, emojis: ['✨', '💗', '🎉', '⭐'], spread: 170 });
        confetti({ particleCount: 90, spread: 80, origin: { y: 0.6 } });
        vibrate([40, 40, 90]);
        store.set(key, true);
        track('gift');
    }

    box.addEventListener('click', (e) => {
        if (taps >= GIFT_TAPS) {
            burstFrom(box, { n: 6 });
            return;
        }
        taps++;
        paint();
        replay(box, 'shake-gift');
        paperBits(e.clientX, e.clientY, box);
        vibrate(15);
        if (taps === GIFT_TAPS) setTimeout(() => open(true), 250);
    });

    paint();
    if (taps >= GIFT_TAPS) open(false);
    return card(site.text['gift.title'], h('p', { class: 'hint', text: site.text['gift.hint'] }), box, dots, reveal);
}

function paperBits(x, y, box) {
    if (!x && !y) {
        const r = box.getBoundingClientRect();
        x = r.left + r.width / 2;
        y = r.top + r.height / 2;
    }
    for (let i = 0; i < 7; i++) {
        const bit = h('span', { class: 'paper-bit', style: { left: x + 'px', top: y + 'px', background: pick(['#e0736b', '#fff1c9', '#f1b4af', '#cf5f57']) } });
        bit.style.setProperty('--dx', rand(-120, 120) + 'px');
        bit.style.setProperty('--dy', rand(-140, 40) + 'px');
        bit.style.setProperty('--rot', rand(-540, 540) + 'deg');
        bit.addEventListener('animationend', () => bit.remove());
        document.body.append(bit);
    }
}

// ---------- the letter ----------

function letterCard(site) {
    const full = site.text['letter.body'];
    if (!site.flags.letter || !full) return null;
    const key = `letter.${site.bsYear}`;
    const chars = graphemes(full);
    const typed = document.createTextNode('');
    const caret = h('span', { class: 'caret' });
    const body = h('div', { class: 'letter-body', 'aria-hidden': 'true' }, typed);
    const sign = h('p', { class: 'letter-sign', text: site.from ? `— ${site.from} 💗` : '💗' });
    const skip = h('button', { class: 'letter-skip', type: 'button', text: 'Sabai ekai choti padhne ⏩' });
    const el = card(site.text['letter.title'], h('p', { class: 'sr-only', text: full }), body, sign, skip);
    el.classList.add('letter');

    const speed = site.nums['letter.speed'];
    let i = 0;
    let timer = null;
    let finished = false;

    function finish(celebrate) {
        if (finished) return;
        finished = true;
        clearTimeout(timer);
        typed.data = full;
        caret.remove();
        sign.classList.add('show');
        skip.hidden = true;
        if (celebrate) {
            store.set(key, true);
            burstFrom(sign, { n: 8 });
            track('letter');
        }
    }

    function step() {
        if (i >= chars.length) return finish(true);
        const ch = chars[i++];
        typed.appendData(ch);
        const pause = /[.!?]/.test(ch) ? 8 : /[,;:]/.test(ch) ? 4 : ch === '\n' ? 6 : 1;
        timer = setTimeout(step, speed * pause);
    }

    skip.addEventListener('click', () => finish(true));
    if (store.get(key, false)) {
        finish(false);
    } else {
        body.append(caret);
        onVisible(el, step, 0.3);
    }
    return el;
}

// ---------- slideshow ----------

function slideshow(site) {
    const photos = site.photos || [];
    if (!site.flags.slideshow || !photos.length) return null;
    const secs = site.nums['slideshow.seconds'];
    const slides = photos.map((p, i) => {
        const fig = h('figure', { class: 'slide' + (i === 0 ? ' active' : '') },
            h('img', { src: i < 2 ? p.src : null, 'data-src': p.src, alt: p.caption || '' }),
            p.caption && h('figcaption', { text: p.caption }));
        fig.style.setProperty('--dur', secs + 1.5 + 's');
        return fig;
    });
    const pausedTag = h('span', { class: 'paused', text: '⏸ ruko', hidden: true });
    const box = h('div', { class: 'slides', role: 'region', 'aria-roledescription': 'carousel', 'aria-label': site.text['slideshow.title'] }, slides, pausedTag);
    const dots = h('div', { class: 'dots', 'aria-hidden': 'true' }, photos.map((_, i) => h('i', { class: i === 0 ? 'on' : null })));
    let i = 0;
    let paused = false;
    let timer = null;

    const load = (n) => {
        const img = slides[(n + slides.length) % slides.length].querySelector('img');
        if (!img.getAttribute('src')) img.src = img.dataset.src;
    };

    function show(n) {
        slides[i].classList.remove('active');
        dots.children[i].classList.remove('on');
        i = (n + slides.length) % slides.length;
        load(i);
        load(i + 1);
        slides[i].classList.add('active');
        dots.children[i].classList.add('on');
    }

    function schedule() {
        clearInterval(timer);
        if (slides.length > 1) timer = setInterval(() => { if (!paused) show(i + 1); }, secs * 1000);
    }

    let startX = null;
    box.addEventListener('pointerdown', (e) => { startX = e.clientX; });
    box.addEventListener('pointerup', (e) => {
        if (startX != null && Math.abs(e.clientX - startX) > 40) {
            show(e.clientX < startX ? i + 1 : i - 1);
            schedule();
        } else {
            paused = !paused;
            pausedTag.hidden = !paused;
        }
        startX = null;
    });

    onVisible(box, schedule, 0.3);
    const el = card(site.text['slideshow.title'], box, dots);
    return { el, destroy: () => clearInterval(timer) };
}
