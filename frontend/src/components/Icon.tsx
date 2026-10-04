// Authored icon set: 24px grid, 1.75 stroke, round joins. Decorative by
// default; give the parent control an accessible name.

const paths = {
  search: "M10.5 18a7.5 7.5 0 1 0 0-15 7.5 7.5 0 0 0 0 15Zm5.3-2.2L21 21",
  close: "M6 6l12 12M18 6 6 18",
  diamond: "M12 3.5 20.5 12 12 20.5 3.5 12Z",
  warning: "M12 4 21 19.5H3L12 4Zm0 6v4.5m0 2.6v.1",
  repair:
    "M14.5 6.5a4 4 0 0 0 5 5L12 19a2.1 2.1 0 0 1-3-3l7.5-7.5Zm0 0L17 4l3 3-2.5 2.5",
  criteria: "M4.5 4.5h15v15h-15zM8 12l3 3 5-6",
  attachment:
    "M16.5 7.5 9 15a2.1 2.1 0 0 0 3 3l7.5-7.5a4.2 4.2 0 0 0-6-6L6 12a6.4 6.4 0 0 0 9 9",
  review: "M7 3.5h7l4.5 4.5v12.5H7zM14 3.5V8h4.5M10 13l2 2 3.5-4",
  external: "M14 4h6v6M20 4l-9 9M18 14v6H4V6h6",
  power: "M12 3v8M7 6.3a8 8 0 1 0 10 0",
  board: "M4 4h4.5v16H4zM9.75 4h4.5v10h-4.5zM15.5 4H20v13h-4.5z",
  lines: "M3 7h6l3 5 3-5h6M3 17h18M6 7v0m12 0v0m-6 10v0",
  table: "M3.5 5h17v14h-17zM3.5 10h17M3.5 14.5h17M9 10v9",
  branch:
    "M6 4v10m0 0a2.5 2.5 0 1 0 0 5 2.5 2.5 0 0 0 0-5Zm12-6a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5Zm0 0c0 5-6 4-12 6",
  next: "M5 12h12M13 7l5 5-5 5",
  chevronDown: "M6 9l6 6 6-6",
  refresh:
    "M20 11a8 8 0 0 0-14.5-4.5L4 8m0-4v4h4M4 13a8 8 0 0 0 14.5 4.5L20 16m0 4v-4h-4",
} as const;

export type IconName = keyof typeof paths;

export function Icon({
  name,
  size = 16,
  className,
}: {
  name: IconName;
  size?: number;
  className?: string;
}) {
  return (
    <svg
      viewBox="0 0 24 24"
      width={size}
      height={size}
      fill="none"
      stroke="currentColor"
      strokeWidth={1.75}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      className={["shrink-0", className].filter(Boolean).join(" ")}
    >
      <path d={paths[name]} />
    </svg>
  );
}
