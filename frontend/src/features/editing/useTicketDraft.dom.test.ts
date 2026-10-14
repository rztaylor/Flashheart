import { act, renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ApiError } from "../../api/client";
import type { Saved } from "../../api/edit";
import { useTicketDraft } from "./useTicketDraft";

// Doc stands for a ticket: a version (hash) and the text a draft edits.
interface Doc {
  hash: string;
  text: string;
}

const read = (doc: Doc) => doc.text;
const isDirty = (draft: string, base: string) => draft !== base;

interface Send {
  base: string;
  resolve(saved: Saved): void;
  reject(error: unknown): void;
}

// fakeSaves stands for a ticket write: each send records the base hash it
// was given and waits until the test answers it.
function fakeSaves() {
  const sends: Send[] = [];
  const send = (base: string) =>
    new Promise<Saved>((resolve, reject) => {
      sends.push({ base, resolve, reject });
    });
  return { sends, send };
}

const savedAs = (hash: string): Saved => ({ hash, revision: 2, warnings: [] });

const conflict = (hash: string, content: string) =>
  new ApiError("changed", 409, "conflict", { current: { hash, content } });

// settle lets a save's response and its state updates run.
const settle = () =>
  act(() => new Promise<void>((resolve) => setTimeout(resolve, 0)));

function draftOf(initial: Doc) {
  const onSaved = vi.fn();
  const view = renderHook(
    ({ detail }: { detail: Doc }) =>
      useTicketDraft({ detail, read, isDirty, onSaved }),
    { initialProps: { detail: initial } },
  );
  return { ...view, onSaved };
}

const one: Doc = { hash: "h1", text: "one" };

describe("useTicketDraft following the file", () => {
  it("follows the file while nothing is edited", () => {
    const { result, rerender } = draftOf(one);
    rerender({ detail: { hash: "h2", text: "two" } });
    expect(result.current.base).toEqual({ hash: "h2", value: "two" });
    expect(result.current.value).toBe("two");
    expect(result.current.dirty).toBe(false);
  });

  it("keeps an edit, and the version it started from, when the file changes", () => {
    const { result, rerender } = draftOf(one);
    act(() => result.current.set("mine"));
    rerender({ detail: { hash: "h2", text: "two" } });
    expect(result.current.base).toEqual({ hash: "h1", value: "one" });
    expect(result.current.value).toBe("mine");
    expect(result.current.dirty).toBe(true);
  });
});

describe("useTicketDraft editing part of a file (FH-66)", () => {
  // A section draft reads only its section: another part of the file may
  // change under the edit without touching what the draft edits.
  interface File {
    hash: string;
    text: string;
    rest: string;
  }
  const part = (initial: File) =>
    renderHook(
      ({ detail }: { detail: File }) =>
        useTicketDraft({
          detail,
          read: (file: File) => file.text,
          isDirty,
          onSaved: vi.fn(),
        }),
      { initialProps: { detail: initial } },
    );

  it("moves an edit's base to a new version that left its part as it was", async () => {
    const { sends, send } = fakeSaves();
    const { result, rerender } = part({ hash: "h1", text: "one", rest: "a" });
    act(() => result.current.set("mine"));
    rerender({ detail: { hash: "h2", text: "one", rest: "b" } });
    expect(result.current.base).toEqual({ hash: "h2", value: "one" });
    expect(result.current.value).toBe("mine");
    act(() => {
      void result.current.save({ send, what: "Saved", mine: "mine" });
    });
    expect(sends[0]?.base).toBe("h2");
  });

  it("keeps the base when its own part changed on disk, so the save conflicts", () => {
    const { result, rerender } = part({ hash: "h1", text: "one", rest: "a" });
    act(() => result.current.set("mine"));
    rerender({ detail: { hash: "h2", text: "theirs", rest: "a" } });
    expect(result.current.base).toEqual({ hash: "h1", value: "one" });
    expect(result.current.value).toBe("mine");
  });

  // A save that crosses an agent's write it had not heard of yet: the file
  // on disk ("part|rest") left the draft's part as it was, so the draft is
  // saved again on that version without asking (FH-16); a change to the
  // part itself still asks.
  const partOfFile = (content: string) => content.split("|")[0] ?? "";
  const racing = (initial: File) =>
    renderHook(
      ({ detail }: { detail: File }) =>
        useTicketDraft({
          detail,
          read: (file: File) => file.text,
          readFile: partOfFile,
          isDirty,
          onSaved: vi.fn(),
        }),
      { initialProps: { detail: initial } },
    );

  it("saves again on a newer version that left its part as it was", async () => {
    const { sends, send } = fakeSaves();
    const { result } = racing({ hash: "h1", text: "one", rest: "a" });
    act(() => result.current.set("mine"));
    let outcome = "";
    act(() => {
      void result.current
        .save({ send, what: "Saved", mine: "mine" })
        .then((kind) => {
          outcome = kind;
        });
    });
    sends[0]?.reject(conflict("h2", "one|b"));
    await settle();
    expect(sends[1]?.base).toBe("h2");
    sends[1]?.resolve(savedAs("h3"));
    await settle();
    expect(outcome).toBe("saved");
    expect(result.current.conflict).toBeNull();
  });

  it("asks when the newer version changed its part, and retries only once", async () => {
    const { sends, send } = fakeSaves();
    const { result } = racing({ hash: "h1", text: "one", rest: "a" });
    act(() => result.current.set("mine"));
    act(() => {
      void result.current.save({ send, what: "Saved", mine: "mine" });
    });
    sends[0]?.reject(conflict("h2", "theirs|a"));
    await settle();
    expect(sends).toHaveLength(1);
    expect(result.current.conflict?.current.hash).toBe("h2");

    act(() => result.current.cancel());
    act(() => {
      void result.current.save({ send, what: "Saved", mine: "mine" });
    });
    sends[1]?.reject(conflict("h3", "one|b"));
    await settle();
    sends[2]?.reject(conflict("h4", "one|c"));
    await settle();
    expect(sends).toHaveLength(3);
    expect(result.current.conflict?.current.hash).toBe("h4");
  });

  it("tells the caller how a save ended", async () => {
    const { sends, send } = fakeSaves();
    const { result } = part({ hash: "h1", text: "one", rest: "a" });
    act(() => result.current.set("mine"));
    const outcomes: string[] = [];
    act(() => {
      void result.current
        .save({ send, what: "Saved", mine: "mine" })
        .then((kind) => outcomes.push(kind));
    });
    sends[0]?.reject(new Error("disk full"));
    await settle();
    act(() => {
      void result.current
        .save({ send, what: "Saved", mine: "mine" })
        .then((kind) => outcomes.push(kind));
    });
    sends[1]?.reject(conflict("h9", "theirs"));
    await settle();
    act(() => {
      void result.current.overwrite().then((kind) => outcomes.push(kind));
    });
    sends[2]?.resolve(savedAs("h10"));
    await settle();
    expect(outcomes).toEqual(["failed", "conflict", "saved"]);
  });
});

describe("useTicketDraft saving", () => {
  it("saves on the base hash and adopts the saved version when it arrives", async () => {
    const { sends, send } = fakeSaves();
    const { result, rerender, onSaved } = draftOf(one);
    act(() => result.current.set("mine"));
    act(() => {
      void result.current.save({ send, what: "Saved", mine: "mine" });
    });
    expect(result.current.busy).toBe(true);
    expect(sends.map((item) => item.base)).toEqual(["h1"]);

    sends[0]?.resolve(savedAs("h2"));
    await settle();
    expect(result.current.busy).toBe(false);
    expect(onSaved).toHaveBeenCalledWith(savedAs("h2"), "Saved");
    // The saved version has not arrived yet; the draft waits for it.
    expect(result.current.base.hash).toBe("h1");

    // It arrives as written on disk (here normalised), and is taken even
    // though the draft differs from the old base.
    rerender({ detail: { hash: "h2", text: "mine, saved" } });
    expect(result.current.base).toEqual({ hash: "h2", value: "mine, saved" });
    expect(result.current.value).toBe("mine, saved");
    expect(result.current.dirty).toBe(false);
  });

  it("adopts at once when a live update brought the saved version first", async () => {
    const { sends, send } = fakeSaves();
    const { result, rerender } = draftOf(one);
    act(() => result.current.set("mine"));
    act(() => {
      void result.current.save({ send, what: "Saved", mine: "mine" });
    });
    rerender({ detail: { hash: "h2", text: "mine" } });
    expect(result.current.base.hash).toBe("h1");

    sends[0]?.resolve(savedAs("h2"));
    await settle();
    expect(result.current.base).toEqual({ hash: "h2", value: "mine" });
    expect(result.current.dirty).toBe(false);
  });

  it("says why a save failed and keeps the draft", async () => {
    const { sends, send } = fakeSaves();
    const { result, onSaved } = draftOf(one);
    act(() => result.current.set("mine"));
    act(() => {
      void result.current.save({ send, what: "Saved", mine: "mine" });
    });
    sends[0]?.reject(new Error("The board is read-only"));
    await settle();
    expect(result.current.error).toBe("The board is read-only");
    expect(result.current.busy).toBe(false);
    expect(result.current.value).toBe("mine");
    expect(result.current.conflict).toBeNull();
    expect(onSaved).not.toHaveBeenCalled();

    act(() => {
      void result.current.save({ send, what: "Saved", mine: "mine" });
    });
    expect(result.current.error).toBe("");
  });
});

// EDIT-7: a save that lost a race becomes a pending choice, never a silent
// overwrite.
describe("useTicketDraft conflicts", () => {
  it("turns a conflict into a pending choice with both versions", async () => {
    const { sends, send } = fakeSaves();
    const { result, onSaved } = draftOf(one);
    act(() => result.current.set("mine"));
    act(() => {
      void result.current.save({ send, what: "Saved", mine: "title: mine" });
    });
    sends[0]?.reject(conflict("h9", "theirs"));
    await settle();
    expect(result.current.conflict).toEqual({
      mine: "title: mine",
      current: { hash: "h9", content: "theirs" },
    });
    expect(result.current.error).toBe("");
    expect(result.current.busy).toBe(false);
    expect(result.current.value).toBe("mine");
    expect(onSaved).not.toHaveBeenCalled();
  });

  it("overwrites on the current file's hash, staying pending if it changes again", async () => {
    const { sends, send } = fakeSaves();
    const { result, onSaved } = draftOf(one);
    act(() => result.current.set("mine"));
    act(() => {
      void result.current.save({ send, what: "Saved", mine: "mine" });
    });
    sends[0]?.reject(conflict("h9", "theirs"));
    await settle();

    act(() => {
      void result.current.overwrite();
    });
    expect(result.current.conflict).toBeNull();
    expect(sends[1]?.base).toBe("h9");
    sends[1]?.reject(conflict("h10", "newer"));
    await settle();
    expect(result.current.conflict).toEqual({
      mine: "mine",
      current: { hash: "h10", content: "newer" },
    });

    act(() => {
      void result.current.overwrite();
    });
    expect(sends[2]?.base).toBe("h10");
    sends[2]?.resolve(savedAs("h11"));
    await settle();
    expect(result.current.conflict).toBeNull();
    expect(onSaved).toHaveBeenCalledWith(savedAs("h11"), "Saved");
  });

  it("reloads by discarding the draft for the file now and the next version", async () => {
    const { sends, send } = fakeSaves();
    const { result, rerender, onSaved } = draftOf(one);
    act(() => result.current.set("mine"));
    act(() => {
      void result.current.save({ send, what: "Saved", mine: "mine" });
    });
    rerender({ detail: { hash: "h9", text: "theirs" } });
    sends[0]?.reject(conflict("h9", "theirs"));
    await settle();

    act(() => result.current.reload());
    expect(result.current.conflict).toBeNull();
    expect(result.current.base).toEqual({ hash: "h9", value: "theirs" });
    expect(result.current.value).toBe("theirs");
    expect(onSaved).toHaveBeenCalledWith(undefined, "Reloaded");

    // The reload may bring a newer version still; it is taken even if the
    // draft was touched meanwhile.
    act(() => result.current.set("typing"));
    rerender({ detail: { hash: "h10", text: "newer" } });
    expect(result.current.base).toEqual({ hash: "h10", value: "newer" });
    expect(result.current.value).toBe("newer");
  });

  it("cancels the choice and keeps the draft", async () => {
    const { sends, send } = fakeSaves();
    const { result } = draftOf(one);
    act(() => result.current.set("mine"));
    act(() => {
      void result.current.save({ send, what: "Saved", mine: "mine" });
    });
    sends[0]?.reject(conflict("h9", "theirs"));
    await settle();
    act(() => result.current.cancel());
    expect(result.current.conflict).toBeNull();
    expect(result.current.value).toBe("mine");
    expect(result.current.base.hash).toBe("h1");
  });
});

describe("useTicketDraft answered after it was left (FH-156)", () => {
  function leftDraft() {
    const onSaved = vi.fn();
    const left = vi.fn();
    const view = renderHook(() =>
      useTicketDraft({ detail: one, read, isDirty, onSaved, left }),
    );
    return { ...view, onSaved, left };
  }

  it("hands a failure that comes after unmounting to left, as the draft was", async () => {
    const { sends, send } = fakeSaves();
    const { result, unmount, left } = leftDraft();
    act(() => result.current.set("mine"));
    act(() => {
      void result.current.save({ send, what: "Saved", mine: "mine" });
    });
    unmount();
    sends[0]?.reject(new ApiError("The project is busy", 503, "busy"));
    await settle();
    expect(left).toHaveBeenCalledWith({
      base: { hash: "h1", value: "one" },
      value: "mine",
      error: "The project is busy",
      conflict: null,
    });
  });

  it("hands a conflict to left with its pending choice, and a success to onSaved", async () => {
    const { sends, send } = fakeSaves();
    const first = leftDraft();
    act(() => first.result.current.set("mine"));
    act(() => {
      void first.result.current.save({ send, what: "Saved", mine: "mine" });
    });
    first.unmount();
    sends[0]?.reject(conflict("h9", "theirs"));
    await settle();
    expect(first.left.mock.calls[0]?.[0]).toMatchObject({
      value: "mine",
      error: "",
      conflict: { mine: "mine", current: { hash: "h9", content: "theirs" } },
    });

    const second = leftDraft();
    act(() => second.result.current.set("mine"));
    act(() => {
      void second.result.current.save({ send, what: "Saved", mine: "mine" });
    });
    second.unmount();
    sends[1]?.resolve(savedAs("h2"));
    await settle();
    expect(second.left).not.toHaveBeenCalled();
    expect(second.onSaved).toHaveBeenCalledWith(savedAs("h2"), "Saved");
  });

  it("starts as a left draft and restores one", () => {
    const left = {
      base: { hash: "h1", value: "one" },
      value: "mine",
      error: "The project is busy",
      conflict: null,
    };
    const view = renderHook(() =>
      useTicketDraft({
        detail: one,
        read,
        isDirty,
        onSaved: vi.fn(),
        initial: left,
      }),
    );
    expect(view.result.current.value).toBe("mine");
    expect(view.result.current.error).toBe("The project is busy");
    expect(view.result.current.dirty).toBe(true);

    const other = draftOf(one);
    act(() => other.result.current.restore(left));
    expect(other.result.current.value).toBe("mine");
    expect(other.result.current.base).toEqual({ hash: "h1", value: "one" });
    expect(other.result.current.error).toBe("The project is busy");
  });
});
