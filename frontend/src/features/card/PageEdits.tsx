import { type KeyboardEvent, type ReactNode, useEffect, useRef } from "react";
import type { TicketDetail } from "../../api/board";
import type { Saved } from "../../api/edit";
import { TextInput } from "../../components/Field";
import { Icon } from "../../components/Icon";
import type { LinkContext } from "../../model/markdown";
import {
  type Criterion,
  criteriaOf,
  criteriaText,
  sectionSource,
  titleLine,
} from "../../model/sectionEdit";
import { InPlaceEditor, type InPlaceItem } from "../editing/InPlace";
import { InPlaceSection } from "../editing/InPlaceSection";
import type { TicketEdits } from "./useTicket";

// The full page's editors for the title and sections (FH-66), each open in
// place through editing's InPlaceEditor and saved through the ticket's
// edits: the title as a field, a section by its heading in the file.

interface EditorProps {
  item: InPlaceItem;
  detail: TicketDetail;
  edit: TicketEdits;
  onSaved(saved: Saved | undefined, what: string): void;
}

// TitleEditor edits the title on one line.
export function TitleEditor({ item, detail, edit, onSaved }: EditorProps) {
  return (
    <InPlaceEditor
      item={item}
      name="title"
      detail={detail}
      read={(ticket: TicketDetail) => ticket.title}
      isDirty={(draft, base) => draft.trim() !== base.trim()}
      check={(value) => (value.trim() ? undefined : "A ticket needs a title.")}
      send={(base, value) => edit.fields(base, { title: value.trim() })}
      mine={(value) => `# ${value.trim()}`}
      theirs={titleLine}
      onSaved={onSaved}
    >
      {(value, set) => (
        <input
          aria-label="Title"
          value={value}
          maxLength={300}
          onChange={(event) => set(event.target.value)}
          className="w-full rounded-control border border-rule bg-card px-2.5 py-1 text-2xl leading-tight text-ink display-cut transition-colors hover:border-ink-muted"
        />
      )}
    </InPlaceEditor>
  );
}

// Links says where a section's links and ticket ids lead, as in the view.
interface Links {
  context: LinkContext;
  keys: Set<string>;
}

// SectionEditor edits a section's markdown, by its heading in the file (an
// empty or missing section starts empty), in the formatted editor (FH-16):
// the text it saves is the text as edited, so an untouched section saves
// nothing. Files pasted, dropped or attached there go into the ticket at
// once and their references into the text, saved with it (FH-18).
export function SectionEditor({
  item,
  detail,
  edit,
  onSaved,
  heading,
  title,
  context,
  keys,
}: EditorProps & Links & { heading: string; title: string }) {
  return (
    <InPlaceSection
      item={item}
      name={title.toLowerCase()}
      title={title}
      heading={heading}
      detail={detail}
      body={(ticket) => ticket.body}
      send={edit.section}
      onSaved={onSaved}
      context={context}
      keys={keys}
      upload={edit.upload}
    />
  );
}

// CriteriaEditor edits the acceptance criteria as a list: add, reword,
// tick, remove and reorder them. A section with other text than its task
// list opens as markdown instead (SectionEditor).
export function CriteriaEditor({
  item,
  detail,
  edit,
  onSaved,
  heading,
  context,
  keys,
}: EditorProps & Links & { heading: string }) {
  if (!criteriaOf(sectionSource(detail.body, heading)))
    return (
      <SectionEditor
        item={item}
        detail={detail}
        edit={edit}
        onSaved={onSaved}
        heading={heading}
        title="Acceptance criteria"
        context={context}
        keys={keys}
      />
    );
  const listOf = (body: string): Row[] => {
    const criteria = criteriaOf(sectionSource(body, heading)) ?? [];
    // An empty list starts with one criterion to write.
    const rows = criteria.length ? criteria : [{ text: "", done: false }];
    return rows.map((criterion, key) => ({ ...criterion, key }));
  };
  return (
    <InPlaceEditor<TicketDetail, Row[]>
      item={item}
      name="acceptance criteria"
      detail={detail}
      read={(ticket) => listOf(ticket.body)}
      readFile={listOf}
      isDirty={(draft, base) => criteriaText(draft) !== criteriaText(base)}
      send={(base, rows) => edit.section(base, heading, criteriaText(rows))}
      mine={criteriaText}
      theirs={(file) => sectionSource(file, heading)}
      onSaved={onSaved}
    >
      {(rows, set) => <CriteriaList rows={rows} onChange={set} />}
    </InPlaceEditor>
  );
}

// Row is a criterion with a key that stays with it as the list changes.
interface Row extends Criterion {
  key: number;
}

type Control = "text" | "up" | "down" | "remove" | "add";

function CriteriaList({
  rows,
  onChange,
}: {
  rows: Row[];
  onChange(rows: Row[]): void;
}) {
  const list = useRef<HTMLFieldSetElement>(null);
  // focus is the control to focus once the list has changed: a row's
  // (by key) or Add a criterion.
  const focus = useRef<{ key?: number; control: Control } | null>(null);
  useEffect(() => {
    const target = focus.current;
    if (!target || !list.current) return;
    focus.current = null;
    const control = (selector: string) =>
      list.current?.querySelector<HTMLElement>(selector);
    const row = `[data-key="${target.key}"]`;
    const element =
      target.control === "add"
        ? control('[data-control="add"]')
        : (control(
            `${row} [data-control="${target.control}"]:not(:disabled)`,
          ) ??
          control(
            `${row} input[aria-label^="Criterion"]:not([type="checkbox"])`,
          ));
    element?.focus();
  });

  const nextKey = () => Math.max(-1, ...rows.map((row) => row.key)) + 1;
  const insertAfter = (index: number) => {
    const key = nextKey();
    const next = [...rows];
    next.splice(index + 1, 0, { key, text: "", done: false });
    focus.current = { key, control: "text" };
    onChange(next);
  };
  const update = (index: number, change: Partial<Criterion>) =>
    onChange(
      rows.map((row, at) => (at === index ? { ...row, ...change } : row)),
    );
  const move = (index: number, by: -1 | 1) => {
    const row = rows[index];
    if (!row) return;
    const next = rows.filter((_, at) => at !== index);
    next.splice(index + by, 0, row);
    focus.current = { key: row.key, control: by < 0 ? "up" : "down" };
    onChange(next);
  };
  const remove = (index: number) => {
    const neighbour = rows[index + 1] ?? rows[index - 1];
    focus.current = neighbour
      ? { key: neighbour.key, control: "text" }
      : { control: "add" };
    onChange(rows.filter((_, at) => at !== index));
  };
  const onKeyDown = (index: number) => (event: KeyboardEvent) => {
    if (event.key !== "Enter" || event.ctrlKey || event.metaKey) return;
    event.preventDefault();
    insertAfter(index);
  };

  return (
    <fieldset ref={list} className="flex min-w-0 flex-col gap-2">
      <legend className="sr-only">Acceptance criteria</legend>
      <ol className="divide-y divide-rule rounded-card border border-rule bg-card">
        {rows.map((row, index) => {
          const n = index + 1;
          return (
            <li
              key={row.key}
              data-key={row.key}
              className="flex items-center gap-1.5 px-2 py-1.5"
            >
              <input
                type="checkbox"
                aria-label={`Criterion ${n} done`}
                checked={row.done}
                onChange={(event) =>
                  update(index, { done: event.target.checked })
                }
                className="size-4 shrink-0 accent-[var(--fh-select)]"
              />
              <span className="min-w-0 flex-1">
                <TextInput
                  aria-label={`Criterion ${n}`}
                  value={row.text}
                  onChange={(text) => update(index, { text })}
                  onKeyDown={onKeyDown(index)}
                  className="min-w-0"
                />
              </span>
              <RowButton
                label={`Move criterion ${n} up`}
                control="up"
                disabled={index === 0}
                onClick={() => move(index, -1)}
              >
                <Icon name="chevronDown" size={14} className="rotate-180" />
              </RowButton>
              <RowButton
                label={`Move criterion ${n} down`}
                control="down"
                disabled={index === rows.length - 1}
                onClick={() => move(index, 1)}
              >
                <Icon name="chevronDown" size={14} />
              </RowButton>
              <RowButton
                label={`Remove criterion ${n}`}
                control="remove"
                onClick={() => remove(index)}
              >
                <Icon name="close" size={14} />
              </RowButton>
            </li>
          );
        })}
      </ol>
      <button
        type="button"
        data-control="add"
        onClick={() => insertAfter(rows.length - 1)}
        className="inline-flex items-center gap-1.5 self-start rounded-control px-2 py-1 text-xs font-semibold text-ink-muted transition-colors hover:bg-well hover:text-ink"
      >
        <Icon name="plus" size={12} />
        Add a criterion
      </button>
    </fieldset>
  );
}

function RowButton({
  label,
  control,
  disabled,
  onClick,
  children,
}: {
  label: string;
  control: Control;
  disabled?: boolean;
  onClick(): void;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      data-control={control}
      disabled={disabled}
      onClick={onClick}
      className="grid size-7 shrink-0 place-items-center rounded-control text-ink-muted transition-colors hover:enabled:bg-well hover:enabled:text-ink disabled:opacity-35"
    >
      {children}
    </button>
  );
}
