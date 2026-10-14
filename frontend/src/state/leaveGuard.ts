// The one guard on leaving the page while an edit is unsaved (FH-66). The
// owner of the unsaved work holds it; the router asks it before any
// navigation (route.ts), and the browser's own prompt covers a reload or a
// closed tab. At most one guard is held: a new one replaces the last.
//
// A save still waiting for its answer is not unsaved work (FH-156, D70):
// leaving the page does not ask about it, and its owner says so if it then
// fails. Closing or reloading the tab would lose that answer, and the work
// with it, so the browser's own prompt stays while a save is in flight
// (guardUnloading).

// Ask is how the guard's owner is asked: it calls proceed once the person
// agrees to drop the work, or never, to stay.
export type Ask = (proceed: () => void) => void;

let held: Ask | null = null;
// saving counts the saves in flight.
let saving = 0;
let listening = false;

function onUnload(event: BeforeUnloadEvent) {
  event.preventDefault();
  // Older browsers show their prompt only when returnValue is set.
  event.returnValue = "";
}

// sync listens for a reload or closed tab while anything holds the prompt.
function sync() {
  const want = held !== null || saving > 0;
  if (want === listening) return;
  listening = want;
  if (want) window.addEventListener("beforeunload", onUnload);
  else window.removeEventListener("beforeunload", onUnload);
}

// guardLeaving holds the guard until the returned release is called.
export function guardLeaving(ask: Ask): () => void {
  held = ask;
  sync();
  return () => {
    if (held !== ask) return;
    held = null;
    sync();
  };
}

// guardUnloading holds the browser's prompt on a reload or a closed tab,
// and only that, until the returned release is called: a save in flight.
// Releasing twice releases once.
export function guardUnloading(): () => void {
  saving += 1;
  sync();
  let released = false;
  return () => {
    if (released) return;
    released = true;
    saving -= 1;
    sync();
  };
}

// guarded reports whether leaving now would ask first.
export const guarded = () => held !== null;

// leave runs go at once, or asks the guard's owner first.
export function leave(go: () => void) {
  if (held) held(go);
  else go();
}
