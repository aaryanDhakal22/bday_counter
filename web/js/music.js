// A music-box "Happy Birthday", synthesized in the browser (no audio file).

import { audioCtx } from './audio.js';

const F = {
    C3: 130.81, D3: 146.83, G3: 196.0,
    C4: 261.63, D4: 293.66, G4: 392.0,
    D5: 587.33, E5: 659.25, Fs5: 739.99, G5: 783.99, A5: 880.0, B5: 987.77, C6: 1046.5, D6: 1174.66,
};

// [note, beats] in 3/4, starting with the "Hap-py" pickup.
const MELODY = [
    ['D5', 0.75], ['D5', 0.25], ['E5', 1], ['D5', 1], ['G5', 1], ['Fs5', 2],
    ['D5', 0.75], ['D5', 0.25], ['E5', 1], ['D5', 1], ['A5', 1], ['G5', 2],
    ['D5', 0.75], ['D5', 0.25], ['D6', 1], ['B5', 1], ['G5', 1], ['Fs5', 1], ['E5', 1],
    ['C6', 0.75], ['C6', 0.25], ['B5', 1], ['G5', 1], ['A5', 1], ['G5', 3],
];

// Root of each bar after the pickup, played oom-pah-pah.
const BASS = [['G3', 'G4'], ['D3', 'D4'], ['D3', 'D4'], ['G3', 'G4'], ['G3', 'G4'], ['C3', 'C4'], ['D3', 'D4'], ['G3', 'G4']];

const BEAT = 0.44;

let playing = null;

function pluck(ctx, out, freq, t, gain, decay) {
    const env = ctx.createGain();
    env.gain.setValueAtTime(0.0001, t);
    env.gain.exponentialRampToValueAtTime(gain, t + 0.005);
    env.gain.exponentialRampToValueAtTime(0.0001, t + decay);
    env.connect(out);
    // A few partials; the slightly-off 4.2× one gives the metal-tine shimmer.
    for (const [mult, amp] of [[1, 1], [2, 0.32], [3, 0.1], [4.2, 0.07]]) {
        const o = ctx.createOscillator();
        o.type = 'sine';
        o.frequency.value = freq * mult;
        const g = ctx.createGain();
        g.gain.setValueAtTime(amp, t);
        if (mult > 1) g.gain.exponentialRampToValueAtTime(0.0001, t + decay / (mult * 1.4));
        o.connect(g).connect(env);
        o.start(t);
        o.stop(t + decay + 0.05);
    }
}

// playBirthdaySong returns false if audio isn't available.
export function playBirthdaySong(onEnd) {
    const ctx = audioCtx();
    if (!ctx) return false;
    stopSong();

    const master = ctx.createGain();
    master.gain.value = 0.5;
    master.connect(ctx.destination);
    // Soft echo so it sounds like a little room.
    const delay = ctx.createDelay();
    delay.delayTime.value = 0.27;
    const feedback = ctx.createGain();
    feedback.gain.value = 0.28;
    const tone = ctx.createBiquadFilter();
    tone.type = 'lowpass';
    tone.frequency.value = 2800;
    const wet = ctx.createGain();
    wet.gain.value = 0.25;
    master.connect(delay);
    delay.connect(tone).connect(feedback).connect(delay);
    tone.connect(wet).connect(ctx.destination);

    const t0 = ctx.currentTime + 0.12;
    let t = t0;
    for (const [note, beats] of MELODY) {
        pluck(ctx, master, F[note], t, 0.22, 1.9);
        t += beats * BEAT;
    }
    BASS.forEach(([low, high], bar) => {
        const start = t0 + (1 + bar * 3) * BEAT;
        pluck(ctx, master, F[low], start, 0.13, 1.6);
        pluck(ctx, master, F[high], start + BEAT, 0.05, 0.8);
        pluck(ctx, master, F[high], start + 2 * BEAT, 0.05, 0.8);
    });

    const total = (t - t0 + 2.2) * 1000;
    const p = {
        master,
        wet,
        timer: setTimeout(() => {
            if (playing === p) playing = null;
            onEnd?.();
        }, total),
    };
    playing = p;
    return true;
}

export function stopSong() {
    if (!playing) return;
    const ctx = audioCtx();
    const now = ctx.currentTime;
    playing.master.gain.setTargetAtTime(0, now, 0.05);
    playing.wet.gain.setTargetAtTime(0, now, 0.2);
    clearTimeout(playing.timer);
    playing = null;
}

export const isSongPlaying = () => playing !== null;
