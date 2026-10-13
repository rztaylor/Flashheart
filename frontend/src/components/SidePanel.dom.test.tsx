import { fireEvent, render } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SidePanel } from "./SidePanel";

describe("SidePanel", () => {
  it("closes on Escape and claims the key, so a prompt the close opens stays open (FH-128)", () => {
    const onClose = vi.fn();
    render(
      <SidePanel label="Ticket FH-1" onClose={onClose}>
        <textarea aria-label="Add a comment" />
      </SidePanel>,
    );
    const handled = fireEvent.keyDown(document.activeElement ?? document, {
      key: "Escape",
    });
    expect(onClose).toHaveBeenCalledOnce();
    // fireEvent returns false when the event's default was prevented.
    expect(handled).toBe(false);
  });

  it("leaves an Escape something inside already handled", () => {
    const onClose = vi.fn();
    const { getByRole } = render(
      <SidePanel label="Ticket FH-1" onClose={onClose}>
        <input
          aria-label="Field"
          onKeyDown={(event) => {
            if (event.key === "Escape") event.preventDefault();
          }}
        />
      </SidePanel>,
    );
    fireEvent.keyDown(getByRole("textbox"), { key: "Escape" });
    expect(onClose).not.toHaveBeenCalled();
  });
});
