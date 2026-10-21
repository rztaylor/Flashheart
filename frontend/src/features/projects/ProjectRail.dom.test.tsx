import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { ProjectSummary } from "../../api/board";
import { noRuns } from "../../api/runs";
import type { Scope } from "../../model/scopes";
import { ProjectRail } from "./ProjectRail";

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

const projects = [
  project("flashheart", "FH", {
    counts: {
      backlog: 10,
      "up-next": 4,
      "in-progress": 3,
      review: 3,
      done: 30,
    },
    blocked: 22,
    runs: { ...noRuns, needsYou: 2 },
  }),
  project("gamma", "GAM", { needsRepair: 1 }),
  project("quill", "QUI"),
];

function setup(scope: Scope = { kind: "all" }) {
  const onSelect = vi.fn();
  render(
    <ProjectRail
      projects={projects}
      runs={{ ...noRuns, needsYou: 2 }}
      scope={scope}
      onSelect={onSelect}
    />,
  );
  const rail = screen.getByRole("navigation", { name: "Projects" });
  // All projects and the three projects.
  const badges = within(rail).getAllByRole("button") as [
    HTMLElement,
    HTMLElement,
    HTMLElement,
    HTMLElement,
  ];
  return { rail, badges, onSelect };
}

// The slim rail (FH-150): key badges only, keeping what needs the user on
// the badge and the rest in its name and tooltip.
describe("ProjectRail", () => {
  it("is All projects, then a badge per project in the order given", () => {
    const { badges } = setup();
    expect(
      badges.map((badge) => badge.textContent?.replace(/\d/g, "").trim()),
    ).toEqual(["", "FH", "GAM", "QUI"]);
    expect(badges[0].getAttribute("aria-label")).toMatch(
      /^All projects \(3 projects\)/,
    );
  });

  it("shows no names, ticket counts, route bars or in-progress and blocked counts", () => {
    const { rail } = setup();
    expect(rail.textContent).not.toMatch(/Flashheart|Gamma|All projects/);
    expect(rail.textContent).not.toMatch(/in progress|blocked/);
    expect(rail.textContent).not.toContain("50");
    expect(within(rail).queryByRole("img")).toBeNull();
  });

  // A name starts with what the badge shows, the key and then the Needs
  // you count, so a voice user can say it (WCAG 2.5.3, FH-184).
  it("names each badge, in its name and tooltip, with the project, its key and its counts", () => {
    const { badges } = setup();
    const flashheart =
      "FH, 2 agents need you: Flashheart (flashheart), 50 tickets, 3 in progress, 22 blocked";
    expect(badges[1].getAttribute("aria-label")).toBe(flashheart);
    expect(badges[1].getAttribute("title")).toBe(flashheart);
    expect(badges[2].getAttribute("aria-label")).toBe(
      "GAM: Gamma (gamma), 7 tickets, 1 in progress, 1 needs repair",
    );
    expect(badges[0].getAttribute("aria-label")).toBe(
      "All projects (3 projects): 64 tickets, 5 in progress, 22 blocked, 1 needs repair, 2 agents need you",
    );
  });

  it("shows a Needs you count only on badges whose runs need the user", () => {
    const { badges } = setup();
    const count = (badge: HTMLElement) =>
      badge.querySelector("[data-mark=needs-you]")?.textContent?.trim();
    expect(count(badges[0])).toBe("2");
    expect(count(badges[1])).toBe("2");
    expect(count(badges[2])).toBeUndefined();
    expect(count(badges[3])).toBeUndefined();
  });

  it("marks only badges with a ticket that needs repair", () => {
    const { badges } = setup();
    const marked = badges.map(
      (badge) => badge.querySelector("[data-mark=repair]") !== null,
    );
    expect(marked).toEqual([true, false, true, false]);
  });

  it("fills the current scope's badge and switches scope from a badge", async () => {
    const user = userEvent.setup();
    const { badges, onSelect } = setup({ kind: "project", project: "gamma" });
    expect(badges.map((badge) => badge.getAttribute("aria-current"))).toEqual([
      null,
      null,
      "page",
      null,
    ]);
    await user.click(badges[3]);
    expect(onSelect).toHaveBeenCalledWith({
      kind: "project",
      project: "quill",
    });
    await user.click(badges[0]);
    expect(onSelect).toHaveBeenCalledWith({ kind: "all" });
  });

  // A project's own image (FH-151) covers its badge, clipped to the
  // badge's shape; the key stays in the name and tooltip, and the marks
  // stay on top.
  it("shows a project's image on its badge and keeps the key in its name", () => {
    render(
      <ProjectRail
        projects={[
          project("flashheart", "FH", {
            image: "/api/projects/flashheart/image?v=abcd1234",
          }),
          project("gamma", "GAM", {
            needsRepair: 1,
            image: "/api/projects/gamma/image?v=0f0f0f0f",
            runs: { ...noRuns, needsYou: 1 },
          }),
          project("quill", "QUI"),
        ]}
        runs={noRuns}
        scope={{ kind: "all" }}
        onSelect={vi.fn()}
      />,
    );
    const rail = screen.getByRole("navigation", { name: "Projects" });
    const [, flashheart, gamma, quill] = within(rail).getAllByRole("button");
    const image = (badge: HTMLElement) =>
      badge.querySelector<HTMLImageElement>("img[data-badge-image]");
    expect(image(flashheart as HTMLElement)?.getAttribute("src")).toBe(
      "/api/projects/flashheart/image?v=abcd1234",
    );
    expect(image(flashheart as HTMLElement)?.getAttribute("alt")).toBe("");
    expect(flashheart?.textContent?.replace(/\d/g, "").trim()).toBe("");
    expect(flashheart?.getAttribute("aria-label")).toMatch(
      /^FH(, [^:]+)?: Flashheart \(flashheart\), /,
    );
    expect(flashheart?.getAttribute("title")).toMatch(/^FH\b/);
    // The repair hatch and Needs you count sit over the image.
    expect(gamma?.querySelector("[data-mark=repair]")).not.toBeNull();
    expect(
      gamma?.querySelector("[data-mark=needs-you]")?.textContent?.trim(),
    ).toBe("1");
    expect(image(gamma as HTMLElement)).not.toBeNull();
    // A project without one shows its key as before.
    expect(image(quill as HTMLElement)).toBeNull();
    expect(quill?.textContent).toBe("QUI");
    // Decorative images are not announced.
    expect(within(rail).queryByRole("img")).toBeNull();
  });
});
