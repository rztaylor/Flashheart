// Zoom and pan for the lightbox (FH-146, ui-layout.md §3). A view is the
// image's zoom, relative to its natural size (1 is one image pixel to one
// CSS pixel), and the offset of the image's centre from the stage's centre
// in CSS pixels. Fit shows the whole image and never enlarges it past its
// natural size; zoom runs from 25% to 800%, reaching down to Fit for an
// image too large to fit at 25%.

export interface Size {
  width: number;
  height: number;
}

export interface Point {
  x: number;
  y: number;
}

export interface View {
  zoom: number;
  x: number;
  y: number;
}

export interface ZoomLimits {
  min: number;
  max: number;
}

export const minZoom = 0.25;
export const maxZoom = 8;

// The zoom buttons and keys step along this ladder, with Fit added.
const ladder = [0.25, 0.33, 0.5, 0.67, 0.75, 1, 1.25, 1.5, 2, 3, 4, 6, 8];
const epsilon = 1e-3;

function known(size: Size): boolean {
  return size.width > 0 && size.height > 0;
}

// fitZoom is the zoom that shows the whole image on the stage, at most 1.
export function fitZoom(image: Size, stage: Size): number {
  if (!known(image) || !known(stage)) return 1;
  return Math.min(1, stage.width / image.width, stage.height / image.height);
}

export function zoomLimits(fit: number): ZoomLimits {
  return { min: Math.min(minZoom, fit), max: maxZoom };
}

export function fitView(image: Size, stage: Size): View {
  return { zoom: fitZoom(image, stage), x: 0, y: 0 };
}

function clamp(value: number, low: number, high: number): number {
  return Math.min(high, Math.max(low, value));
}

// spare is how far the image may move from the centre on one axis: none
// while it is smaller than the stage, otherwise until its edge meets the
// stage's edge.
function spare(image: number, stage: number, zoom: number): number {
  return Math.max(0, (image * zoom - stage) / 2);
}

// clampPan keeps a view's zoom within the limits and its image covering
// the stage on each axis where it is larger, centred where it is smaller.
export function clampPan(view: View, image: Size, stage: Size): View {
  const limits = zoomLimits(fitZoom(image, stage));
  const zoom = clamp(view.zoom, limits.min, limits.max);
  const sx = spare(image.width, stage.width, zoom);
  const sy = spare(image.height, stage.height, zoom);
  return {
    zoom,
    x: clamp(view.x, -sx, sx) || 0,
    y: clamp(view.y, -sy, sy) || 0,
  };
}

// zoomAt changes the zoom keeping the image point under point (relative to
// the stage's centre; the centre itself when undefined) where it is.
export function zoomAt(
  view: View,
  zoom: number,
  point: Point | undefined,
  image: Size,
  stage: Size,
): View {
  const limits = zoomLimits(fitZoom(image, stage));
  const next = clamp(zoom, limits.min, limits.max);
  const at = point ?? { x: 0, y: 0 };
  const ratio = next / view.zoom;
  return clampPan(
    {
      zoom: next,
      x: at.x - (at.x - view.x) * ratio,
      y: at.y - (at.y - view.y) * ratio,
    },
    image,
    stage,
  );
}

// panBy moves the image by dx and dy CSS pixels, as far as its edges allow.
export function panBy(
  view: View,
  dx: number,
  dy: number,
  image: Size,
  stage: Size,
): View {
  return clampPan({ ...view, x: view.x + dx, y: view.y + dy }, image, stage);
}

// stepZoom is the next zoom up (direction 1) or down (-1) the ladder, with
// Fit as one of its steps, stopping at the limits.
export function stepZoom(
  zoom: number,
  direction: 1 | -1,
  fit: number,
  limits: ZoomLimits,
): number {
  const steps = [
    ...ladder.filter((step) => Math.abs(step / fit - 1) > 0.04),
    fit,
    limits.min,
    limits.max,
  ]
    .filter((step) => step >= limits.min && step <= limits.max)
    .sort((a, b) => a - b);
  if (direction > 0) {
    return steps.find((step) => step > zoom * (1 + epsilon)) ?? limits.max;
  }
  return (
    steps.reverse().find((step) => step < zoom * (1 - epsilon)) ?? limits.min
  );
}

// isZoomedIn is true when the view is larger than Fit, so the arrow keys
// pan rather than step between images.
export function isZoomedIn(view: View, image: Size, stage: Size): boolean {
  return view.zoom > fitZoom(image, stage) * (1 + epsilon);
}

// wheelFactor is the zoom change for one wheel event: up zooms in, down
// out; lines (deltaMode 1) and pages (2) count as pixels, a trackpad pinch
// (a wheel event with ctrlKey) more strongly; one event is at most double
// or half.
export function wheelFactor(
  deltaY: number,
  deltaMode: number,
  pinch: boolean,
): number {
  const pixels = deltaY * (deltaMode === 1 ? 16 : deltaMode === 2 ? 400 : 1);
  return clamp(Math.exp(-pixels * (pinch ? 0.01 : 0.002)), 0.5, 2);
}

export function zoomPercent(zoom: number): string {
  return `${Math.round(zoom * 100)}%`;
}
