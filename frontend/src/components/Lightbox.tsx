import {
  type PointerEvent as ReactPointerEvent,
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";

import {
  clampPan,
  fitView,
  fitZoom,
  isZoomedIn,
  type Point,
  panBy,
  type Size,
  stepZoom,
  type View,
  wheelFactor,
  zoomAt,
  zoomLimits,
  zoomPercent,
} from "../model/zoom";
import { Button } from "./Button";
import { Dialog } from "./Dialog";
import { Icon } from "./Icon";

export interface LightboxItem {
  src: string;
  caption: string;
  name: string;
}

// An arrow key pans the image this far, four times as far with Shift.
const panStep = 80;

const arrows: Record<string, Point> = {
  ArrowLeft: { x: 1, y: 0 },
  ArrowRight: { x: -1, y: 0 },
  ArrowUp: { x: 0, y: 1 },
  ArrowDown: { x: 0, y: -1 },
};

// Shown is the image on view: its natural size once loaded, and its view,
// null while it is at Fit (so Fit follows the stage as it resizes).
interface Shown {
  src: string;
  natural: Size | null;
  view: View | null;
}

// Lightbox shows one image of a set in a dialog that fills the viewport
// less a small margin (ui-layout.md §3, FH-146): its caption, file name
// and place in the set, Previous and Next when there is more than one, and
// zoom from 25% to 800% of the image's natural size (model/zoom). Keys,
// while focus is in the dialog: Cmd or Ctrl (or nothing) with plus or
// minus zooms, 0 fits; the arrows pan while zoomed in past Fit and step
// between images at Fit. The wheel or a trackpad pinch zooms around the
// pointer, dragging pans, two fingers pinch. Escape closes it and focus
// returns to whatever opened it (Dialog). Nothing animates.
export function Lightbox({
  items,
  index,
  onIndex,
  onClose,
}: {
  items: LightboxItem[];
  index: number;
  onIndex(index: number): void;
  onClose(): void;
}) {
  const item = items[index];
  const last = items.length - 1;
  const stageRef = useRef<HTMLDivElement>(null);
  const [stage, setStage] = useState<Size>({ width: 0, height: 0 });
  const [shown, setShown] = useState<Shown>({
    src: item?.src ?? "",
    natural: null,
    view: null,
  });
  // A new image opens at Fit.
  if (item && shown.src !== item.src) {
    setShown({ src: item.src, natural: null, view: null });
  }
  const natural = shown.src === item?.src ? shown.natural : null;
  const ready =
    natural !== null &&
    natural.width > 0 &&
    stage.width > 0 &&
    stage.height > 0;
  const fit = ready ? fitZoom(natural, stage) : 1;
  const limits = zoomLimits(fit);
  const view: View | null = ready
    ? shown.view
      ? clampPan(shown.view, natural, stage)
      : fitView(natural, stage)
    : null;

  // The handlers below read the stage and the view through a ref, so the
  // listeners stay put while the view changes.
  const latest = useRef({ stage, view, natural });
  latest.current = { stage, view, natural };

  // change applies next to the image's current view, so several events
  // between two renders (a fast wheel, a held key) all count.
  const change = useCallback(
    (next: (view: View, image: Size, stage: Size) => View) => {
      setShown((current) => {
        const { stage } = latest.current;
        const image = current.natural;
        if (!image || image.width <= 0 || stage.width <= 0 || stage.height <= 0)
          return current;
        const view = current.view
          ? clampPan(current.view, image, stage)
          : fitView(image, stage);
        return {
          ...current,
          view: clampPan(next(view, image, stage), image, stage),
        };
      });
    },
    [],
  );
  const step = useCallback(
    (direction: 1 | -1) =>
      change((view, image, stage) => {
        const fit = fitZoom(image, stage);
        const zoom = stepZoom(view.zoom, direction, fit, zoomLimits(fit));
        return zoomAt(view, zoom, undefined, image, stage);
      }),
    [change],
  );
  const fitAgain = useCallback(
    () => setShown((current) => ({ ...current, view: null })),
    [],
  );

  // pointAt is a client position relative to the stage's centre.
  const pointAt = useCallback((clientX: number, clientY: number): Point => {
    const rect = stageRef.current?.getBoundingClientRect();
    if (!rect) return { x: 0, y: 0 };
    return {
      x: clientX - rect.left - rect.width / 2,
      y: clientY - rect.top - rect.height / 2,
    };
  }, []);

  // The stage's size, measured now and whenever it changes.
  useLayoutEffect(() => {
    const element = stageRef.current;
    if (!element) return;
    // Inside the stage's border, where the image is drawn.
    const measure = () => {
      const width = element.clientWidth;
      const height = element.clientHeight;
      setStage((current) =>
        current.width === width && current.height === height
          ? current
          : { width, height },
      );
    };
    measure();
    if (typeof ResizeObserver === "undefined") {
      window.addEventListener("resize", measure);
      return () => window.removeEventListener("resize", measure);
    }
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  // Keys while focus is in the dialog (or has fallen to the page's body,
  // which the modal dialog leaves inert).
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.defaultPrevented || event.altKey) return;
      const dialog = stageRef.current?.closest("dialog");
      const target = event.target;
      if (
        !dialog ||
        !(
          target === document.body ||
          (target instanceof Node && dialog.contains(target))
        )
      ) {
        return;
      }
      const { key } = event;
      if (key === "+" || key === "=") {
        event.preventDefault();
        step(1);
        return;
      }
      if (key === "-" || key === "_") {
        event.preventDefault();
        step(-1);
        return;
      }
      if (key === "0") {
        event.preventDefault();
        fitAgain();
        return;
      }
      const arrow = arrows[key];
      if (!arrow || event.metaKey || event.ctrlKey) return;
      const { natural, stage, view } = latest.current;
      if (natural && view && isZoomedIn(view, natural, stage)) {
        event.preventDefault();
        const distance = panStep * (event.shiftKey ? 4 : 1);
        change((view) => ({
          ...view,
          x: view.x + arrow.x * distance,
          y: view.y + arrow.y * distance,
        }));
        return;
      }
      if (key === "ArrowLeft" && index > 0) onIndex(index - 1);
      if (key === "ArrowRight" && index < last) onIndex(index + 1);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [change, fitAgain, index, last, onIndex, step]);

  // The wheel zooms around the pointer; a wheel event with ctrlKey is a
  // trackpad pinch. React's wheel listener is passive, so this is native.
  useEffect(() => {
    const element = stageRef.current;
    if (!element) return;
    const onWheel = (event: WheelEvent) => {
      event.preventDefault();
      const point = pointAt(event.clientX, event.clientY);
      const factor = wheelFactor(event.deltaY, event.deltaMode, event.ctrlKey);
      change((view, image, stage) =>
        zoomAt(view, view.zoom * factor, point, image, stage),
      );
    };
    element.addEventListener("wheel", onWheel, { passive: false });
    return () => element.removeEventListener("wheel", onWheel);
  }, [change, pointAt]);

  // Pointers down on the stage: one drags, two pinch.
  const pointers = useRef(new Map<number, Point>());
  const onPointerDown = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (event.pointerType === "mouse" && event.button !== 0) return;
    event.preventDefault();
    try {
      event.currentTarget.setPointerCapture(event.pointerId);
    } catch {
      // A synthetic pointer cannot be captured; dragging still works.
    }
    pointers.current.set(event.pointerId, {
      x: event.clientX,
      y: event.clientY,
    });
  };
  const onPointerMove = (event: ReactPointerEvent<HTMLDivElement>) => {
    const before = pointers.current.get(event.pointerId);
    if (!before) return;
    const now = { x: event.clientX, y: event.clientY };
    const others = [...pointers.current.entries()].filter(
      ([id]) => id !== event.pointerId,
    );
    pointers.current.set(event.pointerId, now);
    const other = others[0]?.[1];
    if (others.length === 1 && other) {
      const apart = (a: Point, b: Point) => Math.hypot(a.x - b.x, a.y - b.y);
      const ratio = apart(now, other) / Math.max(1, apart(before, other));
      const centre = pointAt((now.x + other.x) / 2, (now.y + other.y) / 2);
      change((view, image, stage) =>
        zoomAt(view, view.zoom * ratio, centre, image, stage),
      );
      return;
    }
    if (others.length > 0) return;
    change((view, image, stage) =>
      panBy(view, now.x - before.x, now.y - before.y, image, stage),
    );
  };
  const onPointerEnd = (event: ReactPointerEvent<HTMLDivElement>) => {
    pointers.current.delete(event.pointerId);
  };

  if (!item) return null;
  const title = item.caption || item.name;
  const atMax = !view || view.zoom >= limits.max;
  const atMin = !view || view.zoom <= limits.min;
  const atFit = !view || !shown.view;
  const pannable =
    view !== null && natural !== null && isZoomedIn(view, natural, stage);
  return (
    <Dialog
      title={title}
      onClose={onClose}
      fill
      actions={
        <>
          <fieldset
            aria-label="Zoom"
            className="mr-auto flex items-center gap-1 border-0 p-0"
          >
            <ZoomButton
              label="Zoom out"
              icon="minus"
              shortcut="Control+- Meta+-"
              disabled={atMin}
              onClick={() => step(-1)}
            />
            <output
              aria-live="polite"
              className="min-w-[3.5rem] text-center text-sm font-semibold tabular-nums"
            >
              {view ? zoomPercent(view.zoom) : "Fit"}
            </output>
            <ZoomButton
              label="Zoom in"
              icon="plus"
              shortcut="Control+= Meta+="
              disabled={atMax}
              onClick={() => step(1)}
            />
            <Button
              aria-keyshortcuts="0 Control+0 Meta+0"
              aria-disabled={atFit}
              onClick={() => {
                if (!atFit) fitAgain();
              }}
              className="ml-1 aria-disabled:cursor-not-allowed aria-disabled:opacity-50"
            >
              <Icon name="fit" size={14} />
              Fit
            </Button>
          </fieldset>
          {last > 0 ? (
            <>
              <span className="self-center text-xs text-ink-muted tabular-nums">
                {index + 1} of {items.length}
              </span>
              <Button disabled={index === 0} onClick={() => onIndex(index - 1)}>
                <Icon name="next" size={14} className="rotate-180" />
                Previous
              </Button>
              <Button
                disabled={index === last}
                onClick={() => onIndex(index + 1)}
              >
                Next
                <Icon name="next" size={14} />
              </Button>
            </>
          ) : null}
        </>
      }
    >
      <figure className="flex min-h-0 flex-1 flex-col gap-2">
        <div
          ref={stageRef}
          data-lightbox-stage=""
          onPointerDown={onPointerDown}
          onPointerMove={onPointerMove}
          onPointerUp={onPointerEnd}
          onPointerCancel={onPointerEnd}
          className={`relative min-h-0 flex-1 touch-none overflow-hidden rounded-card border border-rule bg-well select-none ${pannable ? "cursor-grab active:cursor-grabbing" : ""}`}
        >
          <img
            key={item.src}
            src={item.src}
            alt={title}
            draggable={false}
            onLoad={(event) => {
              const image = event.currentTarget;
              const size = {
                width: image.naturalWidth,
                height: image.naturalHeight,
              };
              setShown((current) =>
                current.src === item.src
                  ? { ...current, natural: size }
                  : current,
              );
            }}
            className={
              view && natural
                ? `absolute top-1/2 left-1/2 max-w-none ${view.zoom >= 2 ? "[image-rendering:pixelated]" : ""}`
                : "absolute inset-0 m-auto max-h-full max-w-full object-contain"
            }
            style={
              view && natural
                ? {
                    width: natural.width * view.zoom,
                    height: natural.height * view.zoom,
                    transform: `translate(-50%, -50%) translate(${view.x}px, ${view.y}px)`,
                  }
                : undefined
            }
          />
        </div>
        <figcaption className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 text-2xs">
          <span className="font-mono text-ink-faint">{item.name}</span>
          <span className="text-ink-muted max-sm:hidden">
            Drag to pan, scroll to zoom; arrows pan when zoomed in, 0 fits.
          </span>
        </figcaption>
      </figure>
    </Dialog>
  );
}

// ZoomButton is an icon button that stays focusable at its limit, so
// pressing it again keeps focus in the dialog.
function ZoomButton({
  label,
  icon,
  shortcut,
  disabled,
  onClick,
}: {
  label: string;
  icon: "plus" | "minus";
  shortcut: string;
  disabled: boolean;
  onClick(): void;
}) {
  return (
    <Button
      aria-label={label}
      title={label}
      aria-keyshortcuts={shortcut}
      aria-disabled={disabled}
      onClick={() => {
        if (!disabled) onClick();
      }}
      className="size-8 px-0 aria-disabled:cursor-not-allowed aria-disabled:opacity-50"
    >
      <Icon name={icon} size={14} />
    </Button>
  );
}
