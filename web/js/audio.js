// One shared AudioContext, unlocked by her first tap (browsers block sound
// until then). The mic and the music box both use it.

let ctx = null;

export function audioCtx() {
    if (!ctx) {
        const AC = window.AudioContext || window.webkitAudioContext;
        if (!AC) return null;
        ctx = new AC();
    }
    if (ctx.state === 'suspended') ctx.resume().catch(() => {});
    return ctx;
}

export function unlockAudioOnGesture() {
    const unlock = () => audioCtx();
    for (const type of ['pointerdown', 'click', 'touchend', 'keydown']) {
        addEventListener(type, unlock, { capture: true, passive: true });
    }
}
