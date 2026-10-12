import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { Dialog } from "./Dialog";

// Opener renders a button that opens the dialog, inside a panel whose own
// Escape handler stands for the card panel underneath.
function Opener({
  onClose = () => {},
  onPanelEscape = () => {},
}: {
  onClose?: () => void;
  onPanelEscape?: () => void;
}) {
  const [open, setOpen] = useState(false);
  return (
    // biome-ignore lint/a11y/noStaticElementInteractions: stands for the card panel's Escape handler.
    <div
      onKeyDown={(event) => {
        if (event.key === "Escape") onPanelEscape();
      }}
    >
      <button type="button" onClick={() => setOpen(true)}>
        Move
      </button>
      {open ? (
        <Dialog
          title="Blocked move"
          onClose={() => {
            onClose();
            setOpen(false);
          }}
          actions={<button type="button">Move anyway</button>}
        >
          <p>FH-3 waits on FH-2.</p>
        </Dialog>
      ) : null}
    </div>
  );
}

// The decision dialog: modal, named by its title, closed by Escape or its
// Close button without closing the panel underneath, and focus returns to
// whatever opened it.
describe("Dialog", () => {
  it("opens modal, named by its title, with its actions", async () => {
    const user = userEvent.setup();
    render(<Opener />);
    await user.click(screen.getByRole("button", { name: "Move" }));
    const dialog = screen.getByRole("dialog", { name: "Blocked move" });
    expect(dialog.hasAttribute("open")).toBe(true);
    expect(screen.getByText("FH-3 waits on FH-2.")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Move anyway" })).toBeTruthy();
  });

  it("closes on Escape without reaching the panel underneath, and returns focus", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    const onPanelEscape = vi.fn();
    render(<Opener onClose={onClose} onPanelEscape={onPanelEscape} />);
    const opener = screen.getByRole("button", { name: "Move" });
    await user.click(opener);
    screen.getByRole("button", { name: "Move anyway" }).focus();
    await user.keyboard("{Escape}");
    expect(onClose).toHaveBeenCalledOnce();
    expect(onPanelEscape).not.toHaveBeenCalled();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(document.activeElement).toBe(opener);
  });

  it("closes through the browser's cancel, keeping the dialog under React's control", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(<Opener onClose={onClose} />);
    await user.click(screen.getByRole("button", { name: "Move" }));
    const dialog = screen.getByRole("dialog", { name: "Blocked move" });
    const cancel = new Event("cancel", { cancelable: true });
    fireEvent(dialog, cancel);
    expect(cancel.defaultPrevented).toBe(true);
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("closes from its Close button and returns focus to the opener", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(<Opener onClose={onClose} />);
    const opener = screen.getByRole("button", { name: "Move" });
    await user.click(opener);
    await user.click(screen.getByRole("button", { name: "Close" }));
    expect(onClose).toHaveBeenCalledOnce();
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(document.activeElement).toBe(opener);
  });
});
