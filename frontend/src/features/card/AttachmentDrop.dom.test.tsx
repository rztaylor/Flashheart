import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { Upload } from "../../model/uploads";
import { AttachmentDrop } from "./AttachmentDrop";

// The Attachments tab and the page's Files take dropped and chosen files
// (FH-18): each is uploaded in turn, a refusal says why, and the slot
// shows it will take files while they are dragged over.
describe("AttachmentDrop", () => {
  // files is a drag of files as a browser reports one (happy-dom's
  // DataTransfer does not list "Files" in its types).
  const files = (...names: string[]) => ({
    types: ["Files"],
    files: names.map((name) => new File(["x"], name)),
    dropEffect: "none",
  });

  it("uploads files dropped on it one at a time and says what was attached", async () => {
    const sent: string[] = [];
    const upload: Upload = vi.fn(async (_file, name) => {
      sent.push(name);
      if (name === "drawing.png")
        throw Object.assign(
          new Error("drawing.png was not attached because it is an SVG image."),
          { code: "type_not_allowed" },
        );
      return { file: `x-${name}`, kind: "other", image: false, reused: false };
    });
    const { container } = render(
      <AttachmentDrop upload={upload}>
        <p>The grid</p>
      </AttachmentDrop>,
    );
    const area = container.firstElementChild as HTMLElement;
    const slot = container.querySelector("[data-drop-slot]");
    expect(slot?.textContent).toContain("Drop files here to attach them");

    const data = files("a.log", "drawing.png", "b.pdf");
    fireEvent.dragEnter(area, { dataTransfer: data });
    expect(area.hasAttribute("data-dropping")).toBe(true);
    fireEvent.drop(screen.getByText("The grid"), { dataTransfer: data });
    expect(area.hasAttribute("data-dropping")).toBe(false);
    expect(screen.getByRole("status").textContent).toBe("Uploading 3 files…");
    await waitFor(() =>
      expect(screen.getByRole("status").textContent).toBe("Attached 2 files."),
    );
    expect(sent).toEqual(["a.log", "drawing.png", "b.pdf"]);
    expect(screen.getByRole("alert").textContent).toBe(
      "drawing.png was not attached because it is an SVG image.",
    );
  });

  it("ignores a drag that carries no files", () => {
    const upload = vi.fn<Upload>();
    const { container } = render(<AttachmentDrop upload={upload} />);
    const area = container.firstElementChild as HTMLElement;
    const text = { types: ["text/plain"], files: [], dropEffect: "none" };
    fireEvent.dragEnter(area, { dataTransfer: text });
    expect(area.hasAttribute("data-dropping")).toBe(false);
    fireEvent.drop(area, { dataTransfer: text });
    expect(upload).not.toHaveBeenCalled();
  });

  it("attaches files chosen with the keyboard through Attach files", async () => {
    const upload = vi.fn<Upload>(async (_file, name) => ({
      file: `x-${name}`,
      kind: "screenshot",
      image: true,
      reused: false,
    }));
    const { container } = render(<AttachmentDrop upload={upload} />);
    const user = userEvent.setup();
    const button = screen.getByRole("button", { name: "Attach files" });
    await user.tab();
    expect(document.activeElement).toBe(button);
    const input =
      container.querySelector<HTMLInputElement>('input[type="file"]');
    if (!input) throw new Error("no file input");
    const pick = vi.spyOn(input, "click");
    await user.keyboard("{Enter}");
    expect(pick).toHaveBeenCalled();
    await act(() =>
      user.upload(input, new File(["png"], "shot.png", { type: "image/png" })),
    );
    await waitFor(() =>
      expect(screen.getByRole("status").textContent).toBe("Attached shot.png."),
    );
    expect(upload).toHaveBeenCalledWith(
      expect.any(File),
      "shot.png",
      undefined,
    );
  });
});
