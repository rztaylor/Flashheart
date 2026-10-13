import { createEvent, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { Lightbox, type LightboxItem } from "./Lightbox";

const items: LightboxItem[] = [
  { src: "/files/one.png", caption: "Board at 1440", name: "one.png" },
  { src: "/files/two.png", caption: "Board at 390", name: "two.png" },
];

// The stage measures 1000 by 500 at the page's origin; the screenshots are
// 1200 by 700, so Fit is 500/700, shown as 71%.
const stageRect = {
  x: 0,
  y: 0,
  left: 0,
  top: 0,
  right: 1000,
  bottom: 500,
  width: 1000,
  height: 500,
  toJSON: () => ({}),
};

beforeEach(() => {
  const original = HTMLElement.prototype.getBoundingClientRect;
  vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(
    function (this: HTMLElement) {
      if (this.hasAttribute("data-lightbox-stage")) return stageRect as DOMRect;
      return original.call(this);
    },
  );
});

afterEach(() => {
  vi.restoreAllMocks();
});

// The stage's inner size, which it measures with clientWidth and
// clientHeight; happy-dom lays nothing out.
for (const [property, size] of [
  ["clientWidth", 1000],
  ["clientHeight", 500],
] as const) {
  const original = Object.getOwnPropertyDescriptor(
    HTMLElement.prototype,
    property,
  );
  beforeEach(() => {
    Object.defineProperty(HTMLElement.prototype, property, {
      configurable: true,
      get(this: HTMLElement) {
        if (this.hasAttribute("data-lightbox-stage")) return size;
        return original?.get?.call(this) ?? 0;
      },
    });
  });
  afterEach(() => {
    if (original)
      Object.defineProperty(HTMLElement.prototype, property, original);
  });
}

function load(width = 1200, height = 700) {
  const image = screen.getByRole("img");
  Object.defineProperty(image, "naturalWidth", { value: width });
  Object.defineProperty(image, "naturalHeight", { value: height });
  fireEvent.load(image);
  return image;
}

function zoom(): string {
  return screen.getByRole("status").textContent ?? "";
}

function offset(): string {
  return (screen.getByRole("img") as HTMLImageElement).style.transform;
}

function stage(): HTMLElement {
  const element = document.querySelector<HTMLElement>("[data-lightbox-stage]");
  if (!element) throw new Error("no stage");
  return element;
}

// Opener opens the lightbox on the first image, as a thumbnail would.
function Opener({ onIndex }: { onIndex?: (index: number) => void }) {
  const [open, setOpen] = useState<number | null>(null);
  return (
    <>
      <button type="button" onClick={() => setOpen(0)}>
        Open Board at 1440
      </button>
      {open !== null ? (
        <Lightbox
          items={items}
          index={open}
          onIndex={(index) => {
            onIndex?.(index);
            setOpen(index);
          }}
          onClose={() => setOpen(null)}
        />
      ) : null}
    </>
  );
}

async function openLightbox(onIndex?: (index: number) => void) {
  const user = userEvent.setup();
  render(<Opener onIndex={onIndex} />);
  await user.click(screen.getByRole("button", { name: "Open Board at 1440" }));
  load();
  return user;
}

describe("Lightbox", () => {
  it("names its zoom controls and shows the zoom as a percentage", async () => {
    await openLightbox();
    const dialog = screen.getByRole("dialog", { name: "Board at 1440" });
    expect(dialog).toBeTruthy();
    expect(screen.getByRole("group", { name: "Zoom" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Zoom out" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Zoom in" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Fit" })).toBeTruthy();
    expect(zoom()).toBe("71%");
  });

  it("zooms with Cmd or Ctrl and plus or minus, keeping the browser's zoom out of it", async () => {
    await openLightbox();
    const close = screen.getByRole("button", { name: "Close" });
    expect(fireEvent.keyDown(close, { key: "=", ctrlKey: true })).toBe(false);
    expect(zoom()).toBe("75%");
    expect(fireEvent.keyDown(close, { key: "+", metaKey: true })).toBe(false);
    expect(zoom()).toBe("100%");
    expect(fireEvent.keyDown(close, { key: "-", metaKey: true })).toBe(false);
    expect(zoom()).toBe("75%");
    fireEvent.keyDown(close, { key: "-", ctrlKey: true });
    expect(zoom()).toBe("71%");
    fireEvent.keyDown(close, { key: "-" });
    expect(zoom()).toBe("67%");
  });

  it("fits again on 0 and Cmd or Ctrl and 0, and on Fit", async () => {
    const user = await openLightbox();
    const close = screen.getByRole("button", { name: "Close" });
    fireEvent.keyDown(close, { key: "=" });
    fireEvent.keyDown(close, { key: "=" });
    expect(zoom()).toBe("100%");
    expect(fireEvent.keyDown(close, { key: "0", ctrlKey: true })).toBe(false);
    expect(zoom()).toBe("71%");
    fireEvent.keyDown(close, { key: "=" });
    fireEvent.keyDown(close, { key: "0" });
    expect(zoom()).toBe("71%");
    await user.click(screen.getByRole("button", { name: "Zoom in" }));
    await user.click(screen.getByRole("button", { name: "Fit" }));
    expect(zoom()).toBe("71%");
  });

  it("stops at 800% and 25%, and says so on the buttons", async () => {
    const user = await openLightbox();
    const zoomIn = screen.getByRole("button", { name: "Zoom in" });
    const zoomOut = screen.getByRole("button", { name: "Zoom out" });
    for (let step = 0; step < 15; step++) await user.click(zoomIn);
    expect(zoom()).toBe("800%");
    expect(zoomIn.getAttribute("aria-disabled")).toBe("true");
    expect(document.activeElement).toBe(zoomIn);
    for (let step = 0; step < 20; step++) await user.click(zoomOut);
    expect(zoom()).toBe("25%");
    expect(zoomOut.getAttribute("aria-disabled")).toBe("true");
    expect(zoomIn.getAttribute("aria-disabled")).toBe("false");
  });

  it("steps between images with the arrows at Fit and pans with them when zoomed in", async () => {
    const onIndex = vi.fn();
    await openLightbox(onIndex);
    const close = screen.getByRole("button", { name: "Close" });
    fireEvent.keyDown(close, { key: "ArrowRight" });
    expect(onIndex).toHaveBeenLastCalledWith(1);
    expect(screen.getByRole("dialog", { name: "Board at 390" })).toBeTruthy();
    load();
    fireEvent.keyDown(close, { key: "ArrowLeft" });
    expect(onIndex).toHaveBeenLastCalledWith(0);
    load();
    onIndex.mockClear();

    // At 200% the image is 2400 by 1400, with 700 and 450 to spare.
    for (const _ of [1, 2, 3, 4, 5]) fireEvent.keyDown(close, { key: "=" });
    expect(zoom()).toBe("200%");
    expect(fireEvent.keyDown(close, { key: "ArrowRight" })).toBe(false);
    expect(offset()).toContain("translate(-80px, 0px)");
    fireEvent.keyDown(close, { key: "ArrowDown" });
    expect(offset()).toContain("translate(-80px, -80px)");
    fireEvent.keyDown(close, { key: "ArrowLeft", shiftKey: true });
    expect(offset()).toContain("translate(240px, -80px)");
    for (const _ of [1, 2, 3, 4, 5, 6, 7]) {
      fireEvent.keyDown(close, { key: "ArrowUp" });
    }
    expect(offset()).toContain("translate(240px, 450px)");
    expect(onIndex).not.toHaveBeenCalled();
  });

  it("opens the next image at Fit", async () => {
    const user = await openLightbox();
    await user.click(screen.getByRole("button", { name: "Zoom in" }));
    expect(zoom()).toBe("75%");
    await user.click(screen.getByRole("button", { name: "Next" }));
    load(600, 300);
    expect(zoom()).toBe("100%");
  });

  it("zooms with the wheel around the pointer", async () => {
    await openLightbox();
    const close = screen.getByRole("button", { name: "Close" });
    for (const _ of [1, 2, 3, 4, 5]) fireEvent.keyDown(close, { key: "=" });
    expect(zoom()).toBe("200%");
    // The pointer 200px right of the stage's centre, over the image point
    // 100 natural pixels right of the image's centre.
    const wheel = createEvent.wheel(stage(), { deltaY: -100 });
    // happy-dom's WheelEvent drops the pointer's position from its init.
    Object.defineProperty(wheel, "clientX", { value: 700 });
    Object.defineProperty(wheel, "clientY", { value: 250 });
    expect(fireEvent(stage(), wheel)).toBe(false);
    const image = screen.getByRole("img") as HTMLImageElement;
    const zoomed = Number.parseFloat(image.style.width) / 1200;
    expect(zoomed).toBeGreaterThan(2);
    const x = Number.parseFloat(
      /translate\((-?[\d.]+)px, (-?[\d.]+)px\)$/.exec(offset())?.[1] ?? "NaN",
    );
    expect((200 - x) / zoomed).toBeCloseTo(100);
    // A trackpad pinch (ctrlKey) and the wheel turned down zoom out.
    fireEvent.wheel(stage(), { deltaY: 40, ctrlKey: true });
    expect(Number.parseFloat(image.style.width) / 1200).toBeLessThan(zoomed);
    for (const _ of [1, 2, 3, 4, 5, 6, 7, 8]) {
      fireEvent.wheel(stage(), { deltaY: 400 });
    }
    expect(zoom()).toBe("25%");
  });

  it("pans by dragging", async () => {
    await openLightbox();
    const close = screen.getByRole("button", { name: "Close" });
    for (const _ of [1, 2, 3, 4, 5]) fireEvent.keyDown(close, { key: "=" });
    fireEvent.pointerDown(stage(), {
      pointerId: 1,
      button: 0,
      clientX: 500,
      clientY: 250,
    });
    fireEvent.pointerMove(stage(), {
      pointerId: 1,
      clientX: 450,
      clientY: 270,
    });
    fireEvent.pointerMove(stage(), {
      pointerId: 1,
      clientX: 400,
      clientY: 290,
    });
    fireEvent.pointerUp(stage(), { pointerId: 1, clientX: 400, clientY: 290 });
    expect(offset()).toContain("translate(-100px, 40px)");
    fireEvent.pointerMove(stage(), { pointerId: 1, clientX: 0, clientY: 0 });
    expect(offset()).toContain("translate(-100px, 40px)");
  });

  it("zooms with two fingers pinched apart", async () => {
    await openLightbox();
    const touch = { pointerType: "touch", clientY: 250 };
    fireEvent.pointerDown(stage(), { ...touch, pointerId: 1, clientX: 400 });
    fireEvent.pointerDown(stage(), { ...touch, pointerId: 2, clientX: 600 });
    // The fingers move from 200px apart to 400px: twice the zoom.
    fireEvent.pointerMove(stage(), { ...touch, pointerId: 2, clientX: 800 });
    fireEvent.pointerUp(stage(), { ...touch, pointerId: 2, clientX: 800 });
    fireEvent.pointerUp(stage(), { ...touch, pointerId: 1, clientX: 400 });
    expect(zoom()).toBe("143%");
  });

  it("leaves keys outside the dialog alone", async () => {
    await openLightbox();
    const outside = document.createElement("input");
    document.body.append(outside);
    expect(fireEvent.keyDown(outside, { key: "=", ctrlKey: true })).toBe(true);
    expect(zoom()).toBe("71%");
    outside.remove();
  });

  it("closes on Escape and returns focus to what opened it", async () => {
    const user = await openLightbox();
    screen.getByRole("button", { name: "Zoom in" }).focus();
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(document.activeElement).toBe(
      screen.getByRole("button", { name: "Open Board at 1440" }),
    );
  });
});
