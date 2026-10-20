// Before her birthday: the countdown, the bear and the toys.

import { h, pick } from '../util.js';
import { createCountdown } from '../countdown.js';
import { createBear } from '../bear.js';
import { toys } from '../toys.js';
import { bouncyTitle, datePill, footer } from '../ui.js';

export function renderPre(app, site) {
    const countdown = createCountdown();
    const bear = createBear({ state: 'pre' });
    const playthings = toys(site);

    app.append(h('div', { class: 'page' },
        h('section', { class: 'hero' },
            datePill(site),
            bouncyTitle(site.text['pre.title']),
            countdown.el,
            bear.el,
            h('p', { class: 'sub', text: site.text['pre.sub'] }),
            playthings && h('a', { class: 'scroll-hint', href: '#toys', text: site.text['pre.scroll'] })),
        playthings,
        footer(site)));

    return {
        tick(now) {
            countdown.set(site.start - now);
        },
        start() {
            setTimeout(() => bear.say(pick(site.lists['bear.lines'] || [])), 700);
        },
        destroy() {
            bear.destroy();
        },
    };
}
