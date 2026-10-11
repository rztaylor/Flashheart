import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { pipelineStages } from "../../model/overview";
import { PipelineHeadline } from "./PipelineHeadline";

const stages = pipelineStages(
  [
    {
      name: "alpha",
      counts: {
        backlog: 12,
        "up-next": 1,
        "in-progress": 0,
        review: 4,
        done: 44,
      },
    },
  ],
  "alpha",
);

const html = renderToStaticMarkup(
  <PipelineHeadline stages={stages} boardHref="#/p/alpha/board" />,
);

const text = (markup: string) =>
  markup
    .replace(/<[^>]+>/g, " ")
    .replace(/\s+/g, " ")
    .trim();

describe("PipelineHeadline", () => {
  it("is a named list of the stages, each a link to the board", () => {
    expect(html).toMatch(/^<nav aria-label="Pipeline"[^>]*><ol/);
    expect([...html.matchAll(/<li\b/g)]).toHaveLength(5);
    expect(
      [...html.matchAll(/<a [^>]*aria-label="([^"]+)"/g)].map(
        (match) => match[1],
      ),
    ).toEqual([
      "Backlog: 12 tickets",
      "Up next: 1 ticket",
      "In progress: 0 tickets",
      "Ready to review: 4 tickets",
      "Done: 44 tickets",
    ]);
    expect(
      [...html.matchAll(/<a [^>]*href="([^"]+)"/g)].map((match) => match[1]),
    ).toEqual(Array(5).fill("#/p/alpha/board"));
  });

  it("shows each count over its column's word, in order", () => {
    expect(text(html)).toBe(
      "12 Backlog 1 Up next 0 In progress 4 Ready to review 44 Done",
    );
  });

  it("marks each stage with its column and state tone", () => {
    expect(
      [...html.matchAll(/data-stage="([^"]+)" data-tone="([^"]+)"/g)].map(
        (match) => `${match[1]}:${match[2]}`,
      ),
    ).toEqual([
      "backlog:neutral",
      "up-next:neutral",
      "in-progress:progress",
      "review:review",
      "done:done",
    ]);
  });
});
