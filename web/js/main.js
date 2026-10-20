// Boots the site: loads data from the server, renders the page for the
// current state (pre / during / post) and switches pages live at midnight.

import { h, revealOnScroll } from './util.js';
import { loadSite, now, track } from './api.js';
import { initEffects, confetti } from './effects.js';
import { initBear } from './bear.js';
import { initSpam } from './spam.js';
import { unlockAudioOnGesture } from './audio.js';
import { maybeShowEnvelope } from './envelope.js';
import { renderPre } from './states/pre.js';
import { renderDuring } from './states/during.js';
import { renderPost } from './states/post.js';

const app = document.getElementById('app');
const renderers = { pre: renderPre, during: renderDuring, post: renderPost };
const MODE_LABEL = { preview: '👀 Preview', 'forced-me': '👀 Forced on this device', forced: '⚠️ Forced for everyone' };
const STATE_LABEL = { pre: 'countdown', during: 'birthday', post: 'teasing week' };

let site;
let view;
let refreshing = false;
let lastFetch = 0;
let entering = 0;

async function boot() {
    try {
        site = await loadSite();
    } catch {
        app.replaceChildren(h('div', { class: 'loading' },
            h('img', { src: '/assets/pre/1.gif', alt: '', width: 120, height: 120 }),
            h('p', { text: 'Server sanga kura bhayena 🥺 Ek chin pachhi reload gara.' })));
        return;
    }
    lastFetch = Date.now();
    initEffects(site);
    unlockAudioOnGesture();
    track('visit');
    render();
    tick();
    enter();
}

// enter shows today's envelope, then starts the page's opening moment.
async function enter() {
    const token = ++entering;
    await maybeShowEnvelope(site);
    if (token === entering) view.start?.();
}

function render() {
    view?.destroy?.();
    initBear(site);
    initSpam(site);
    document.title = `${site.name} 💗`;
    document.body.dataset.state = site.state;
    app.replaceChildren();
    view = renderers[site.state](app, site);
    if (site.admin && site.mode) {
        app.append(h('div', { class: 'preview-badge', text: `${MODE_LABEL[site.mode]} · ${STATE_LABEL[site.state]}` }));
    }
    revealOnScroll(app);
    view.tick?.(now());
}

const boundary = () => (site.state === 'pre' ? site.start : site.state === 'during' ? site.end : site.postEnd);

function tick() {
    const t = now();
    view?.tick?.(t);
    if (t >= boundary() && !refreshing && Date.now() - lastFetch > 4000) refresh();
    setTimeout(tick, 1000 - (t % 1000) + 10);
}

// refresh asks the server what's next once a countdown reaches zero.
async function refresh() {
    refreshing = true;
    const before = site.state;
    try {
        // When previewing a time, carry the preview clock forward.
        site = await loadSite(site.mode === 'preview' ? { at: Math.floor(now()) } : {});
        if (site.state !== before) {
            scrollTo({ top: 0 });
            render();
            if (site.state === 'during') confetti({ particleCount: 160, spread: 120 });
            enter();
        }
    } catch {
        // Try again after the cool-down.
    } finally {
        lastFetch = Date.now();
        refreshing = false;
    }
}

boot();
