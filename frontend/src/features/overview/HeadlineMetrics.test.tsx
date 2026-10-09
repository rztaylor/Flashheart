import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { Metrics } from "../../api/metrics";
import { dayAndTime } from "../../model/time";
import { HeadlineMetrics } from "./HeadlineMetrics";

const metrics: Metrics = {
  revision: 1,
  since: "2026-10-08T18:00:00Z",
  lastChange: true,
  done: 0,
  review: 3,
  created: 6,
  criteriaTicked: 9,
};
const now = new Date("2026-10-09T12:00:00Z");

const text = (html: string) =>
  html
    .replace(/<[^>]+>/g, " ")
    .replace(/\s+/g, " ")
    .trim();

describe("HeadlineMetrics", () => {
  it("labels the time it counts from and draws a tile per metric", () => {
    const html = renderToStaticMarkup(
      <HeadlineMetrics metrics={metrics} now={now} />,
    );
    expect(text(html)).toBe(
      `Since your last change · ${dayAndTime(metrics.since, now)} ` +
        "Done 0 To review 3 New tickets 6 Criteria ticked 9",
    );
    expect(html).toContain(`dateTime="${metrics.since}"`);
    // The word names each number for assistive technology; the tiles read
    // as a description list, number over word only visually.
    expect(html.match(/<dt/g)).toHaveLength(4);
    expect(html.match(/<dd[^>]*tabular-nums/g)).toHaveLength(4);
    expect(html).not.toMatch(/Need you/i);
  });

  it("says when it counts the last 24 hours", () => {
    const html = renderToStaticMarkup(
      <HeadlineMetrics metrics={{ ...metrics, lastChange: false }} now={now} />,
    );
    expect(text(html)).toMatch(/^In the last 24 hours Done 0/);
    expect(html).not.toContain("<time");
  });

  it("carries no meaning in colour: ink only", () => {
    const html = renderToStaticMarkup(
      <HeadlineMetrics metrics={metrics} now={now} />,
    );
    expect(html).not.toMatch(/attention|danger|success|state-|accent/);
  });
});
