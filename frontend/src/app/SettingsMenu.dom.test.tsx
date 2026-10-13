import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { ThemePreference } from "../api/info";
import { SettingsMenu } from "./SettingsMenu";
import type { ServerInfoState } from "./useServerInfo";

const root = "/Users/someone/reports/Kanban";

const ready: ServerInfoState = {
  status: "ready",
  info: {
    name: "Flashheart",
    version: "0.9.0",
    commit: "abc1234",
    buildDate: "2026-10-13",
    protocolVersion: 2,
    root,
    theme: "system",
    answers: true,
  },
};

function setup({
  theme = "system",
  info = ready,
}: {
  theme?: ThemePreference;
  info?: ServerInfoState;
} = {}) {
  const user = userEvent.setup();
  const onTheme = vi.fn();
  render(
    <>
      <SettingsMenu root={root} info={info} theme={theme} onTheme={onTheme} />
      <button type="button">Quit</button>
    </>,
  );
  const gear = screen.getByRole("button", { name: "Settings" });
  return { user, onTheme, gear };
}

// The band's Settings menu (FH-149): the app-wide settings that sat in the
// rail's footer, behind a gear on every screen.
describe("SettingsMenu", () => {
  it("is a closed gear button named Settings, its name also its tooltip", () => {
    const { gear } = setup();
    expect(gear.getAttribute("aria-expanded")).toBe("false");
    expect(gear.getAttribute("title")).toBe("Settings");
    expect(screen.queryByRole("group", { name: "Theme" })).toBeNull();
  });

  it("holds the theme choice, the board root and the version line", async () => {
    const { user, gear } = setup({ theme: "dark" });
    await user.click(gear);
    const theme = screen.getByRole("group", { name: "Theme" });
    expect(
      within(theme)
        .getAllByRole("radio")
        .map((radio) => radio.getAttribute("value")),
    ).toEqual(["system", "light", "dark"]);
    expect(within(theme).getByRole("radio", { name: "Dark" })).toHaveProperty(
      "checked",
      true,
    );
    // The root is cut from its start, with the whole path in its tooltip.
    const path = screen.getByText(root);
    expect(path.closest("[title]")?.getAttribute("title")).toBe(root);
    expect(screen.getByText("0.9.0 · agent protocol 2")).toBeTruthy();
  });

  it("leaves the version line out until the server has answered", async () => {
    const { user, gear } = setup({ info: { status: "loading" } });
    await user.click(gear);
    expect(screen.getByText(root)).toBeTruthy();
    expect(screen.queryByText(/agent protocol/)).toBeNull();
  });

  it("saves a theme chosen in it at once and stays open", async () => {
    const { user, gear, onTheme } = setup();
    await user.click(gear);
    await user.click(screen.getByText("Light", { exact: true }));
    expect(onTheme).toHaveBeenCalledWith("light");
    expect(gear.getAttribute("aria-expanded")).toBe("true");
  });

  it("opens with Enter or Space, and Escape closes it back on the gear", async () => {
    const { user, gear } = setup();
    gear.focus();
    await user.keyboard("{Enter}");
    expect(gear.getAttribute("aria-expanded")).toBe("true");
    await user.tab();
    expect(document.activeElement).toBe(
      screen.getByRole("radio", { name: "System" }),
    );
    await user.keyboard("{Escape}");
    expect(gear.getAttribute("aria-expanded")).toBe("false");
    expect(document.activeElement).toBe(gear);

    await user.keyboard(" ");
    expect(gear.getAttribute("aria-expanded")).toBe("true");
    await user.keyboard("{Escape}");
    expect(gear.getAttribute("aria-expanded")).toBe("false");
    expect(document.activeElement).toBe(gear);
  });

  it("chooses a theme from the keyboard", async () => {
    const { user, gear, onTheme } = setup();
    gear.focus();
    await user.keyboard("{Enter}");
    await user.tab();
    await user.keyboard("{ArrowRight}");
    expect(onTheme).toHaveBeenCalledWith("light");
  });
});
