import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import {
  focusHeading,
  focusItem,
  focusList,
  useKeepFocus,
} from "./useKeepFocus";

type Lists = Record<string, string[]>;

// Board is a container of titled lists whose items each hold a link and a
// button, like a board's columns of cards, and may hold a titled list of
// parts first, like a session's subagents on the Runs tab.
function Board({ lists, nested }: { lists: Lists; nested?: Lists }) {
  const keepFocus = useKeepFocus();
  return (
    <>
      <div ref={keepFocus}>
        {Object.entries(lists).map(([list, items]) => (
          <section key={list} {...focusList(list)} aria-label={list}>
            <h2 {...focusHeading} tabIndex={-1}>
              {list}
            </h2>
            <ul>
              {items.map((item) => (
                <li key={item} {...focusItem(item)}>
                  {nested?.[item] ? (
                    <div {...focusList(`${item} parts`)}>
                      <h3 {...focusHeading} tabIndex={-1}>
                        {`${item} parts`}
                      </h3>
                      <ul>
                        {nested[item].map((part) => (
                          <li key={part} {...focusItem(part)}>
                            <button type="button">{part}</button>
                          </li>
                        ))}
                      </ul>
                    </div>
                  ) : null}
                  <a href={`#${item}`}>{`${item} link`}</a>
                  <button type="button">{item}</button>
                </li>
              ))}
            </ul>
          </section>
        ))}
      </div>
      <button type="button">Outside</button>
    </>
  );
}

const button = (name: string) => screen.getByRole("button", { name });
const focused = () => document.activeElement;

// useKeepFocus keeps keyboard focus on a moved item and hands it to the
// item's neighbour when it disappears (FH-84, ui-layout.md §8).
describe("useKeepFocus", () => {
  it("follows a focused item that moves to another list", async () => {
    const { rerender } = render(
      <Board lists={{ todo: ["a", "b"], done: ["c"] }} />,
    );
    button("a").focus();
    rerender(<Board lists={{ todo: ["b"], done: ["a", "c"] }} />);
    await waitFor(() => expect(focused()).toBe(button("a")));
    expect(button("a").closest("section")?.getAttribute("aria-label")).toBe(
      "done",
    );
  });

  it("keeps a focused item that moves within its list", async () => {
    const { rerender } = render(<Board lists={{ todo: ["a", "b", "c"] }} />);
    button("a").focus();
    rerender(<Board lists={{ todo: ["b", "c", "a"] }} />);
    await waitFor(() => expect(focused()).toBe(button("a")));
  });

  it("hands focus to the next item when the focused one disappears", async () => {
    const { rerender } = render(<Board lists={{ todo: ["a", "b", "c"] }} />);
    button("b").focus();
    rerender(<Board lists={{ todo: ["a", "c"] }} />);
    await waitFor(() => expect(focused()).toBe(button("c")));
  });

  it("hands focus to the previous item when no item follows", async () => {
    const { rerender } = render(<Board lists={{ todo: ["a", "b", "c"] }} />);
    button("c").focus();
    rerender(<Board lists={{ todo: ["a", "b"] }} />);
    await waitFor(() => expect(focused()).toBe(button("b")));
  });

  it("hands focus to the list's heading when the list empties", async () => {
    const { rerender } = render(<Board lists={{ todo: ["a"], done: ["b"] }} />);
    button("a").focus();
    rerender(<Board lists={{ todo: [], done: ["b"] }} />);
    await waitFor(() =>
      expect(focused()).toBe(screen.getByRole("heading", { name: "todo" })),
    );
  });

  it("keeps to the same control of the item, or its neighbour", async () => {
    const { rerender } = render(<Board lists={{ todo: ["a", "b", "c"] }} />);
    screen.getByRole("link", { name: "b link" }).focus();
    rerender(<Board lists={{ todo: ["a", "c"] }} />);
    await waitFor(() =>
      expect(focused()).toBe(screen.getByRole("link", { name: "c link" })),
    );
  });

  it("moves to the enclosing item's neighbour when a nested list goes", async () => {
    const { rerender } = render(
      <Board
        lists={{ runs: ["s1", "s2"] }}
        nested={{ s1: ["x"], s2: ["y"] }}
      />,
    );
    button("x").focus();
    rerender(<Board lists={{ runs: ["s2"] }} nested={{ s2: ["y"] }} />);
    // s2's first control: its nested list's heading is never one.
    await waitFor(() => expect(focused()).toBe(button("y")));
  });

  it("does not take focus from outside the lists", async () => {
    const { rerender } = render(<Board lists={{ todo: ["a", "b"] }} />);
    button("Outside").focus();
    rerender(<Board lists={{ todo: ["b"] }} />);
    await new Promise((resolve) => setTimeout(resolve, 10));
    expect(focused()).toBe(button("Outside"));
  });

  it("does not follow an item after focus has left for elsewhere", async () => {
    const { rerender } = render(<Board lists={{ todo: ["a", "b"] }} />);
    button("a").focus();
    button("Outside").focus();
    rerender(<Board lists={{ todo: ["b"] }} />);
    await new Promise((resolve) => setTimeout(resolve, 10));
    expect(focused()).toBe(button("Outside"));
  });

  it("does not take focus back after it was let go", async () => {
    const { rerender } = render(<Board lists={{ todo: ["a", "b"] }} />);
    button("a").focus();
    button("a").blur();
    await new Promise((resolve) => setTimeout(resolve, 10));
    rerender(<Board lists={{ todo: ["b"] }} />);
    await new Promise((resolve) => setTimeout(resolve, 10));
    expect(focused()).toBe(document.body);
  });

  it("moves focus without jumping the page, then brings it into view", async () => {
    const focus = vi.spyOn(HTMLElement.prototype, "focus");
    const scroll = vi.fn();
    const scrollIntoView = HTMLElement.prototype.scrollIntoView;
    HTMLElement.prototype.scrollIntoView = scroll;
    try {
      const { rerender } = render(<Board lists={{ todo: ["a", "b"] }} />);
      button("a").focus();
      focus.mockClear();
      rerender(<Board lists={{ todo: ["b"] }} />);
      await waitFor(() => expect(focused()).toBe(button("b")));
      expect(focus).toHaveBeenCalledWith({ preventScroll: true });
      expect(scroll).toHaveBeenCalledWith({
        block: "nearest",
        inline: "nearest",
      });
    } finally {
      focus.mockRestore();
      HTMLElement.prototype.scrollIntoView = scrollIntoView;
    }
  });
});
