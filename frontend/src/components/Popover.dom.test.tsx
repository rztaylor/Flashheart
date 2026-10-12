import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { Popover } from "./Popover";

function setup() {
  const user = userEvent.setup();
  render(
    <>
      <Popover label="Type" button="Type">
        {(close) => (
          <>
            <button type="button">Bug</button>
            <button type="button" onClick={close}>
              Done
            </button>
          </>
        )}
      </Popover>
      <button type="button">Outside</button>
      <p>Elsewhere</p>
    </>,
  );
  const trigger = screen.getByRole("button", { name: "Type" });
  return { user, trigger };
}

// The disclosure of FH-39: Escape, a click outside and tabbing away close
// it; Escape and a finishing choice return focus to its button.
describe("Popover", () => {
  it("opens its panel from the button, tied to it", async () => {
    const { user, trigger } = setup();
    expect(trigger.getAttribute("aria-expanded")).toBe("false");
    expect(screen.queryByRole("button", { name: "Bug" })).toBeNull();

    await user.click(trigger);
    expect(trigger.getAttribute("aria-expanded")).toBe("true");
    const choice = screen.getByRole("button", { name: "Bug" });
    expect(trigger.getAttribute("aria-controls")).toBe(
      choice.parentElement?.id,
    );

    await user.click(trigger);
    expect(trigger.getAttribute("aria-expanded")).toBe("false");
  });

  it("closes on Escape from inside, returns focus to its button and claims the key", async () => {
    const { user, trigger } = setup();
    const underneath = vi.fn();
    const listener = (event: KeyboardEvent) =>
      underneath(event.defaultPrevented);
    document.addEventListener("keydown", listener);
    try {
      await user.click(trigger);
      await user.tab();
      expect(document.activeElement).toBe(
        screen.getByRole("button", { name: "Bug" }),
      );
      await user.keyboard("{Escape}");
      expect(screen.queryByRole("button", { name: "Bug" })).toBeNull();
      expect(document.activeElement).toBe(trigger);
      // The card panel underneath sees the key already handled.
      expect(underneath).toHaveBeenCalledWith(true);
    } finally {
      document.removeEventListener("keydown", listener);
    }
  });

  it("leaves an Escape another control already claimed", async () => {
    const { user, trigger } = setup();
    // A dialog on top claims Escape before the popover's document listener.
    const claim = (event: KeyboardEvent) => event.preventDefault();
    window.addEventListener("keydown", claim, { capture: true });
    try {
      await user.click(trigger);
      await user.keyboard("{Escape}");
      expect(trigger.getAttribute("aria-expanded")).toBe("true");
    } finally {
      window.removeEventListener("keydown", claim, { capture: true });
    }
  });

  it("closes on a click outside, even on something that takes no focus", async () => {
    const { user, trigger } = setup();
    await user.click(trigger);
    await user.click(screen.getByText("Elsewhere"));
    expect(trigger.getAttribute("aria-expanded")).toBe("false");
  });

  it("closes when focus tabs away, and stays open while it moves inside", async () => {
    const { user, trigger } = setup();
    await user.click(trigger);
    await user.tab();
    await user.tab();
    expect(document.activeElement).toBe(
      screen.getByRole("button", { name: "Done" }),
    );
    expect(trigger.getAttribute("aria-expanded")).toBe("true");
    await user.tab();
    expect(document.activeElement).toBe(
      screen.getByRole("button", { name: "Outside" }),
    );
    expect(trigger.getAttribute("aria-expanded")).toBe("false");
  });

  it("stays open when a click blurs to nothing focusable inside it", async () => {
    const { user, trigger } = setup();
    await user.click(trigger);
    fireEvent.blur(trigger, { relatedTarget: null });
    expect(trigger.getAttribute("aria-expanded")).toBe("true");
  });

  it("closes from a choice that finishes, returning focus to its button", async () => {
    const { user, trigger } = setup();
    await user.click(trigger);
    await user.click(screen.getByRole("button", { name: "Done" }));
    expect(trigger.getAttribute("aria-expanded")).toBe("false");
    expect(document.activeElement).toBe(trigger);
  });
});
