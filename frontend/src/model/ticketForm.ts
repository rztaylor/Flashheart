import { statusWord, type TicketDetail } from "../api/board";
import type { FieldValue } from "../api/edit";

// TicketForm is a ticket's frontmatter fields as text, the way typed
// controls edit them (EDIT-6): lists are comma-separated.
export interface TicketForm {
  title: string;
  status: string;
  type: string;
  priority: string;
  created: string;
  branch: string;
  pr: string;
  workstream: string;
  "depends-on": string;
  tags: string;
}

// TicketFormSource is the part of a ticket the form reads.
export type TicketFormSource = Pick<
  TicketDetail,
  | "title"
  | "column"
  | "rejected"
  | "type"
  | "priority"
  | "created"
  | "branch"
  | "pr"
  | "workstream"
  | "dependsOn"
  | "tags"
>;

const lists: (keyof TicketForm)[] = ["depends-on", "tags"];

// formOf is a ticket's fields as the form shows them.
export function formOf(ticket: TicketFormSource): TicketForm {
  return {
    title: ticket.title,
    status: statusWord(ticket),
    type: ticket.type,
    priority: ticket.priority,
    created: ticket.created,
    branch: ticket.branch,
    pr: ticket.pr,
    workstream: ticket.workstream,
    "depends-on": ticket.dependsOn.join(", "),
    tags: ticket.tags.join(", "),
  };
}

// split reads a comma-separated list, dropping empty items.
export const split = (value: string) =>
  value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);

// changes lists the fields that differ from the saved ticket, as the API
// expects them (trimmed; lists as arrays).
export function changes(
  form: TicketForm,
  saved: TicketForm,
): Record<string, FieldValue> {
  const out: Record<string, FieldValue> = {};
  for (const key of Object.keys(form) as (keyof TicketForm)[]) {
    if (form[key].trim() === saved[key].trim()) continue;
    out[key] = lists.includes(key) ? split(form[key]) : form[key].trim();
  }
  return out;
}

// fieldsText shows field changes one per line, lists in brackets: "your
// version" of a fields save in the conflict dialog (EDIT-7).
export const fieldsText = (fields: Record<string, FieldValue>) =>
  Object.entries(fields)
    .map(
      ([key, value]) =>
        `${key}: ${Array.isArray(value) ? `[${value.join(", ")}]` : value}`,
    )
    .join("\n");
