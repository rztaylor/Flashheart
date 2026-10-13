import { typeToken } from "../model/paint";
import { Icon, type IconName } from "./Icon";

// A ticket's type and priority as small marks (FH-147, ui-layout.md §2),
// drawn for the board's cards and its type chips. They are drawings: the
// type's name and the priority's word go in the tooltip and the
// accessible name of whatever holds them.

const typeIcons: Record<string, IconName> = {
  feature: "star",
  bug: "bug",
  infra: "server",
  test: "beaker",
  refactor: "swap",
  docs: "doc",
  spike: "bulb",
};

// TypeIcon is a type's icon in its paint colour; a type with no icon of
// its own is a dot in the "other" colour.
export function TypeIcon({ type, size = 13 }: { type: string; size?: number }) {
  const color = `var(--fh-paint-${typeToken(type)})`;
  const icon = typeIcons[type];
  if (!icon)
    return (
      <span
        aria-hidden="true"
        data-type={type}
        data-type-dot=""
        style={{ color, width: size, height: size }}
        className="grid shrink-0 place-items-center"
      >
        <span className="size-[45%] rounded-full bg-current" />
      </span>
    );
  return (
    <span
      aria-hidden="true"
      data-type={type}
      style={{ color }}
      className="inline-flex shrink-0"
    >
      <Icon name={icon} size={size} />
    </span>
  );
}

const levels: Record<string, number> = { high: 3, medium: 2, low: 1 };

// hasPriorityMarker says whether a priority has bars to draw: a word a
// file uses beyond high, medium and low has none.
export const hasPriorityMarker = (priority: string) =>
  levels[priority] !== undefined;

// PriorityMarker is three ascending bars: high fills all three in its warm
// colour, medium two in neutral grey, low one in blue; the rest are empty.
// An unknown or missing priority draws nothing.
export function PriorityMarker({
  priority,
  size = 12,
}: {
  priority: string;
  size?: number;
}) {
  const filled = levels[priority];
  if (!filled) return null;
  return (
    <svg
      viewBox="0 0 12 12"
      width={size}
      height={size}
      aria-hidden="true"
      data-priority={priority}
      className="shrink-0"
    >
      {[4.5, 7.75, 11].map((height, index) => {
        const on = index < filled;
        return (
          <rect
            key={height}
            data-bar={on ? "on" : "off"}
            x={0.5 + index * 4}
            y={11.5 - height}
            width={3}
            height={height}
            rx={0.75}
            fill={
              on ? `var(--fh-priority-${priority})` : "var(--fh-priority-off)"
            }
          />
        );
      })}
    </svg>
  );
}
