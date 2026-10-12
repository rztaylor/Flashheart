import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it } from "vitest";

import { panelId, Tabs } from "./Tabs";

const items = [
  { id: "plan", label: "Plan" },
  { id: "runs", label: "Runs" },
  { id: "files", label: "Files" },
];

function Controlled({ initial = "plan" }: { initial?: string }) {
  const [selected, setSelected] = useState(initial);
  return (
    <>
      <button type="button">Before</button>
      <Tabs
        idPrefix="card"
        label="Ticket sections"
        items={items}
        selected={selected}
        onSelect={setSelected}
      />
      <button type="button">After</button>
    </>
  );
}

const tab = (name: string) => screen.getByRole("tab", { name });

function expectSelected(name: string) {
  expect(tab(name).getAttribute("aria-selected")).toBe("true");
  expect(document.activeElement).toBe(tab(name));
}

// The ARIA tablist: one tab stop, arrows move and select with wrapping,
// Home and End jump to the ends.
describe("Tabs", () => {
  it("is one tab stop, on the selected tab, tied to its panel", async () => {
    const user = userEvent.setup();
    render(<Controlled initial="runs" />);
    expect(
      screen.getByRole("tablist", { name: "Ticket sections" }),
    ).toBeTruthy();
    expect(tab("Runs").getAttribute("aria-controls")).toBe(
      panelId("card", "runs"),
    );
    screen.getByRole("button", { name: "Before" }).focus();
    await user.tab();
    expect(document.activeElement).toBe(tab("Runs"));
    await user.tab();
    expect(document.activeElement).toBe(
      screen.getByRole("button", { name: "After" }),
    );
  });

  it("moves and selects with the arrows, wrapping at the ends", async () => {
    const user = userEvent.setup();
    render(<Controlled />);
    tab("Plan").focus();
    await user.keyboard("{ArrowRight}");
    expectSelected("Runs");
    await user.keyboard("{ArrowRight}{ArrowRight}");
    expectSelected("Plan");
    await user.keyboard("{ArrowLeft}");
    expectSelected("Files");
    expect(tab("Plan").getAttribute("aria-selected")).toBe("false");
    expect(tab("Plan").tabIndex).toBe(-1);
    expect(tab("Files").tabIndex).toBe(0);
  });

  it("jumps to the first and last tab with Home and End", async () => {
    const user = userEvent.setup();
    render(<Controlled initial="runs" />);
    tab("Runs").focus();
    await user.keyboard("{End}");
    expectSelected("Files");
    await user.keyboard("{Home}");
    expectSelected("Plan");
  });

  it("ignores other keys and selects on click", async () => {
    const user = userEvent.setup();
    render(<Controlled />);
    tab("Plan").focus();
    await user.keyboard("{ArrowDown}x");
    expectSelected("Plan");
    await user.click(tab("Files"));
    expectSelected("Files");
  });
});
