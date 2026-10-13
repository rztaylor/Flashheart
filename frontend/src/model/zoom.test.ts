import { describe, expect, it } from "vitest";

import {
  clampPan,
  fitView,
  fitZoom,
  isZoomedIn,
  maxZoom,
  minZoom,
  panBy,
  stepZoom,
  wheelFactor,
  zoomAt,
  zoomLimits,
  zoomPercent,
} from "./zoom";

// A 1200 by 700 screenshot on a 1000 by 500 stage fits at 500/700.
const image = { width: 1200, height: 700 };
const stage = { width: 1000, height: 500 };
const fit = 500 / 700;

describe("fitZoom", () => {
  it("shows the whole image, limited by its tighter side", () => {
    expect(fitZoom(image, stage)).toBeCloseTo(fit);
    expect(fitZoom({ width: 2000, height: 100 }, stage)).toBeCloseTo(0.5);
  });

  it("never enlarges an image past its natural size", () => {
    expect(fitZoom({ width: 200, height: 100 }, stage)).toBe(1);
  });

  it("is natural size while either size is unknown", () => {
    expect(fitZoom({ width: 0, height: 0 }, stage)).toBe(1);
    expect(fitZoom(image, { width: 0, height: 0 })).toBe(1);
  });
});

describe("zoomLimits", () => {
  it("is 25% to 800% of natural size", () => {
    expect(minZoom).toBe(0.25);
    expect(maxZoom).toBe(8);
    expect(zoomLimits(fit)).toEqual({ min: 0.25, max: 8 });
  });

  it("reaches down to Fit for an image too large to fit at 25%", () => {
    expect(zoomLimits(0.1)).toEqual({ min: 0.1, max: 8 });
  });
});

describe("fitView", () => {
  it("is centred at the fitting zoom", () => {
    expect(fitView(image, stage)).toEqual({
      zoom: fitZoom(image, stage),
      x: 0,
      y: 0,
    });
  });
});

describe("clampPan", () => {
  it("centres an axis where the image is smaller than the stage", () => {
    expect(clampPan({ zoom: fit, x: 40, y: -30 }, image, stage)).toEqual({
      zoom: fit,
      x: 0,
      y: 0,
    });
  });

  it("keeps the image's edges at or beyond the stage's edges", () => {
    // At 200% the image is 2400 by 1400: 700 and 450 to spare each side.
    expect(clampPan({ zoom: 2, x: 5000, y: -5000 }, image, stage)).toEqual({
      zoom: 2,
      x: 700,
      y: -450,
    });
    expect(clampPan({ zoom: 2, x: -100, y: 120 }, image, stage)).toEqual({
      zoom: 2,
      x: -100,
      y: 120,
    });
  });

  it("clamps the zoom to the limits", () => {
    expect(clampPan({ zoom: 20, x: 0, y: 0 }, image, stage).zoom).toBe(8);
    expect(clampPan({ zoom: 0.01, x: 0, y: 0 }, image, stage).zoom).toBe(0.25);
  });
});

describe("zoomAt", () => {
  it("keeps the image point under the pointer where it was", () => {
    const from = { zoom: 1, x: -50, y: 20 };
    const point = { x: 120, y: -80 };
    const to = zoomAt(from, 3, point, image, stage);
    expect(to.zoom).toBe(3);
    // The image point under the pointer, in natural pixels from its centre.
    const before = (point.x - from.x) / from.zoom;
    const after = (point.x - to.x) / to.zoom;
    expect(after).toBeCloseTo(before);
    expect((point.y - to.y) / to.zoom).toBeCloseTo(
      (point.y - from.y) / from.zoom,
    );
  });

  it("zooms around the centre when no point is given", () => {
    expect(
      zoomAt({ zoom: 1, x: 100, y: 0 }, 2, undefined, image, stage),
    ).toEqual({ zoom: 2, x: 200, y: 0 });
  });

  it("stops at the limits and keeps the image in bounds", () => {
    expect(
      zoomAt({ zoom: 4, x: 0, y: 0 }, 40, { x: 0, y: 0 }, image, stage).zoom,
    ).toBe(8);
    const out = zoomAt(
      { zoom: 2, x: 600, y: 400 },
      0.1,
      { x: 0, y: 0 },
      image,
      stage,
    );
    expect(out).toEqual({ zoom: 0.25, x: 0, y: 0 });
  });
});

describe("panBy", () => {
  it("moves the image and stops at its edges", () => {
    const view = { zoom: 2, x: 0, y: 0 };
    expect(panBy(view, -80, 30, image, stage)).toEqual({
      zoom: 2,
      x: -80,
      y: 30,
    });
    expect(panBy(view, -9000, 9000, image, stage)).toEqual({
      zoom: 2,
      x: -700,
      y: 450,
    });
  });

  it("does nothing at Fit", () => {
    const view = fitView(image, stage);
    expect(panBy(view, 80, 80, image, stage)).toEqual(view);
  });
});

describe("stepZoom", () => {
  const limits = zoomLimits(fit);

  it("steps up and down a ladder that includes Fit", () => {
    expect(stepZoom(1, 1, fit, limits)).toBe(1.25);
    expect(stepZoom(1, -1, fit, limits)).toBe(0.75);
    expect(stepZoom(0.75, -1, fit, limits)).toBeCloseTo(fit);
    expect(stepZoom(fit, -1, fit, limits)).toBe(0.67);
    expect(stepZoom(fit, 1, fit, limits)).toBe(0.75);
    expect(stepZoom(1.1, 1, fit, limits)).toBe(1.25);
    expect(stepZoom(1.1, -1, fit, limits)).toBe(1);
  });

  it("stops at 800% and 25%", () => {
    expect(stepZoom(6, 1, fit, limits)).toBe(8);
    expect(stepZoom(8, 1, fit, limits)).toBe(8);
    expect(stepZoom(0.33, -1, fit, limits)).toBe(0.25);
    expect(stepZoom(0.25, -1, fit, limits)).toBe(0.25);
  });

  it("reaches Fit below 25% for a very large image", () => {
    const small = zoomLimits(0.1);
    expect(stepZoom(0.25, -1, 0.1, small)).toBe(0.1);
    expect(stepZoom(0.1, 1, 0.1, small)).toBe(0.25);
  });
});

describe("isZoomedIn", () => {
  it("is true only past Fit", () => {
    expect(isZoomedIn(fitView(image, stage), image, stage)).toBe(false);
    expect(isZoomedIn({ zoom: 0.25, x: 0, y: 0 }, image, stage)).toBe(false);
    expect(isZoomedIn({ zoom: 1, x: 0, y: 0 }, image, stage)).toBe(true);
  });
});

describe("wheelFactor", () => {
  it("zooms in on a wheel turned up and out on one turned down", () => {
    expect(wheelFactor(-100, 0, false)).toBeGreaterThan(1);
    expect(wheelFactor(100, 0, false)).toBeLessThan(1);
    expect(
      wheelFactor(100, 0, false) * wheelFactor(-100, 0, false),
    ).toBeCloseTo(1);
  });

  it("treats lines and pages as pixels and a pinch more strongly", () => {
    expect(wheelFactor(3, 1, false)).toBeCloseTo(wheelFactor(48, 0, false));
    expect(wheelFactor(-10, 0, true)).toBeGreaterThan(
      wheelFactor(-10, 0, false),
    );
  });

  it("limits one notch to a reasonable step", () => {
    expect(wheelFactor(-10000, 0, false)).toBeLessThanOrEqual(2);
    expect(wheelFactor(10000, 0, false)).toBeGreaterThanOrEqual(0.5);
  });
});

describe("zoomPercent", () => {
  it("rounds to a whole percentage", () => {
    expect(zoomPercent(1)).toBe("100%");
    expect(zoomPercent(fit)).toBe("71%");
    expect(zoomPercent(8)).toBe("800%");
    expect(zoomPercent(0.25)).toBe("25%");
  });
});
