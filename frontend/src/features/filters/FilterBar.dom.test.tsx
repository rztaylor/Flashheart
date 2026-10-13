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
