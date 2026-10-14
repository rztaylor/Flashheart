import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { emptyFilters } from "../../model/filters";
import { FilterBar } from "./FilterBar";

// The State menu marks its Blocked choice with the board's one blocked
// symbol, the no-entry sign (FH-148).
describe("FilterBar's State menu", () => {
  it("offers Blocked with the no-entry sign", async () => {
    const user = userEvent.setup();
    render(
      <FilterBar
        filters={emptyFilters}
        options={{ types: [], priorities: [], workstreams: [] }}
        onChange={() => undefined}
      />,
    );
    await user.click(screen.getByRole("button", { name: "State" }));
    const blocked = screen.getByRole("button", { name: "Blocked" });
    expect(blocked.querySelector('[data-icon="blocked"]')).not.toBeNull();
    const notBlocked = screen.getByRole("button", { name: "Not blocked" });
    expect(notBlocked.querySelector('[data-icon="blocked"]')).toBeNull();
  });
});

// The Priority menu lists each priority with the card's bars before the
// word, as the Colour by Priority chips do (FH-166).
describe("FilterBar's Priority menu", () => {
  it("shows each priority's bars before its word", async () => {
    const user = userEvent.setup();
    render(
      <FilterBar
        filters={emptyFilters}
        options={{
          types: ["bug"],
          priorities: ["high", "low"],
          workstreams: [],
        }}
        onChange={() => undefined}
      />,
    );
    await user.click(screen.getByRole("button", { name: "Priority" }));
    for (const [name, value] of [
      ["High", "high"],
      ["Low", "low"],
    ] as const) {
      const item = screen.getByRole("button", { name });
      const bars = item.querySelector("[data-priority]");
      expect(bars?.getAttribute("data-priority")).toBe(value);
      // The bars lead the word.
      expect(bars?.parentElement?.firstElementChild).toBe(bars);
      expect(bars?.parentElement?.textContent).toBe(name);
    }
    await user.keyboard("{Escape}");
    await user.click(screen.getByRole("button", { name: "Type" }));
    expect(
      screen
        .getByRole("button", { name: "bug" })
        .querySelector("[data-priority]"),
    ).toBeNull();
  });
});
