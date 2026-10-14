import { describe, expect, it } from "vitest";

import type { Workstream, WorkstreamTicket } from "../../api/board";
import { successorOf } from "./successor";

const member = (id: string, missing = false) =>
  ({ id, missing }) as WorkstreamTicket;
const line = (
  slug: string,
  title: string,
  tickets: WorkstreamTicket[] = [],
): Workstream => ({ slug, title, tickets }) as unknown as Workstream;

const last = line("board-ui", "Board UI", [member("AL-1"), member("AL-2")]);

// FH-174, D80: the workstream a page's vanished one became, found among the
// project's workstreams by its name (unique in a project, D76) or by its
// tickets, all now in one other workstream.
describe("successorOf", () => {
  it("is the workstream with the same name", () => {
    expect(
      successorOf(line("board-ui", "Board  ui"), [
        line("cli", "CLI"),
        line("board", "board UI"),
      ])?.slug,
    ).toBe("board");
  });

  it("is the workstream that all its tickets are in, for a new name too", () => {
    expect(
      successorOf(last, [
        line("cli", "CLI", [member("AL-3")]),
        line("board", "Board", [
          member("AL-2"),
          member("AL-1"),
          member("AL-4"),
        ]),
      ])?.slug,
    ).toBe("board");
  });

  it("is none when its tickets went different ways, or nowhere", () => {
    expect(
      successorOf(last, [
        line("cli", "CLI", [member("AL-1")]),
        line("board", "Board", [member("AL-2")]),
      ]),
    ).toBeUndefined();
    expect(successorOf(last, [line("cli", "CLI")])).toBeUndefined();
    // Tickets listed but missing from the board say nothing.
    expect(
      successorOf(line("x", "X", [member("AL-9", true)]), [
        line("y", "Y", [member("AL-9", true)]),
      ]),
    ).toBeUndefined();
  });

  it("is none when the name and the tickets point at different ones", () => {
    expect(
      successorOf(last, [
        line("ui", "Board UI"),
        line("board", "Board", [member("AL-1"), member("AL-2")]),
      ]),
    ).toBeUndefined();
  });

  it("is never the vanished slug itself", () => {
    expect(successorOf(last, [line("board-ui", "Board UI")])).toBeUndefined();
  });
});
