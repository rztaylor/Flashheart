import { type RefObject, useEffect } from "react";

// narrowQuery is the phone layout, where the board shows the one column the
// Column picker chose (index.css); it agrees with Tailwind's md breakpoint.
export const narrowQuery = "(width < 48rem)";

// useStartOnWiden scrolls the board's column strip back to its start when the
// window leaves the narrow one-column layout (FH-155). The one column a phone
// showed stays the strip's snap target as it widens, so the browser kept the
// strip scrolled to it and the first column was cut off at the left.
export function useStartOnWiden(scroller: RefObject<HTMLElement | null>) {
  useEffect(() => {
    const media = window.matchMedia?.(narrowQuery);
    if (!media) return;
    const onChange = (event: MediaQueryListEvent) => {
      if (event.matches || !scroller.current) return;
      scroller.current.scrollLeft = 0;
    };
    media.addEventListener("change", onChange);
    return () => media.removeEventListener("change", onChange);
  }, [scroller]);
}
