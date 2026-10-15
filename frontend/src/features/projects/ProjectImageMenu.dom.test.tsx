import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { ProjectImageMenu } from "./ProjectImageMenu";

function setup({
  image,
  onSet = vi.fn(async () => {}),
  onClear = vi.fn(async () => {}),
}: {
  image?: string;
  onSet?: (file: File) => Promise<void>;
  onClear?: () => Promise<void>;
} = {}) {
  const user = userEvent.setup();
  const onDone = vi.fn();
  const { container } = render(
    <>
      <ProjectImageMenu
        project={{ displayName: "Flashheart", key: "FH", image }}
        onSet={onSet}
        onClear={onClear}
        onDone={onDone}
      />
      <button type="button">Elsewhere</button>
    </>,
  );
  const badge = screen.getByRole("button", { name: "Project image: FH" });
  const input = () =>
    container.querySelector<HTMLInputElement>('input[type="file"]');
  return { user, badge, input, onSet, onClear, onDone };
}

const png = () => new File(["\x89PNG"], "logo.png", { type: "image/png" });

// The page header's badge opens the project's image menu (FH-151).
describe("ProjectImageMenu", () => {
  it("is the key badge as a button, named with the key, closed", () => {
    const { badge } = setup();
    expect(badge.getAttribute("aria-expanded")).toBe("false");
    expect(badge.getAttribute("title")).toBe("Project image: FH");
    expect(badge.textContent).toBe("FH");
    expect(screen.queryByRole("button", { name: /Choose image/ })).toBeNull();
  });

  it("shows the project's image on the badge when it has one", () => {
    const { badge } = setup({ image: "/api/projects/flashheart/image?v=1" });
    const img = badge.querySelector("img");
    expect(img?.getAttribute("src")).toBe("/api/projects/flashheart/image?v=1");
    expect(img?.getAttribute("alt")).toBe("");
    expect(badge.textContent).toBe("");
  });

  it("offers Choose image for a project without one, and sends the chosen file", async () => {
    const onSet = vi.fn(async () => {});
    const { user, badge, input, onDone } = setup({ onSet });
    await user.click(badge);
    expect(badge.getAttribute("aria-expanded")).toBe("true");
    expect(screen.getByText(/Choose a JPEG or PNG/)).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Remove image" })).toBeNull();
    const choose = screen.getByRole("button", { name: "Choose image…" });
    const file = png();
    expect(input()?.getAttribute("accept")).toBe("image/png,image/jpeg");
    await user.click(choose);
    const picker = input();
    if (!picker) throw new Error("no file input");
    await user.upload(picker, file);
    await waitFor(() => expect(onSet).toHaveBeenCalledWith(file));
    await waitFor(() =>
      expect(onDone).toHaveBeenCalledWith("Project image updated."),
    );
    // Saved: the menu closes and focus returns to the badge.
    expect(badge.getAttribute("aria-expanded")).toBe("false");
    expect(document.activeElement).toBe(badge);
  });

  it("offers Replace and Remove for a project with an image", async () => {
    const onClear = vi.fn(async () => {});
    const { user, badge, onDone } = setup({
      image: "/api/projects/flashheart/image?v=1",
      onClear,
    });
    await user.click(badge);
    expect(screen.getByRole("button", { name: "Replace image…" })).toBeTruthy();
    expect(screen.getByText(/its key is FH/)).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "Remove image" }));
    await waitFor(() => expect(onClear).toHaveBeenCalled());
    await waitFor(() =>
      expect(onDone).toHaveBeenCalledWith("Project image removed."),
    );
    expect(badge.getAttribute("aria-expanded")).toBe("false");
  });

  it("shows the server's refusal in the menu and leaves it open", async () => {
    const onSet = vi.fn(async () => {
      throw new Error(
        "The image was not saved because it is not a JPEG or PNG image. Choose a JPEG or PNG image.",
      );
    });
    const { user, badge, input, onDone } = setup({ onSet });
    await user.click(badge);
    const picker = input();
    if (!picker) throw new Error("no file input");
    // A text file renamed .png passes the picker's filter; the server
    // judges it by its bytes.
    await user.upload(
      picker,
      new File(["words"], "notes.png", { type: "image/png" }),
    );
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toBe(
      "The image was not saved because it is not a JPEG or PNG image. Choose a JPEG or PNG image.",
    );
    expect(badge.getAttribute("aria-expanded")).toBe("true");
    expect(onDone).not.toHaveBeenCalled();
    // The button is usable again for another try.
    expect(
      screen
        .getByRole("button", { name: "Choose image…" })
        .hasAttribute("disabled"),
    ).toBe(false);
  });

  it("closes on Escape with focus back on the badge", async () => {
    const { user, badge } = setup();
    await user.click(badge);
    await user.keyboard("{Escape}");
    expect(badge.getAttribute("aria-expanded")).toBe("false");
    expect(document.activeElement).toBe(badge);
  });
});
