// Small DOM + storage helpers shared by every module.

export const params = new URLSearchParams(location.search);

export const reducedMotion = matchMedia('(prefers-reduced-motion: reduce)').matches;

// h('div', { class: 'card', onclick: fn }, child, 'text', ...)
export function h(tag, props, ...children) {
    const el = document.createElement(tag);
    for (const [k, v] of Object.entries(props || {})) {
        if (v == null || v === false) continue;
        if (k === 'class') el.className = v;
        else if (k === 'text') el.textContent = v;
        else if (k === 'style' && typeof v === 'object') Object.assign(el.style, v);
        else if (k.startsWith('on') && typeof v === 'function') el.addEventListener(k.slice(2), v);
        else el.setAttribute(k, v === true ? '' : v);
    }
    for (const c of children.flat(Infinity)) {
        if (c == null || c === false) continue;
        el.append(c instanceof Node ? c : document.createTextNode(String(c)));
    }
    return el;
}

// Same as h() but for SVG elements.
export function svg(tag, attrs, ...children) {
    const el = document.createElementNS('http://www.w3.org/2000/svg', tag);
    for (const [k, v] of Object.entries(attrs || {})) {
        if (v == null) continue;
        if (k.startsWith('on') && typeof v === 'function') el.addEventListener(k.slice(2), v);
        else el.setAttribute(k, v);
    }
    for (const c of children.flat()) if (c) el.append(c);
    return el;
}

export const pick = (arr) => arr[Math.floor(Math.random() * arr.length)];
export const rand = (a, b) => a + Math.random() * (b - a);
export const clamp = (x, a, b) => Math.min(b, Math.max(a, x));
export const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

export function shuffle(arr) {
    const a = arr.slice();
    for (let i = a.length - 1; i > 0; i--) {
        const j = Math.floor(Math.random() * (i + 1));
        [a[i], a[j]] = [a[j], a[i]];
    }
    return a;
}

// pickNew avoids saying the same thing twice in a row.
export function pickNew(arr, last) {
    if (!arr || !arr.length) return '';
    if (arr.length === 1) return arr[0];
    let v;
    do v = pick(arr); while (v === last);
    return v;
}

// localStorage that never throws (private mode, blocked storage, previews).
export const store = {
    get(key, fallback) {
        try {
            const v = localStorage.getItem('bday.' + key);
            return v == null ? fallback : JSON.parse(v);
        } catch {
            return fallback;
        }
    },
    set(key, value) {
        try {
            localStorage.setItem('bday.' + key, JSON.stringify(value));
        } catch { /* storage unavailable: the page still works, it just forgets */ }
    },
};

// Restart a CSS animation class on an element.
export function replay(el, cls) {
    if (!el) return;
    el.classList.remove(cls);
    void el.offsetWidth;
    el.classList.add(cls);
}

// Run fn once when el scrolls into view.
export function onVisible(el, fn, threshold = 0.35) {
    if (!('IntersectionObserver' in window)) return void fn();
    const io = new IntersectionObserver((entries) => {
        if (entries.some((e) => e.isIntersecting)) {
            io.disconnect();
            fn();
        }
    }, { threshold });
    io.observe(el);
}

// Fade sections in as they scroll into view.
export function revealOnScroll(root) {
    root.querySelectorAll('.reveal:not(.in)').forEach((el) => onVisible(el, () => el.classList.add('in'), 0.12));
}

export function center(el) {
    const r = el.getBoundingClientRect();
    return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
}
