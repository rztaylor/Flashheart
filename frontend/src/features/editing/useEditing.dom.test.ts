import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { AuthenticatedFetch } from "../../api/client";
import { type Movable, useEditing } from "./useEditing";

interface Save {
  body: Record<string, unknown>;
  answer(): void;
}

// fakeMoves stands for POST /api/tickets/<id>/move: each save waits until
// the test answers it.
function fakeMoves() {
  const saves: Save[] = [];
  const fetcher: AuthenticatedFetch = (_input, init) =>
    new Promise<Response>((resolve) => {
      saves.push({
        body: JSON.parse(String(init?.body)),
        answer: () =>
          resolve(
            new Response(
              JSON.stringify({ hash: "h", revision: 1, warnings: [] }),
              { headers: { "Content-Type": "application/json" } },
            ),
          ),
      });
    });
  return { fetcher, saves };
}

const card: Movable = { id: "FH-2", title: "Second", column: "up-next" };

// settle lets queued saves, their responses and their toasts run.
const settle = () =>
  act(() => new Promise<void>((resolve) => setTimeout(resolve, 0)));

// EDIT-9 with Undo (FH-115): the Undo on screen is always the latest move's.
// Two quick steps of one card both read "Moved FH-2 in Up next.", so an
// earlier move's Undo left showing would put the card back where it was
// before both.
describe("useEditing Undo", () => {
  it("withdraws a move's Undo as soon as a later move starts", async () => {
    const { fetcher, saves } = fakeMoves();
    const { result } = renderHook(() => useEditing(fetcher, () => {}));

    act(() => {
      void result.current.move(card, "up-next", "", {
        after: "",
        undoAfter: "FH-1",
      });
    });
    await settle();
    saves[0]?.answer();
    await settle();
    expect(result.current.toast?.text).toBe("Moved FH-2 in Up next.");
    expect(result.current.toast?.action?.label).toBe("Undo");

    act(() => {
      void result.current.move(card, "up-next", "", {
        after: "FH-1",
        undoAfter: "",
      });
    });
    await settle();
    expect(result.current.toast).toBeNull();

    saves[1]?.answer();
    await settle();
    act(() => result.current.toast?.action?.run());
    await settle();
    expect(saves[2]?.body.after).toBe("");
  });

  it("offers no Undo for a move whose save returns after a later move started", async () => {
    const { fetcher, saves } = fakeMoves();
    const { result } = renderHook(() => useEditing(fetcher, () => {}));

    act(() => {
      void result.current.move(card, "up-next", "", {
        after: "",
        undoAfter: "FH-1",
      });
    });
    act(() => {
      void result.current.move(card, "up-next", "", {
        after: "FH-1",
        undoAfter: "",
      });
    });
    await settle();
    saves[0]?.answer();
    await settle();
    expect(result.current.toast?.text).toBe("Moved FH-2 in Up next.");
    expect(result.current.toast?.action).toBeUndefined();

    saves[1]?.answer();
    await settle();
    expect(result.current.toast?.action?.label).toBe("Undo");
    act(() => result.current.toast?.action?.run());
    await settle();
    expect(saves[2]?.body.after).toBe("");
  });
});
