// TailText shows one line of a path, branch or id cut from its start, so its
// distinguishing tail stays in view (ui-layout.md §8, "Long content"). The
// box runs right to left to put the ellipsis at the start; the value sits
// in a <bdi> so its own characters keep their left-to-right order (a
// trailing slash or dot is not moved to the front). The full value stays in
// the title, and the text itself is whole for assistive technology.
export function TailText({
  children,
  as: Element = "span",
  className,
}: {
  children: string;
  as?: "span" | "dd";
  className?: string;
}) {
  return (
    <Element
      className={`block min-w-0 truncate text-left [direction:rtl] ${className ?? ""}`}
      title={children}
    >
      <bdi>{children}</bdi>
    </Element>
  );
}
