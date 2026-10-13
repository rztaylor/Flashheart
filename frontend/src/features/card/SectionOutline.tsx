import { Icon } from "../../components/Icon";
import { Popover } from "../../components/Popover";
import type { PageSection } from "../../model/ticketSections";

// countWords is a section's count as read aloud: "1 of 2" for a checklist.
const countWords = (count: string) => count.replace("/", " of ");

// Entry is one section in the outline or the Jump to menu: a button that
// goes to it, its count beside it, or, for an empty section, its name
// marked empty and nowhere to go.
function Entry({
  section,
  current,
  onChoose,
  nested = false,
  group = false,
}: {
  section: PageSection;
  current: boolean;
  onChoose(id: string): void;
  nested?: boolean;
  group?: boolean;
}) {
  const indent = nested ? "pl-5" : "pl-2.5";
  if (section.empty)
    return (
      <li
        className={`flex h-8 items-center pr-2.5 text-sm text-ink-faint italic ${indent}`}
      >
        {`${section.title}, empty`}
      </li>
    );
  return (
    <li>
      <button
        type="button"
        aria-current={current ? "location" : undefined}
        onClick={() => onChoose(section.id)}
        className={`flex min-h-8 w-full items-center gap-2 rounded-control py-1 pr-2.5 text-left text-sm transition-colors ${indent} ${
          current
            ? "bg-select-surface font-semibold text-ink"
            : group
              ? "font-semibold text-ink hover:bg-well"
              : "text-ink-muted hover:bg-well hover:text-ink"
        }`}
      >
        {section.kind === "review" ? (
          <Icon
            name="review"
            size={14}
            className="shrink-0 text-(--fh-state-review-ink)"
          />
        ) : null}
        <span className="min-w-0 flex-1">{section.title}</span>
        {section.count ? (
          <>
            <span
              aria-hidden="true"
              className={`shrink-0 text-2xs font-semibold tabular-nums ${
                section.complete
                  ? "text-(--fh-state-done-ink)"
                  : "text-ink-faint"
              }`}
            >
              {section.count}
            </span>
            <span className="sr-only"> {countWords(section.count)}</span>
          </>
        ) : null}
      </button>
    </li>
  );
}

// OutlineList lists the page's sections in their groups: the ticket's
// own, then Review with its sections beneath it, then Files and Activity.
function OutlineList({
  sections,
  current,
  onChoose,
}: {
  sections: PageSection[];
  current: string;
  onChoose(id: string): void;
}) {
  const entry = (section: PageSection, nested = false) => (
    <Entry
      key={section.id}
      section={section}
      current={section.id === current}
      onChoose={onChoose}
      nested={nested}
      group={section.kind === "review" || section.kind === "activity"}
    />
  );
  const ticket = sections.filter((section) => section.group === "ticket");
  const review = sections.filter((section) => section.group === "review");
  const rest = sections.filter(
    (section) => section.group === "files" || section.group === "activity",
  );
  const [part, ...parts] = review;
  return (
    <ul className="flex flex-col gap-0.5">
      {ticket.map((section) => entry(section))}
      {part ? (
        <li className="mt-3">
          <ul className="flex flex-col gap-0.5">
            {entry(part)}
            {parts.map((section) => entry(section, true))}
          </ul>
        </li>
      ) : null}
      {rest.map((section) => (
        <li key={section.id} className="mt-3">
          <ul>{entry(section)}</ul>
        </li>
      ))}
    </ul>
  );
}

// SectionOutline is the full page's outline from 1280px (FH-65): it stays
// in view, marks the section in view and goes to a chosen one.
export function SectionOutline(props: {
  sections: PageSection[];
  current: string;
  onChoose(id: string): void;
}) {
  return (
    <nav aria-label="Sections">
      <OutlineList {...props} />
    </nav>
  );
}

// JumpTo is the outline below 1280px: a menu of the same sections, named
// with the section in view.
export function JumpTo({
  sections,
  current,
  onChoose,
}: {
  sections: PageSection[];
  current: string;
  onChoose(id: string): void;
}) {
  const here = sections.find((section) => section.id === current);
  return (
    <Popover
      label="Jump to"
      button={
        <>
          <span>Jump to</span>
          {here ? (
            <span className="max-w-48 truncate font-normal text-ink-muted">
              {here.title}
            </span>
          ) : null}
          <Icon name="chevronDown" size={12} className="text-ink-muted" />
        </>
      }
    >
      {(close) => (
        <div className="max-h-[60vh] w-64 overflow-y-auto">
          <OutlineList
            sections={sections}
            current={current}
            onChoose={(id) => {
              close();
              onChoose(id);
            }}
          />
        </div>
      )}
    </Popover>
  );
}
