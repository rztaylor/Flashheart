import { renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { useNoFileNavigation } from "./fileDrops";

// A file dropped where nothing takes it must not open in the tab, leaving
// the board (FH-18); a drop a target took, and drags of text, are left.
describe("useNoFileNavigation", () => {
  const drag = (type: string, types: string[], taken = false) => {
    const event = new Event(type, { bubbles: true, cancelable: true });
    Object.defineProperty(event, "dataTransfer", {
      value: { types, dropEffect: "copy" },
    });
    if (taken) event.preventDefault();
    window.dispatchEvent(event);
    return event as Event & { dataTransfer: { dropEffect: string } };
  };

  it("stops the browser opening a file dropped outside a drop target", () => {
    const { unmount } = renderHook(() => useNoFileNavigation());
    const over = drag("dragover", ["Files"]);
    expect(over.defaultPrevented).toBe(true);
    expect(over.dataTransfer.dropEffect).toBe("none");
    expect(drag("drop", ["Files"]).defaultPrevented).toBe(true);

    // A target that took the drag keeps its drop effect.
    expect(drag("dragover", ["Files"], true).dataTransfer.dropEffect).toBe(
      "copy",
    );
    // Text and cards are not files.
    expect(drag("drop", ["text/plain"]).defaultPrevented).toBe(false);

    unmount();
    expect(drag("drop", ["Files"]).defaultPrevented).toBe(false);
  });
});
