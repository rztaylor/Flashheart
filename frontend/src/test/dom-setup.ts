import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";

// Testing Library unmounts after each test only when the runner exposes
// globals, which Vitest does not; every interaction test shares this.
afterEach(() => {
  cleanup();
});

// Browsers queue a task to fire selectionchange
// (https://w3c.github.io/selection-api/#selectionchange-event); happy-dom
// fires it synchronously inside Selection.collapse, which re-enters
// CodeMirror's view update while it places the cursor. Defer it as a
// browser does. happy-dom calls dispatchEvent again on the document's own
// phase, so a deferred event passes straight through. The real setTimeout
// is kept so fake timers cannot hold it.
const dispatch = document.dispatchEvent.bind(document);
const queueTask = globalThis.setTimeout;
const deferred = new WeakSet<Event>();
document.dispatchEvent = (event: Event): boolean => {
  if (event.type !== "selectionchange" || deferred.has(event)) {
    return dispatch(event);
  }
  deferred.add(event);
  queueTask(() => dispatch(event));
  return true;
};
