import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { ProjectSummary, WorkstreamBrief } from "../../api/board";
import { noRuns } from "../../api/runs";
import { NewWorkstreamDialog } from "./NewWorkstreamDialog";

const brief = (slug: string, title: string): WorkstreamBrief => ({
  slug,
  title,
  created: "2026-10-01",
  status: "active",
  done: 0,
  total: 2,
});

const project = (
  name: string,
  workstreams: WorkstreamBrief[] = [],
): ProjectSummary => ({
  name,
  displayName: name[0]?.toUpperCase() + name.slice(1),
  key: name.slice(0, 2).toUpperCase(),
  keyDerived: false,
  repos: [],
  counts: {
    backlog: 0,
    "up-next": 0,
    "in-progress": 0,
    review: 0,
    done: 0,
  },
  needsRepair: 0,
  blocked: 0,
  warnings: [],
  lastModified: "",
  workstreams,
  archived: 0,
  rejected: 0,
  runs: noRuns,
});

const flashheart = project("flashheart", [
  brief("board-ui", "Board UI"),
  brief("offline", "Offline"),
]);

// server answers each request with status and body, recording the calls.
function server(status: number, body: unknown) {
  const calls: { url: string; init?: RequestInit }[] = [];
  const fetcher = vi.fn(
    async (input: RequestInfo | URL, init?: RequestInit) => {
      calls.push({ url: String(input), init });
      return new Response(JSON.stringify(body), { status });
    },
  );
  return { fetcher, calls };
}

// New workstream (FH-17, ui-layout.md §4): title, goal, priority and the
// workstreams it depends on, written from board-format's template; the
// keyboard and focus follow New ticket's.
describe("NewWorkstreamDialog", () => {
  it("creates a workstream from its fields and reports the new slug", async () => {
    const user = userEvent.setup();
    const { fetcher, calls } = server(201, {
      slug: "workstream-editing",
      hash: "h1",
      revision: 7,
    });
    const onCreated = vi.fn();
    render(
      <NewWorkstreamDialog
        fetcher={fetcher}
        projects={[flashheart]}
        project="flashheart"
        onCreated={onCreated}
        onClose={() => undefined}
      />,
    );
    const dialog = screen.getByRole("dialog", { name: "New workstream" });
    const title = within(dialog).getByRole("textbox", { name: "Title" });
    expect(document.activeElement).toBe(title);
    const create = within(dialog).getByRole("button", {
      name: "Create workstream",
    });
    expect(create).toHaveProperty("disabled", true);

    await user.type(title, "  Workstream editing ");
    await user.type(
      within(dialog).getByRole("textbox", { name: "Goal" }),
      "Make and fill workstreams on the board.",
    );
    await user.selectOptions(
      within(dialog).getByRole("combobox", { name: "Priority" }),
      "high",
    );
    const dependencies = within(dialog).getByRole("group", {
      name: "Depends on workstreams",
    });
    await user.click(
      within(dependencies).getByRole("checkbox", { name: "Board UI" }),
    );
    expect(create).toHaveProperty("disabled", false);
    // Enter in the title submits too; happy-dom has no implicit submission
    // through a button outside the form, so the browser spec presses it.
    await user.click(create);

    await waitFor(() => expect(onCreated).toHaveBeenCalledOnce());
    expect(onCreated).toHaveBeenCalledWith({
      project: "flashheart",
      slug: "workstream-editing",
      title: "Workstream editing",
    });
    expect(calls).toHaveLength(1);
    expect(calls[0]?.url).toBe("/api/projects/flashheart/workstreams");
    expect(calls[0]?.init?.method).toBe("POST");
    expect(JSON.parse(String(calls[0]?.init?.body))).toEqual({
      title: "Workstream editing",
      goal: "Make and fill workstreams on the board.",
      priority: "high",
      dependsOnWorkstreams: ["board-ui"],
    });
  });

  it("shows a refusal in words and keeps the form", async () => {
    const user = userEvent.setup();
    const { fetcher } = server(404, {
      error: {
        code: "not_found",
        message:
          'workstream "offline" is not in flashheart; its workstreams: board-ui',
      },
    });
    const onCreated = vi.fn();
    render(
      <NewWorkstreamDialog
        fetcher={fetcher}
        projects={[flashheart]}
        project="flashheart"
        onCreated={onCreated}
        onClose={() => undefined}
      />,
    );
    await user.type(screen.getByRole("textbox", { name: "Title" }), "Later");
    await user.click(screen.getByRole("checkbox", { name: "Offline" }));
    await user.click(screen.getByRole("button", { name: "Create workstream" }));
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain('workstream "offline" is not in');
    expect(onCreated).not.toHaveBeenCalled();
    expect(screen.getByRole("textbox", { name: "Title" })).toHaveProperty(
      "value",
      "Later",
    );
    expect(
      screen.getByRole("button", { name: "Create workstream" }),
    ).toHaveProperty("disabled", false);
  });

  it("offers dependencies only where the project has workstreams, and the project only in the all scope", async () => {
    const user = userEvent.setup();
    const { fetcher } = server(201, { slug: "x", hash: "h", revision: 1 });
    render(
      <NewWorkstreamDialog
        fetcher={fetcher}
        projects={[flashheart, project("alpha")]}
        project=""
        onCreated={() => undefined}
        onClose={() => undefined}
      />,
    );
    const picker = screen.getByRole("combobox", { name: "Project" });
    await user.click(screen.getByRole("checkbox", { name: "Board UI" }));
    // Dependencies belong to the chosen project.
    await user.selectOptions(picker, "alpha");
    expect(
      screen.queryByRole("group", { name: "Depends on workstreams" }),
    ).toBeNull();
    await user.selectOptions(picker, "flashheart");
    expect(screen.getByRole("checkbox", { name: "Board UI" })).toHaveProperty(
      "checked",
      false,
    );
  });

  it("closes on Cancel and Escape", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(
      <NewWorkstreamDialog
        fetcher={server(201, {}).fetcher}
        projects={[flashheart]}
        project="flashheart"
        onCreated={() => undefined}
        onClose={onClose}
      />,
    );
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    await user.keyboard("{Escape}");
    expect(onClose).toHaveBeenCalledTimes(2);
  });
});
