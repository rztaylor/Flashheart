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

    act(() => result.current.overwrite());
    expect(result.current.conflict).toBeNull();
    expect(sends[1]?.base).toBe("h9");
    sends[1]?.reject(conflict("h10", "newer"));
    await settle();
    expect(result.current.conflict).toEqual({
      mine: "mine",
      current: { hash: "h10", content: "newer" },
    });

    act(() => result.current.overwrite());
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
