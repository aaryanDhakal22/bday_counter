// The birthday cake. Candles go out when she taps them, or when she blows
// into the mic (it listens for loud, steady noise).

import { h, pick } from './util.js';
import { audioCtx } from './audio.js';
import { track } from './api.js';
import { burst, vibrate } from './effects.js';

const STRIPES = ['#e0736b', '#7cc6a4', '#f0b44c', '#9b87e0', '#5fa8e0', '#f08fb0'];

export const micSupported = () => !!(window.isSecureContext && navigator.mediaDevices?.getUserMedia);

export function createCake(site, { onAllOut }) {
    const n = Math.round(site.nums['cake.candles']) || 5;
    const candles = Array.from({ length: n }, (_, i) => {
        const c = h('button', { class: 'candle', type: 'button', 'aria-label': `Candle ${i + 1}` }, h('span', { class: 'flame' }));
        c.style.setProperty('--stripe', STRIPES[i % STRIPES.length]);
        c.addEventListener('click', () => blowOut(c, 'tap'));
        return c;
    });
    const row = h('div', { class: 'candles' }, candles);
    if (n > 6) {
        row.style.gap = '.25rem';
        candles.forEach((c) => { c.style.width = '.8rem'; });
    }
    const el = h('div', { class: 'cake' },
        h('div', { class: 'cake-plate' }),
        h('div', { class: 'cake-tier t1' }, h('div', { class: 'icing' })),
        h('div', { class: 'cake-tier t2' }, h('div', { class: 'icing' })),
        row);

    let stream = null;
    let raf = 0;

    const lit = () => candles.filter((c) => !c.classList.contains('out'));

    function blowOut(c, how) {
        if (c.classList.contains('out')) return;
        c.classList.add('out');
        c.setAttribute('aria-label', 'Candle out');
        track(how === 'mic' ? 'cake_mic' : 'cake_tap');
        vibrate(18);
        const r = c.getBoundingClientRect();
        burst(r.left + r.width / 2, r.top - 20, { n: 2, emojis: ['💨'], spread: 40 });
        if (!lit().length) {
            stopMic();
            setTimeout(() => onAllOut(how), 450);
        }
    }

    // startMic asks for the mic and starts listening. Throws if refused.
    async function startMic(meter) {
        const ctx = audioCtx();
        if (!ctx) throw new Error('no audio');
        stream = await navigator.mediaDevices.getUserMedia({
            audio: { echoCancellation: false, noiseSuppression: false, autoGainControl: false },
        });
        const analyser = ctx.createAnalyser();
        analyser.fftSize = 1024;
        ctx.createMediaStreamSource(stream).connect(analyser);
        const buf = new Float32Array(analyser.fftSize);
        // Sensitivity 1–10 → loudness needed (RMS). 6 ≈ a proper blow from a few cm away.
        const threshold = 0.3 - site.nums['mic.sensitivity'] * 0.026;
        const need = site.nums['mic.blowMs'];
        let blowing = 0;
        let last = performance.now();

        const loop = (t) => {
            analyser.getFloatTimeDomainData(buf);
            let sum = 0;
            for (let i = 0; i < buf.length; i++) sum += buf[i] * buf[i];
            const rms = Math.sqrt(sum / buf.length);
            const dt = t - last;
            last = t;
            const wind = Math.min(1, rms / threshold);
            el.style.setProperty('--wind', rms > threshold * 0.35 ? wind.toFixed(2) : '0');
            if (meter) meter.style.width = (wind * 100).toFixed(0) + '%';
            if (rms > threshold) {
                blowing += dt;
                if (blowing >= need) {
                    blowing = 0;
                    const left = lit();
                    if (left.length) blowOut(pick(left), 'mic');
                }
            } else {
                blowing = Math.max(0, blowing - dt);
            }
            if (stream) raf = requestAnimationFrame(loop);
        };
        raf = requestAnimationFrame(loop);
    }

    function stopMic() {
        cancelAnimationFrame(raf);
        stream?.getTracks().forEach((t) => t.stop());
        stream = null;
        el.style.setProperty('--wind', '0');
    }

    function relight() {
        candles.forEach((c, i) => {
            c.classList.remove('out');
            c.setAttribute('aria-label', `Candle ${i + 1}`);
        });
    }

    function putOut() {
        candles.forEach((c) => c.classList.add('out'));
    }

    return { el, startMic, stopMic, relight, putOut };
}
