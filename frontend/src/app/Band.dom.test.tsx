import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { initialLifecycleState } from "../lifecycle/state";
import type { SingleserveLifecycle } from "../lifecycle/useSingleserve";
import { Band } from "./Band";
import { SettingsMenu } from "./SettingsMenu";

const lifecycle: SingleserveLifecycle = {
  state: { ...initialLifecycleState, phase: "connected" },
  ready: true,
  checking: false,
  fetch: vi.fn(),
  checkHealth: vi.fn(),
  quit: vi.fn(),
  dismissDenial: vi.fn(),
};

function renderBand(view: "board" | "ticket") {
  render(
    <Band
      lifecycle={lifecycle}
      view={view}
      onView={vi.fn()}
      needsYou={0}
      needsYouOn={false}
      onNeedsYou={vi.fn()}
      settings={
        <SettingsMenu
          root="/board"
          info={{ status: "loading" }}
          theme="system"
          onTheme={vi.fn()}
        />
      }
    />,
  );
}

// The gear sits in the band's right-hand group, between the backend status
// and Quit, on the views and the ticket page alike (FH-149).
describe("Band", () => {
  it.each(["board", "ticket"] as const)(
    "puts Settings between the backend status and Quit on %s",
    (view) => {
      renderBand(view);
      const names = screen
        .getAllByRole("button")
        .map(
          (button) => button.getAttribute("aria-label") ?? button.textContent,
        );
      const status = names.indexOf("Backend connected. Check connection");
      const settings = names.indexOf("Settings");
      const quit = names.indexOf("Quit");
      expect(status).toBeGreaterThan(-1);
      expect(settings).toBe(status + 1);
      expect(quit).toBe(settings + 1);
    },
  );
});
