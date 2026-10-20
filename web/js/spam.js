// Spam guard: tap a toy too many times too fast and the bear gets grumpy.
// Thresholds come from /config → Settings → Spam guard.

import { grumpy } from './bear.js';
import { track } from './api.js';
import { replay } from './util.js';

let site;
const hits = {};
const lockedUntil = {};

export function initSpam(s) {
    site = s;
}

// guard returns false (and shakes el) when the feature is locked or just got spammed.
export function guard(feature, el) {
    if (!site?.flags.spamGuard) return true;
    const max = site.nums[`spam.${feature}.max`];
    const win = site.nums[`spam.${feature}.window`] * 1000;
    const lock = site.nums[`spam.${feature}.lock`] * 1000;
    if (!max) return true;

    const t = performance.now();
    if ((lockedUntil[feature] || 0) > t) {
        replay(el, 'shake');
        return false;
    }
    const recent = (hits[feature] || []).filter((x) => t - x < win);
    recent.push(t);
    hits[feature] = recent;
    if (recent.length <= max) return true;

    hits[feature] = [];
    lockedUntil[feature] = t + lock;
    replay(el, 'shake');
    el?.classList.add('is-locked');
    setTimeout(() => el?.classList.remove('is-locked'), lock);
    grumpy(lock);
    track('grumpy');
    return false;
}
