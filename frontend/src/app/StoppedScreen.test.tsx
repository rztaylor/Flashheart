import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { StoppedScreen } from "./StoppedScreen";

describe("StoppedScreen", () => {
  it("confirms an accepted quit and explains how to close the tab", () => {
    const markup = renderToStaticMarkup(
      <StoppedScreen phase="stopped" failures={0} />,
    );
    expect(markup).toContain("Flashheart has stopped");
    expect(markup).toContain("Close tab");
    expect(markup).toContain("close it yourself");
  });

  it("explains a lost backend and how to restart", () => {
    const markup = renderToStaticMarkup(
      <StoppedScreen phase="lost" failures={3} />,
    );
    expect(markup).toContain("Lost connection to Flashheart");
    expect(markup).toContain("3 missed heartbeats");
    expect(markup).toContain("<code>flashheart</code>");
  });

  it("explains a failed connection with the browser's reason", () => {
    const markup = renderToStaticMarkup(
      <StoppedScreen phase="failed" failures={0} detail="Unauthorized" />,
    );
    expect(markup).toContain("Could not connect to Flashheart");
    expect(markup).toContain("Unauthorized");
  });
});
