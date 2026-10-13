import {
  type RefObject,
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";

// sectionElementId is the element id of a full-page section (CARD-7).
export const sectionElementId = (id: string) => `section-${id}`;

// Input that means the person is moving the page themselves, which hands
// the outline back to the scroll after a section was chosen.
const moving = ["wheel", "touchmove", "keydown", "pointerdown"] as const;

// useSectionInView follows which of the page's sections is in view, for the
// outline (FH-65): the first section, in page order, crossing the top third
// of the window. Between sections the last one stays current. Each section
// is an element under container marked data-section with its id. choose
// makes a section current and keeps it so while the page scrolls to it,
// until the person scrolls, types or clicks: a short section near the end
// may never reach the top third.
export function useSectionInView(
  container: RefObject<HTMLElement | null>,
  ids: string[],
): [string, (id: string) => void] {
  const [current, setCurrent] = useState(ids[0] ?? "");
  const chosen = useRef("");
  const order = ids.join(" ");
  useEffect(() => {
    const root = container.current;
    if (!root || typeof IntersectionObserver === "undefined") return;
    const sections = order.split(" ");
    const visible = new Map<string, boolean>();
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          const id = (entry.target as HTMLElement).dataset.section;
          if (id) visible.set(id, entry.isIntersecting);
        }
        const first = sections.find((id) => visible.get(id));
        if (first && !chosen.current) setCurrent(first);
      },
      { rootMargin: "0px 0px -66% 0px" },
    );
    for (const element of root.querySelectorAll<HTMLElement>("[data-section]"))
      observer.observe(element);
    const release = () => {
      chosen.current = "";
    };
    for (const kind of moving)
      window.addEventListener(kind, release, { capture: true, passive: true });
    return () => {
      observer.disconnect();
      for (const kind of moving)
        window.removeEventListener(kind, release, { capture: true });
    };
  }, [container, order]);
  const choose = useCallback((id: string) => {
    chosen.current = id;
    setCurrent(id);
  }, []);
  return [ids.includes(current) ? current : (ids[0] ?? ""), choose];
}

// focusSection brings a section to the top of the page and moves keyboard
// focus to its heading, without a second scroll.
export function focusSection(id: string) {
  const section = document.getElementById(sectionElementId(id));
  if (!section) return;
  const target =
    section.querySelector<HTMLElement>("h2, h3") ?? (section as HTMLElement);
  if (!target.hasAttribute("tabindex")) target.setAttribute("tabindex", "-1");
  const still = window.matchMedia?.("(prefers-reduced-motion: reduce)").matches;
  section.scrollIntoView?.({
    block: "start",
    behavior: still ? "auto" : "smooth",
  });
  target.focus({ preventScroll: true });
}
