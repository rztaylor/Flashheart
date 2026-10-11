import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { Workstream, WorkstreamTicket } from "../../api/board";
import { WorkstreamLine } from "./WorkstreamsView";

// A finished workstream (every ticket Done, Rejected or Archived) shows as
// its route card head with a disclosure chevron and no stations until the
// user expands it; every other workstream shows in full (FH-68,
// ui-layout.md §4).
const ticket = (
  id: string,
  column: WorkstreamTicket["column"],
  extra: Partial<WorkstreamTicket> = {},
): WorkstreamTicket => ({
  id,
  title: `Ticket ${id}`,
  column,
  blocked: false,
  held: false,
  missing: false,
  dependsOn: [],
  outside: [],
  ...extra,
});

const workstream = (
  tickets: WorkstreamTicket[],
  status: Workstream["status"] = "completed",
): Workstream => ({
  slug: "foundation",
  title: "Foundation",
  status,
  suspended: false,
  declaredStatus: "",
  priority: "high",
  created: "2026-10-01",
  done: tickets.filter((t) => t.column === "done").length,
  total: tickets.length,
  next: "",
  blockedBy: [],
  dependsOnWorkstreams: [],
  tags: [],
  needsRepair: [],
  warnings: [],
  tickets,
});

// chevron matches the chevronDown icon's svg, with its classes.
const chevron = /<svg[^>]*class="([^"]*)"[^>]*><path d="M6 9l6 6 6-6"/;

const render = (line: Workstream, expanded = false) =>
  renderToStaticMarkup(
    <WorkstreamLine
      headingLevel={2}
      project="flashheart"
      workstream={line}
      line={{ colour: 1, initials: "FO" }}
      expanded={expanded}
      onToggle={() => undefined}
      onOpen={() => undefined}
    />,
  );

const finished = workstream([
  ticket("FH-1", "done"),
  ticket("FH-2", "done", { rejected: true }),
  ticket("FH-3", "archived"),
]);

describe("WorkstreamLine", () => {
  it("collapses a finished workstream to its head", () => {
    const markup = render(finished);
    expect(markup).toMatch(/<button[^>]*aria-expanded="false"/);
    expect(markup).toContain("Foundation");
    expect(markup).toContain("Completed");
    expect(markup.replace(/<[^>]*>/g, "")).toContain("2 of 3 served");
    expect(markup).toMatch(chevron);
    expect(markup).not.toContain("data-station");
  });

  it("names the workstream in the disclosure button", () => {
    const markup = render(finished);
    const button = markup.slice(
      markup.indexOf("<button"),
      markup.indexOf("</button>"),
    );
    expect(button).toContain("Foundation");
  });

  it("shows the full card when the user expanded it", () => {
    const markup = render(finished, true);
    expect(markup).toMatch(/<button[^>]*aria-expanded="true"/);
    expect(markup).toContain('data-station="FH-1"');
  });

  it("stays expanded while a ticket waits in Ready to review", () => {
    const markup = render(
      workstream([ticket("FH-1", "done"), ticket("FH-2", "review")]),
    );
    expect(markup).toContain("Completed");
    expect(markup).not.toContain("aria-expanded");
    expect(markup).toContain('data-station="FH-2"');
  });

  it("always shows an active workstream in full", () => {
    const markup = render(
      workstream([ticket("FH-1", "done"), ticket("FH-2", "up-next")], "active"),
    );
    expect(markup).toContain("Running");
    expect(markup).not.toContain("aria-expanded");
    expect(markup).toContain('data-station="FH-2"');
  });

  it("does not animate the chevron beyond the token-driven transition", () => {
    // Reduced motion sets the default transition duration to 0 ms
    // (styles/index.css); an explicit duration would escape it.
    const classes = render(finished).match(chevron)?.[1] ?? "";
    expect(classes).toContain("transition-transform");
    expect(classes).not.toMatch(/duration-/);
  });
});
