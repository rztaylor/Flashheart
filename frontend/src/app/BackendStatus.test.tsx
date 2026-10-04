import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

import { initialLifecycleState, type LifecycleState } from "../lifecycle/state";
import { BackendStatus } from "./BackendStatus";

function render(state: Partial<LifecycleState>, checking = false) {
  return renderToStaticMarkup(
    <BackendStatus
      state={{ ...initialLifecycleState, ...state }}
      checking={checking}
      onCheck={vi.fn()}
    />,
  );
}

describe("BackendStatus", () => {
  it("is visually quiet but named when healthy", () => {
    const markup = render({ phase: "connected" });
    expect(markup).toContain(
      'aria-label="Backend connected. Check connection"',
    );
    expect(markup).not.toContain("Reconnecting");
  });

  it("explains a degraded connection in visible text", () => {
    const markup = render({ phase: "degraded", failures: 2 });
    expect(markup).toContain("Reconnecting · 2 missed heartbeats");
  });

  it("reports the latest manual health check", () => {
    const markup = render({
      phase: "connected",
      health: { ok: false, at: new Date("2026-10-04T12:00:00Z") },
    });
    expect(markup).toContain("Not responding");
    expect(markup).toContain('aria-live="polite"');
  });

  it("shows progress while checking", () => {
    expect(render({ phase: "connected" }, true)).toContain("Checking");
  });
});
