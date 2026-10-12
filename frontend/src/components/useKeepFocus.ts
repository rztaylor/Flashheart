import { useCallback } from "react";

import { neighbourOf } from "../model/navigation";

// Keeping focus (FH-84, ui-layout.md §8). A view marks its lists and their
// items; when a live reload, a filter or the view's own keyboard move
// re-renders the focused item elsewhere, focus follows it, and when the
// item disappears focus goes to its neighbour: the next item in its list,
// else the previous one (as the list stood before the change), else the
// list's heading. An item whose list went too hands over as its enclosing
// item would. Nothing happens unless focus was inside the container and was
// lost to the page (the body): focus in a dialog, the panel or anywhere else
// stays put.

const ITEM = "data-focus-item";
const LIST = "data-focus-list";
const HEADING = "data-focus-heading";

// focusItem marks an item by a key stable across renders (a ticket or run id).
export const focusItem = (key: string) => ({ [ITEM]: key });
// focusList marks a list of items by a key unique in its container.
export const focusList = (key: string) => ({ [LIST]: key });
// focusHeading marks a list's heading, the last place focus goes; give it
// tabIndex -1 unless it holds a control of its own.
export const focusHeading = { [HEADING]: "" };

const CONTROLS =
  "a[href], button, input, select, textarea, summary, [tabindex]";

const focusable = (element: Element): element is HTMLElement =>
  element instanceof HTMLElement &&
  element.matches(CONTROLS) &&
  !(element as HTMLButtonElement).disabled &&
  !element.closest("[hidden], [inert]");

// controls are an item's focusable elements, the item first when it is one;
// a nested list's heading is a last resort, never a control.
const controls = (item: Element): HTMLElement[] => [
  ...[item].filter(focusable),
  ...[...item.querySelectorAll(CONTROLS)]
    .filter(focusable)
    .filter((element) => !element.hasAttribute(HEADING)),
];

// A part names a control within its item: its tag and its place among the
// item's controls of that tag, so a neighbour's link takes a link's focus.
interface Part {
  tag: string;
  index: number;
}

const partOf = (item: Element, control: Element): Part => ({
  tag: control.tagName,
  index: controls(item)
    .filter((element) => element.tagName === control.tagName)
    .indexOf(control as HTMLElement),
});

const controlAt = (item: Element, part?: Part): HTMLElement | undefined => {
  const all = controls(item);
  if (!part) return all[0];
  return (
    all.filter((element) => element.tagName === part.tag)[part.index] ?? all[0]
  );
};

// Level is one item around the focused control, innermost first: its key,
// its list and the list's order when last seen.
interface Level {
  key: string;
  list: string;
  order: string[];
}

interface Tracked {
  element: HTMLElement;
  part: Part;
  levels: Level[];
}

const keyOf = (item: Element) => item.getAttribute(ITEM) ?? "";

function listOf(container: HTMLElement, item: Element): Element | null {
  const list = item.parentElement?.closest(`[${LIST}]`) ?? null;
  return list && container.contains(list) ? list : null;
}

function itemsOf(container: HTMLElement, list: Element): Element[] {
  return [...list.querySelectorAll(`[${ITEM}]`)].filter(
    (item) => listOf(container, item) === list,
  );
}

function findList(container: HTMLElement, key: string): Element | undefined {
  if (!key) return undefined;
  return [container, ...container.querySelectorAll(`[${LIST}]`)].find(
    (list) => list.getAttribute(LIST) === key,
  );
}

// findItem finds an item by key, in its old list first: the same ticket may
// be listed twice (the Overview's At risk and Up next).
function findItem(container: HTMLElement, level: Level): Element | undefined {
  const all = [...container.querySelectorAll(`[${ITEM}]`)].filter(
    (item) => keyOf(item) === level.key,
  );
  return (
    all.find(
      (item) => listOf(container, item)?.getAttribute(LIST) === level.list,
    ) ?? all[0]
  );
}

function track(container: HTMLElement, control: Element): Tracked | null {
  if (!(control instanceof HTMLElement)) return null;
  const inner = control.closest(`[${ITEM}]`);
  if (!inner || !container.contains(inner)) return null;
  const levels: Level[] = [];
  let item: Element | null = inner;
  while (item && container.contains(item)) {
    const list = listOf(container, item);
    levels.push({
      key: keyOf(item),
      list: list?.getAttribute(LIST) ?? "",
      order: list ? itemsOf(container, list).map(keyOf) : [],
    });
    item = item.parentElement?.closest(`[${ITEM}]`) ?? null;
  }
  return { element: control, part: partOf(inner, control), levels };
}

// moveFocus focuses an element without scrolling the page to it, then
// brings it into view only as far as needed.
function moveFocus(element: HTMLElement | undefined): boolean {
  if (!element) return false;
  element.focus({ preventScroll: true });
  if (document.activeElement !== element) return false;
  element.scrollIntoView?.({ block: "nearest", inline: "nearest" });
  return true;
}

// restore puts focus back where tracked says it was, or on its neighbour.
function restore(container: HTMLElement, tracked: Tracked): boolean {
  for (const [depth, level] of tracked.levels.entries()) {
    const part = depth === 0 ? tracked.part : undefined;
    const item = findItem(container, level);
    if (item && moveFocus(controlAt(item, part))) return true;
    const list = findList(container, level.list);
    if (!list) continue;
    const remaining = new Map(
      itemsOf(container, list).map((element) => [keyOf(element), element]),
    );
    const neighbour = neighbourOf(level.order, level.key, (key) => {
      const element = remaining.get(key);
      return element !== undefined && controls(element).length > 0;
    });
    const next = neighbour ? remaining.get(neighbour) : undefined;
    if (next && moveFocus(controlAt(next, part))) return true;
    const heading = [...list.querySelectorAll(`[${HEADING}]`)].find(
      (element) => element.closest(`[${LIST}]`) === list,
    );
    if (heading && moveFocus(controls(heading)[0])) return true;
  }
  return false;
}

const lostToPage = () =>
  document.activeElement === null || document.activeElement === document.body;

// keepFocus watches container until the returned function stops it.
export function keepFocus(container: HTMLElement): () => void {
  let tracked: Tracked | null = null;
  let timer = 0;

  const onFocusIn = (event: FocusEvent) => {
    if (event.target instanceof Element)
      tracked = track(container, event.target);
  };
  const onFocusOut = (event: FocusEvent) => {
    const next = event.relatedTarget;
    if (next instanceof Node && !container.contains(next)) {
      tracked = null;
      return;
    }
    if (next) return;
    // Focus left for nowhere: a click on the page lets go of it, but a
    // removed item, an item moved in the DOM or the window losing focus
    // does not. Settle once the change is done.
    const target = event.target;
    window.clearTimeout(timer);
    timer = window.setTimeout(() => {
      if (
        tracked?.element === target &&
        tracked.element.isConnected &&
        lostToPage()
      )
        tracked = null;
    }, 0);
  };
  const observer = new MutationObserver((records) => {
    if (!tracked) return;
    const active = document.activeElement;
    if (!lostToPage()) {
      // Focus is still on its control, or moved on (focusin tracked it):
      // keep the lists' order current for a later change.
      if (active && container.contains(active))
        tracked = track(container, active);
      return;
    }
    const element = tracked.element;
    const removed =
      !element.isConnected ||
      records.some((record) =>
        [...record.removedNodes].some((node) => node.contains(element)),
      );
    if (!removed) return;
    // Focus that lands tracks its new place (focusin); none lets go.
    const lost = tracked;
    tracked = null;
    restore(container, lost);
  });

  container.addEventListener("focusin", onFocusIn);
  container.addEventListener("focusout", onFocusOut);
  observer.observe(container, { childList: true, subtree: true });
  if (document.activeElement && container.contains(document.activeElement))
    tracked = track(container, document.activeElement);
  return () => {
    window.clearTimeout(timer);
    observer.disconnect();
    container.removeEventListener("focusin", onFocusIn);
    container.removeEventListener("focusout", onFocusOut);
  };
}

// useKeepFocus returns a ref for the element holding the lists whose focus
// it keeps (see keepFocus).
export function useKeepFocus() {
  return useCallback((container: HTMLElement | null) => {
    if (container) return keepFocus(container);
  }, []);
}
