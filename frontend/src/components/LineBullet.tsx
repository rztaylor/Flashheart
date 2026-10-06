import type { CSSProperties } from "react";

import type { Line } from "../model/lines";

interface LineBulletProps {
  line?: Line;
  // Workstream name for assistive technology; omit when the name is shown
  // next to the bullet.
  label?: string;
  size?: "sm" | "md" | "lg" | "xl";
  dimmed?: boolean;
}

const sizes = {
  sm: "size-5 text-[0.6875rem]",
  md: "size-6 text-xs",
  lg: "size-8 text-sm",
  xl: "size-11 text-lg",
};

// LineBullet is the round route bullet that marks a workstream. Its colour
// always means a workstream and nothing else.
export function LineBullet({
  line,
  label,
  size = "md",
  dimmed,
}: LineBulletProps) {
  if (!line) return null;
  const style = {
    background: `var(--fh-line-${line.colour})`,
    color: `var(--fh-line-ink-${line.colour})`,
    boxShadow: "inset 0 0 0 1px var(--fh-casing)",
  } satisfies CSSProperties;
  const className = `inline-grid shrink-0 place-items-center rounded-full leading-none display-cut transition-opacity ${sizes[size]} ${dimmed ? "opacity-30" : ""}`;
  if (label) {
    return (
      <span
        role="img"
        aria-label={`Workstream ${label}`}
        className={className}
        style={style}
      >
        {line.initials}
      </span>
    );
  }
  return (
    <span aria-hidden="true" className={className} style={style}>
      {line.initials}
    </span>
  );
}
