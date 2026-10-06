import type { ReactNode } from "react";

const sizes = {
  md: "h-7 min-w-8 px-1.5 text-sm",
  lg: "h-10 min-w-10 px-2 text-lg",
};

// KeyBadge is a project's key (FH) as a badge filled in the accent: the page
// header's identity and a project section's heading in Workstreams.
// Decorative; the project's name is beside it.
export function KeyBadge({
  children,
  size = "md",
}: {
  children: ReactNode;
  size?: keyof typeof sizes;
}) {
  return (
    <span
      aria-hidden="true"
      className={`grid shrink-0 place-items-center rounded-control bg-rail-active-mark text-on-rail-active-mark ${sizes[size]}`}
    >
      {children}
    </span>
  );
}
