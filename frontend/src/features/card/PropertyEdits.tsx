import {
  type MouseEvent,
  type ReactNode,
  useEffect,
  useRef,
  useState,
} from "react";

import { TICKET_ID, type TicketDetail } from "../../api/board";
import type { FieldValue, Saved } from "../../api/edit";
import {
  ChipInput,
  type ChipList,
  withTyped,
} from "../../components/ChipInput";
import { Select, TextInput } from "../../components/Field";
import { fieldLines } from "../../model/sectionEdit";
import { fieldsText } from "../../model/ticketForm";
import {
  EditPencil,
  editGroup,
  InPlaceEditor,
  useInPlaceItem,
} from "../editing/InPlace";
import type { TicketEdits } from "./useTicket";

// A property of the full page edited in place (FH-66, EDIT-6): its row
// shows the value with a pencil; open, the row holds its typed control
// under its name, saved as that one field.

// PropertyRow is one property: its name and value, and the pencil that
// opens it when it can be edited.
export function PropertyRow({
  label,
  children,
  action,
  onDoubleClick,
}: {
  label: string;
  children: ReactNode;
  action?: ReactNode;
  onDoubleClick?(event: MouseEvent): void;
}) {
  return (
    <div className={`contents ${editGroup}`}>
      <dt className="py-1.5 text-ink-muted">{label}</dt>
      <dd
        onDoubleClick={onDoubleClick}
        className="flex min-w-0 flex-wrap items-center gap-1.5 py-1.5 text-ink"
      >
        {children}
        {action ? <span className="ml-auto -my-2">{action}</span> : null}
      </dd>
    </div>
  );
}

export interface PropertyEditing {
  detail: TicketDetail;
  edit: TicketEdits;
  onSaved(saved: Saved | undefined, what: string): void;
}

// EditableProperty is a row whose value opens as a typed control: field is
// the frontmatter key saved, name what the pencil says ("Edit depends
// on"). Without editing it is a plain row.
export function EditableProperty<T>({
  label,
  field,
  editing,
  view,
  read,
  isDirty,
  value: toValue,
  check,
  control,
}: {
  label: string;
  field: string;
  editing?: PropertyEditing;
  view: ReactNode;
  read(detail: TicketDetail): T;
  isDirty(draft: T, base: T): boolean;
  // value is what is saved for a draft.
  value(draft: T): FieldValue;
  check?(draft: T): string | undefined;
  control(value: T, set: (value: T) => void): ReactNode;
}) {
  const name = label.toLowerCase();
  const item = useInPlaceItem(`property-${field}`, name);
  if (!editing || !item.editable)
    return <PropertyRow label={label}>{view}</PropertyRow>;
  if (item.open)
    return (
      <div className="contents">
        <dt className="col-span-2 pt-1.5 text-ink-muted">{label}</dt>
        <dd className="col-span-2 pt-1 pb-1.5">
          <InPlaceEditor
            item={item}
            name={name}
            detail={editing.detail}
            read={read}
            isDirty={isDirty}
            check={check}
            send={(base, draft) =>
              editing.edit.fields(base, { [field]: toValue(draft) })
            }
            mine={(draft) => fieldsText({ [field]: toValue(draft) })}
            theirs={(file) => fieldLines(file, field)}
            onSaved={editing.onSaved}
            className="p-2.5"
          >
            {(value, set) => control(value, (next) => set(() => next))}
          </InPlaceEditor>
        </dd>
      </div>
    );
  return (
    <PropertyRow
      label={label}
      onDoubleClick={item.onDoubleClick}
      action={<EditPencil item={item} name={name} />}
    >
      {view}
    </PropertyRow>
  );
}

// textProperty reads and saves a one-line field (branch, pull request).
export const textProperty = (key: keyof TicketDetail, label: string) => ({
  read: (detail: TicketDetail) => String(detail[key] ?? ""),
  isDirty: (draft: string, base: string) => draft.trim() !== base.trim(),
  value: (draft: string) => draft.trim(),
  control: (value: string, set: (value: string) => void) => (
    <TextInput
      aria-label={label}
      value={value}
      onChange={set}
      spellCheck={false}
      className="font-mono text-xs"
    />
  ),
});

// choiceProperty reads and saves a field chosen from a list (priority,
// type, workstream); a value not in the list stays offered.
export const choiceProperty = (
  key: keyof TicketDetail,
  label: string,
  choices: { value: string; label: string }[],
) => ({
  read: (detail: TicketDetail) => String(detail[key] ?? ""),
  isDirty: (draft: string, base: string) => draft !== base,
  value: (draft: string) => draft,
  control: (value: string, set: (value: string) => void) => (
    <Select aria-label={label} value={value} onChange={set}>
      {choices.some((choice) => choice.value === value)
        ? null
        : [
            <option key={value} value={value}>
              {value || "None"}
            </option>,
          ]}
      {choices.map((choice) => (
        <option key={choice.value} value={choice.value}>
          {choice.label}
        </option>
      ))}
    </Select>
  ),
});

const sameList = (a: string[], b: string[]) =>
  a.length === b.length && a.every((item, index) => item === b[index]);

// listProperty reads and saves a list field as chips; what is typed and
// not yet added is saved with them.
const listProperty = (
  read: (detail: TicketDetail) => string[],
  normalize?: (text: string) => string,
) => ({
  read: (detail: TicketDetail): ChipList => ({
    values: read(detail),
    text: "",
  }),
  isDirty: (draft: ChipList, base: ChipList) =>
    !sameList(withTyped(draft, normalize), base.values),
  value: (draft: ChipList) => withTyped(draft, normalize),
});

// tagsProperty edits tags as chips.
export const tagsProperty = () => ({
  ...listProperty((detail) => detail.tags),
  control: (value: ChipList, set: (value: ChipList) => void) => (
    <ChipInput
      label="Tags"
      list={value}
      onChange={set}
      addLabel="Add a tag"
      removeLabel={(tag) => `Remove tag ${tag}`}
      placeholder="Type a tag, then Enter"
    />
  ),
});

const ticketId = (text: string) => text.trim().toUpperCase();

// dependsOnProperty edits depends on as a picker of ticket ids, offering
// the project's tickets.
export const dependsOnProperty = (
  self: string,
  load?: TicketEdits["tickets"],
) => {
  const check = (id: string) =>
    !TICKET_ID.test(id)
      ? `${id} is not a ticket id like FH-42.`
      : id === self
        ? "A ticket cannot depend on itself."
        : undefined;
  return {
    ...listProperty((detail) => detail.dependsOn, ticketId),
    check: (draft: ChipList) => {
      const typed = ticketId(draft.text);
      return typed ? check(typed) : undefined;
    },
    control: (value: ChipList, set: (value: ChipList) => void) => (
      <TicketPicker value={value} onChange={set} check={check} load={load} />
    ),
  };
};

function TicketPicker({
  value,
  onChange,
  check,
  load,
}: {
  value: ChipList;
  onChange(value: ChipList): void;
  check(id: string): string | undefined;
  load?: TicketEdits["tickets"];
}) {
  const [tickets, setTickets] = useState<{ id: string; title: string }[]>([]);
  // The project's tickets are loaded once, as the picker opens.
  const loader = useRef(load);
  useEffect(() => {
    if (!loader.current) return;
    const abort = new AbortController();
    loader.current(abort.signal).then(setTickets, () => undefined);
    return () => abort.abort();
  }, []);
  return (
    <ChipInput
      label="Depends on"
      list={value}
      onChange={onChange}
      addLabel="Add a ticket"
      removeLabel={(id) => `Remove ${id}`}
      normalize={ticketId}
      check={check}
      suggestions={tickets
        .filter((ticket) => !value.values.includes(ticket.id))
        .map((ticket) => ({ value: ticket.id, label: ticket.title }))}
      placeholder="FH-42, then Enter"
    />
  );
}
