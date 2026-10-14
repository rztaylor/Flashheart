import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { ProjectSummary } from "../../api/board";
import { noRuns } from "../../api/runs";
import { ProjectCounts } from "./ProjectCounts";

function project(
  name: string,
  key: string,
  extra: Partial<ProjectSummary> = {},
): ProjectSummary {
  return {
    name,
    displayName: name.charAt(0).toUpperCase() + name.slice(1),
    key,
    keyDerived: false,
    repos: [],
    counts: { backlog: 2, "up-next": 1, "in-progress": 1, review: 0, done: 3 },
    needsRepair: 0,
    blocked: 0,
    warnings: [],
    lastModified: "",
    workstreams: [],
    archived: 0,
    rejected: 0,
    runs: noRuns,
    ...extra,
  };
}

// The rail's order, most recently active first: quill before flashheart.
const projects = [
  project("quill", "QUI"),
  project("flashheart", "FH", {
    counts: {
      backlog: 10,
      "up-next": 4,
      "in-progress": 3,
      review: 3,
      done: 30,
    },
    blocked: 22,
    needsRepair: 1,
    runs: { ...noRuns, working: 2, quiet: 1, needsYou: 2, live: 5 },
  }),
];

function setup() {
  render(
    <ProjectCounts
      projects={projects}
      hrefFor={(name) => `#/p/${name}/overview`}
    />,
  );
  const list = screen.getByRole("region", { name: "Projects" });
  return { list, rows: within(list).getAllByRole("link") };
}

describe("ProjectCounts (FH-153)", () => {
  it("lists every project in the rail's order, each row leading to it", () => {
    const { rows } = setup();
    expect(rows).toHaveLength(2);
    expect(rows.map((row) => row.getAttribute("href"))).toEqual([
      "#/p/quill/overview",
      "#/p/flashheart/overview",
    ]);
  });

  it("names each row with the project and its counts", () => {
    const { rows } = setup();
    expect(rows[1]?.getAttribute("aria-label")).toBe(
      "Flashheart (flashheart): 50 tickets (10 in backlog, 4 up next, 3 in progress, 3 ready to review, 30 done), 22 blocked, 1 needs repair, 3 agents live, 2 agents need you",
    );
    expect(rows[0]?.getAttribute("aria-label")).toMatch(
      /^Quill \(quill\): 7 tickets \(/,
    );
  });

  it("shows the key, name and counts, with the column words", () => {
    const { list, rows } = setup();
    const flashheart = rows[1] as HTMLElement;
    expect(flashheart.textContent).toContain("FH");
    expect(flashheart.textContent).toContain("Flashheart");
    for (const value of ["50", "10", "4", "30", "22"])
      expect(within(flashheart).getAllByText(value).length).toBeGreaterThan(0);
    for (const word of [
      "Tickets",
      "Backlog",
      "Up next",
      "In progress",
      "Ready to review",
      "Done",
      "Blocked",
      "Needs repair",
      "Live",
      "Needs you",
    ])
      expect(within(list).getAllByText(word).length).toBeGreaterThan(0);
  });

  it("marks Needs you and repair only where they apply", () => {
    const { rows } = setup();
    const [quill, flashheart] = rows as [HTMLElement, HTMLElement];
    expect(flashheart.querySelector("[data-mark=needs-you]")?.textContent).toBe(
      "2",
    );
    expect(flashheart.querySelector("[data-mark=repair]")?.textContent).toBe(
      "1",
    );
    expect(quill.querySelector("[data-mark]")).toBeNull();
  });

  // The live count (FH-162) sits before Needs you, a zero faint like the
  // other counts; stacked, it follows tickets and blocked in words.
  it("shows each project's live agents, a zero faint", () => {
    const { list, rows } = setup();
    const [quill, flashheart] = rows as [HTMLElement, HTMLElement];
    const live = (row: HTMLElement) =>
      row.querySelector<HTMLElement>("[data-count=live]");
    expect(live(flashheart)?.textContent).toBe("3");
    expect(live(flashheart)?.className).toContain("text-ink");
    expect(live(flashheart)?.className).not.toContain("text-ink-faint");
    expect(live(quill)?.textContent).toBe("0");
    expect(live(quill)?.className).toContain("text-ink-faint");
    // The column word carries the run-state mark, as the board columns
    // carry their status icons.
    const head = list.querySelector("[data-columns]") as HTMLElement;
    expect(
      within(head).getByText("Live").querySelector("[data-run-state]"),
    ).not.toBeNull();
    expect(flashheart.textContent).toContain("22 blocked · 3 live");
    expect(quill.textContent).toContain("0 blocked · 0 live");
  });

  it("shows nothing with no projects", () => {
    const { container } = render(
      <ProjectCounts projects={[]} hrefFor={() => ""} />,
    );
    expect(container.innerHTML).toBe("");
  });
});
