// The one guard on leaving the page while an edit is unsaved (FH-66). The
// owner of the unsaved work holds it; the router asks it before any
// navigation (route.ts), and the browser's own prompt covers a reload or a
// closed tab. At most one guard is held: a new one replaces the last.

// Ask is how the guard's owner is asked: it calls proceed once the person
// agrees to drop the work, or never, to stay.
export type Ask = (proceed: () => void) => void;

let held: Ask | null = null;

function onUnload(event: BeforeUnloadEvent) {
  event.preventDefault();
  // Older browsers show their prompt only when returnValue is set.
  event.returnValue = "";
}

// guardLeaving holds the guard until the returned release is called.
export function guardLeaving(ask: Ask): () => void {
  if (!held) window.addEventListener("beforeunload", onUnload);
  held = ask;
  return () => {
    if (held !== ask) return;
    held = null;
    window.removeEventListener("beforeunload", onUnload);
  };
}

// guarded reports whether leaving now would ask first.
export const guarded = () => held !== null;

// leave runs go at once, or asks the guard's owner first.
export function leave(go: () => void) {
  if (held) held(go);
  else go();
}
