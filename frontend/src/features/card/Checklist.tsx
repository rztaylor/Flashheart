import { type MouseEvent, type ReactNode, useEffect, useState } from "react";

import { Icon } from "../../components/Icon";

export interface ChecklistItem {
  text: string;
  done: boolean;
}

// Checklist is a list of tickable items with a "d of t" count: a ticket's
// acceptance criteria (CARD-3) or a review's How to Verify steps (REV-3).
// Items are addressed by position in the file, and their text may repeat,
// so the key is position and text. Without onToggle it is read-only.
export function Checklist({
  id,
  title,
  items,
  render = (text) => text,
  onToggle,
  level = 3,
  intro,
  outro,
  action,
  onDoubleClick,
  className = "",
}: {
  id: string;
  title: string;
  items: ChecklistItem[];
  render?(text: string): ReactNode;
  // onToggle resolves false when the change was not saved.
  onToggle?(index: number, checked: boolean): Promise<boolean>;
  // level is the heading's: 3 in the panel, 2 or 3 on the full page.
  level?: 2 | 3;
  // intro and outro are the section's own text around the list (a
  // review's How to Verify on the full page).
  intro?: ReactNode;
  outro?: ReactNode;
  // action sits beside the heading (the full page's pencil, FH-66), and
  // onDoubleClick on the heading opens the same edit.
  action?: ReactNode;
  onDoubleClick?(event: MouseEvent): void;
  className?: string;
}) {
  const done = items.filter((item) => item.done).length;
  const keyed = items.map((item, index) => ({
    ...item,
    index,
    key: `${index}:${item.text}`,
  }));
  const Heading = level === 2 ? "h2" : "h3";
  return (
    <section aria-labelledby={id} className={className}>
      {/* biome-ignore lint/a11y/noStaticElementInteractions: a double-click is a shortcut; the pencil is the control. */}
      <div
        className="mb-2 flex items-center gap-2"
        onDoubleClick={onDoubleClick}
      >
        <Heading
          id={id}
          className="flex flex-1 items-baseline justify-between text-md heading-cut"
        >
          {title}
          <span className="text-sm font-normal text-ink-muted">
            {done} of {items.length}
          </span>
        </Heading>
        {action ? <span className="-my-1.5">{action}</span> : null}
      </div>
      {intro ? <div className="mb-3">{intro}</div> : null}
      <ul className="divide-y divide-rule overflow-hidden rounded-card border border-rule bg-card">
        {keyed.map((item) =>
          onToggle ? (
            <li key={item.key}>
              <Tickable
                done={item.done}
                onToggle={(checked) => onToggle(item.index, checked)}
              >
                {render(item.text)}
              </Tickable>
            </li>
          ) : (
            <li
              key={item.key}
              className="flex items-start gap-2.5 px-3 py-2 text-sm"
            >
              <span
                aria-hidden="true"
                className={`mt-[0.15em] grid size-4 shrink-0 place-items-center rounded-mark border ${item.done ? "border-select bg-select text-card" : "border-ink-muted"}`}
              >
                {item.done ? <Icon name="check" size={11} /> : null}
              </span>
              <span className={item.done ? "text-ink-muted" : "text-ink"}>
                <span className="sr-only">
                  {item.done ? "Done: " : "Not done: "}
                </span>
                {render(item.text)}
              </span>
            </li>
          ),
        )}
      </ul>
      {outro ? <div className="mt-3">{outro}</div> : null}
    </section>
  );
}

// Tickable shows the new state at once and follows the file when it
// reloads.
function Tickable({
  done,
  onToggle,
  children,
}: {
  done: boolean;
  onToggle(checked: boolean): Promise<boolean>;
  children: ReactNode;
}) {
  const [checked, setChecked] = useState(done);
  useEffect(() => setChecked(done), [done]);
  return (
    <label className="flex cursor-pointer items-start gap-2.5 px-3 py-2 text-sm">
      <input
        type="checkbox"
        checked={checked}
        onChange={(event) => {
          const next = event.target.checked;
          setChecked(next);
          void onToggle(next).then((ok) => {
            if (!ok) setChecked(done);
          });
        }}
        className="mt-[0.15em] size-4 shrink-0 accent-[var(--fh-select)]"
      />
      <span className={checked ? "text-ink-muted" : "text-ink"}>
        {children}
      </span>
    </label>
  );
}
