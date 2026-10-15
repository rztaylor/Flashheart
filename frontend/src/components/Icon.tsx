// Authored icon set: 24px grid, 1.75 stroke, round joins. Decorative by
// default; give the parent control an accessible name.

const paths = {
  search: "M10.5 18a7.5 7.5 0 1 0 0-15 7.5 7.5 0 0 0 0 15Zm5.3-2.2L21 21",
  close: "M6 6l12 12M18 6 6 18",
  warning: "M12 4 21 19.5H3L12 4Zm0 6v4.5m0 2.6v.1",
  repair:
    "M14.5 6.5a4 4 0 0 0 5 5L12 19a2.1 2.1 0 0 1-3-3l7.5-7.5Zm0 0L17 4l3 3-2.5 2.5",
  criteria: "M4.5 4.5h15v15h-15zM8 12l3 3 5-6",
  attachment:
    "M16.5 7.5 9 15a2.1 2.1 0 0 0 3 3l7.5-7.5a4.2 4.2 0 0 0-6-6L6 12a6.4 6.4 0 0 0 9 9",
  review: "M7 3.5h7l4.5 4.5v12.5H7zM14 3.5V8h4.5M10 13l2 2 3.5-4",
  external: "M14 4h6v6M20 4l-9 9M18 14v6H4V6h6",
  expand: "M14 4h6v6M10 20H4v-6M20 4l-6.5 6.5M4 20l6.5-6.5",
  power: "M12 3v8M7 6.3a8 8 0 1 0 10 0",
  // settings: a gear of eight teeth round a hub, the band's Settings menu
  // (FH-149).
  settings:
    "M10 5.3l.5-2.7h3l.5 2.7a7 7 0 0 1 1.3.5l2.3-1.5 2.1 2.1-1.5 2.3a7 7 0 0 1 .5 1.3l2.7.5v3l-2.7.5a7 7 0 0 1-.5 1.3l1.5 2.3-2.1 2.1-2.3-1.5a7 7 0 0 1-1.3.5l-.5 2.7h-3l-.5-2.7a7 7 0 0 1-1.3-.5l-2.3 1.5-2.1-2.1 1.5-2.3a7 7 0 0 1-.5-1.3l-2.7-.5v-3l2.7-.5a7 7 0 0 1 .5-1.3L4.3 6.4l2.1-2.1 2.3 1.5a7 7 0 0 1 1.3-.5ZM12 9a3 3 0 1 0 0 6 3 3 0 0 0 0-6Z",
  board: "M4 4h4.5v16H4zM9.75 4h4.5v10h-4.5zM15.5 4H20v13h-4.5z",
  lines: "M3 7h6l3 5 3-5h6M3 17h18M6 7v0m12 0v0m-6 10v0",
  table: "M3.5 5h17v14h-17zM3.5 10h17M3.5 14.5h17M9 10v9",
  branch:
    "M6 4v10m0 0a2.5 2.5 0 1 0 0 5 2.5 2.5 0 0 0 0-5Zm12-6a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5Zm0 0c0 5-6 4-12 6",
  // pullRequest: two commits, the head's line arcing back into the base.
  pullRequest:
    "M6 8v8m0-8a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5Zm0 8a2.5 2.5 0 1 0 0 5 2.5 2.5 0 0 0 0-5Zm12 0a2.5 2.5 0 1 0 0 5 2.5 2.5 0 0 0 0-5Zm0 0V9a3 3 0 0 0-3-3h-4m2-2.5L10.5 6 13 8.5",
  next: "M5 12h12M13 7l5 5-5 5",
  chevronDown: "M6 9l6 6 6-6",
  plus: "M12 5v14M5 12h14",
  minus: "M5 12h14",
  fit: "M4 9V4h5M15 4h5v5M20 15v5h-5M9 20H4v-5M9 9h6v6H9z",
  archive: "M3.5 5h17v4h-17zM5 9v10h14V9M10 13h4",
  image: "M4 5.5h16v13H4zM4 15.5l5-5 4 4 2.5-2.5 4.5 4.5M15.5 9.5h.01",
  undo: "M9 7 4 12l5 5M4 12h11a5 5 0 0 1 0 10h-2",
  edit: "M4 20h4L19 9l-4-4L4 16v4ZM13.5 6.5l4 4",
  // comment: a speech balloon, a person's comment on a ticket.
  comment: "M4.5 5h15v10.5h-8.5l-4.5 4v-4h-2Z",
  // history: a clock face with an arrow turning back, a ticket's changes.
  history: "M4.5 12a7.5 7.5 0 1 0 2.2-5.3L4.5 9M4.5 5v4h4M12 8v4.5l3 1.5",
  grip: "M9 6h.01M9 12h.01M9 18h.01M15 6h.01M15 12h.01M15 18h.01",
  more: "M5 12a1 1 0 1 0 2 0 1 1 0 1 0-2 0M11 12a1 1 0 1 0 2 0 1 1 0 1 0-2 0M17 12a1 1 0 1 0 2 0 1 1 0 1 0-2 0",
  // overview: section cards stacked in one column.
  overview: "M4.5 4h15v5h-15zM4.5 12h15v3.5h-15zM4.5 19.5h9",
  check: "M5 12.5l4.5 4.5L19 7.5",
  ring: "M12 4.5a7.5 7.5 0 1 0 0 15 7.5 7.5 0 0 0 0-15Z",
  half: "M12 4.5a7.5 7.5 0 1 0 0 15 7.5 7.5 0 0 0 0-15ZM12 4.5v15",
  // The editor's selection toolbar (FH-16): a link's two chain loops, a
  // bulleted list, and code's angle brackets.
  link: "M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1",
  list: "M9.5 6.5h10M9.5 12h10M9.5 17.5h10M5 6.5h.01M5 12h.01M5 17.5h.01",
  code: "M8.5 7 3.5 12l5 5M15.5 7l5 5-5 5",
  refresh:
    "M20 11a8 8 0 0 0-14.5-4.5L4 8m0-4v4h4M4 13a8 8 0 0 0 14.5 4.5L20 16m0 4v-4h-4",
  // trash: a bin with its lid, deleting a comment (FH-140).
  trash:
    "M4.5 6.5h15M9.5 6.5V4h5v2.5M6.5 6.5l1 13.5h9l1-13.5M10.5 10.5v6m3-6v6",
  // Ticket types (FH-147, components/TicketMarks): feature a star, bug a
  // beetle, infra a server stack, test a beaker, refactor two swapping
  // arrows, docs a page of lines, spike a light bulb.
  star: "M12 3.5l2.6 5.3 5.9.9-4.25 4.1 1 5.8L12 16.85 6.75 19.6l1-5.8L3.5 9.7l5.9-.9Z",
  bug: "M9 8a3 3 0 0 1 6 0M8 11a4 4 0 0 1 8 0v3.5a4 4 0 0 1-8 0ZM12 11v7.5M4.5 13H8m8 0h3.5M5.5 8.5 8 10m10.5-1.5L16 10M5.5 18.5 8 17m10.5 1.5L16 17",
  server: "M4.5 4.5h15v6h-15zM4.5 13.5h15v6h-15zM8 7.5h.01M8 16.5h.01",
  beaker:
    "M9 3.5h6M10.25 3.5v6.25L5.2 18.6A1.25 1.25 0 0 0 6.3 20.5h11.4a1.25 1.25 0 0 0 1.1-1.9l-5.05-8.85V3.5M7.6 14.5h8.8",
  swap: "M4 8h14m-3.5-3.5L18 8l-3.5 3.5M20 16H6m3.5-3.5L6 16l3.5 3.5",
  doc: "M6.5 3.5H14l3.5 3.5v13.5h-11zM13.5 3.5V7.5h4M9.5 12h5m-5 3.5h5",
  bulb: "M9.5 18h5M10.5 21h3M12 3.5a5.5 5.5 0 0 0-3.25 9.9c.8.6 1.25 1.45 1.25 2.4V16h4v-.2c0-.95.45-1.8 1.25-2.4A5.5 5.5 0 0 0 12 3.5Z",
} as const;

// Filled marks: a solid shape with its inner mark cut out (even-odd), so
// the cut shows the surface behind it in any theme. blocked is the
// no-entry sign, the board's one symbol for blocked (FH-148): a disc with
// a horizontal bar, legible at 10–13px.
const filled = {
  blocked:
    "M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18ZM7.75 10.4h8.5a.6.6 0 0 1 .6.6v2a.6.6 0 0 1-.6.6h-8.5a.6.6 0 0 1-.6-.6v-2a.6.6 0 0 1 .6-.6Z",
} as const;

export type IconName = keyof typeof paths | keyof typeof filled;

// iconNames lists the whole set.
export const iconNames = [
  ...Object.keys(paths),
  ...Object.keys(filled),
] as IconName[];

export function Icon({
  name,
  size = 16,
  className,
}: {
  name: IconName;
  size?: number;
  className?: string;
}) {
  const classes = ["shrink-0", className].filter(Boolean).join(" ");
  if (name in filled)
    return (
      <svg
        viewBox="0 0 24 24"
        width={size}
        height={size}
        fill="currentColor"
        aria-hidden="true"
        data-icon={name}
        className={classes}
      >
        <path fillRule="evenodd" d={filled[name as keyof typeof filled]} />
      </svg>
    );
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
      data-icon={name}
      className={classes}
    >
      <path d={paths[name as keyof typeof paths]} />
    </svg>
  );
}
