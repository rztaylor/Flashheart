import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { ArchiveCheck, ProjectDeletePlan } from "../../api/archive";
import { ArchiveSummary } from "./ArchiveProjectDialog";
import { ProjectDeletePreview } from "./DeleteProjectDialog";

const check: ArchiveCheck = {
  name: "alpha",
  displayName: "Alpha",
  tickets: 7,
  counts: { backlog: 3, "up-next": 1, "in-progress": 1, review: 1, done: 1 },
  liveRuns: 0,
  claimed: 0,
  openQuestions: 0,
};

describe("archiving a project", () => {
  it("shows its tickets by column", () => {
    const html = renderToStaticMarkup(<ArchiveSummary check={check} />);
    expect(html).toContain("7 tickets");
    expect(html).toContain("Backlog");
    expect(html).not.toContain("still");
  });

  it("warns about live runs, claimed tickets and open questions", () => {
    const html = renderToStaticMarkup(
      <ArchiveSummary
        check={{ ...check, liveRuns: 2, claimed: 1, openQuestions: 1 }}
      />,
    );
    expect(html).toContain("2 agent sessions are still running");
    expect(html).toContain("1 ticket is being worked on");
    expect(html).toContain("1 question is waiting for an answer");
  });
});

describe("deleting a project", () => {
  const plan: ProjectDeletePlan = {
    name: "alpha",
    displayName: "Alpha",
    key: "AL",
    tickets: 8,
    references: [
      { project: "beta", id: "BE-1", title: "Hello", dependsOn: ["AL-3"] },
    ],
    token: "t",
  };

  it("lists what goes and what is rewritten", () => {
    const html = renderToStaticMarkup(<ProjectDeletePreview plan={plan} />);
    for (const text of [
      "8 tickets",
      "BE-1",
      "AL-3",
      "AL stays reserved",
      "cannot be undone",
    ]) {
      expect(html).toContain(text);
    }
  });
});
