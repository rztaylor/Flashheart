import { type FormEvent, useEffect, useRef, useState } from "react";
import {
  COLUMNS,
  type TicketDetail,
  type WorkstreamBrief,
} from "../../api/board";
import type { AuthenticatedFetch } from "../../api/client";
import {
  type Conflict,
  conflictOf,
  type FieldValue,
  patchTicket,
  type Saved,
  saveRaw,
} from "../../api/edit";
import { Button } from "../../components/Button";
import { FormField, Select, TextArea, TextInput } from "../../components/Field";
import { SegmentedControl } from "../../components/SegmentedControl";
import { PRIORITIES, TICKET_TYPES } from "../../model/tickets";
import { ConflictDialog } from "../editing/ConflictDialog";

interface Form {
  title: string;
  status: string;
  type: string;
  priority: string;
  created: string;
  branch: string;
  workstream: string;
  "depends-on": string;
  tags: string;
}

const lists: (keyof Form)[] = ["depends-on", "tags"];

function baseOf(detail: TicketDetail) {
  return { hash: detail.hash, form: formOf(detail), raw: detail.raw };
}

function formOf(detail: TicketDetail): Form {
  return {
    title: detail.title,
    status: detail.column,
    type: detail.type,
    priority: detail.priority,
    created: detail.created,
    branch: detail.branch,
    workstream: detail.workstream,
    "depends-on": detail.dependsOn.join(", "),
    tags: detail.tags.join(", "),
  };
}

const split = (value: string) =>
  value
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);

// changes lists the fields that differ from the saved ticket, as the API
// expects them (lists as arrays).
function changes(form: Form, saved: Form): Record<string, FieldValue> {
  const out: Record<string, FieldValue> = {};
  for (const key of Object.keys(form) as (keyof Form)[]) {
    if (form[key].trim() === saved[key].trim()) continue;
    out[key] = lists.includes(key) ? split(form[key]) : form[key].trim();
  }
  return out;
}

type Pending =
  | { kind: "fields"; fields: Record<string, FieldValue>; conflict: Conflict }
  | { kind: "raw"; content: string; conflict: Conflict };

// EditTab edits a ticket's frontmatter with typed controls, or the whole
// file in a raw editor (EDIT-6). Every save carries the hash the panel read;
// a stale hash opens the conflict dialog (EDIT-7).
export function EditTab({
  detail,
  fetcher,
  workstreams,
  onSaved,
}: {
  detail: TicketDetail;
  fetcher: AuthenticatedFetch;
  workstreams: WorkstreamBrief[];
  onSaved(saved: Saved | undefined, what: string): void;
}) {
  // base is the version of the file the drafts started from. Edits are
  // measured against it and saved with its hash, so a change on disk since
  // then is a conflict, never a silent overwrite (STO-3, EDIT-7).
  const [base, setBase] = useState(() => baseOf(detail));
  const [mode, setMode] = useState<"fields" | "raw">("fields");
  const [form, setForm] = useState<Form>(base.form);
  const [raw, setRaw] = useState(base.raw);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [pending, setPending] = useState<Pending | null>(null);
  // adopt takes the next version of the file after a save or a reload.
  const adopt = useRef(false);
  const latest = useRef(detail);
  latest.current = detail;
  const edits = changes(form, base.form);
  const dirty = Object.keys(edits).length > 0;
  const rawDirty = raw !== base.raw;

  const reset = (from: TicketDetail) => {
    const next = baseOf(from);
    setBase(next);
    setForm(next.form);
    setRaw(next.raw);
  };
  // Follow the file while nothing is being edited, and after a save.
  // biome-ignore lint/correctness/useExhaustiveDependencies: runs when the file itself changes.
  useEffect(() => {
    if (detail.hash === base.hash) return;
    if (adopt.current || (!dirty && !rawDirty)) {
      adopt.current = false;
      reset(detail);
    }
  }, [detail.hash]);

  const run = async (
    save: () => Promise<Saved>,
    what: string,
    onConflict: (c: Conflict) => Pending,
  ) => {
    setBusy(true);
    setError("");
    try {
      const result = await save();
      // Live updates may already have brought the saved version.
      if (latest.current.hash === result.hash) reset(latest.current);
      else adopt.current = true;
      onSaved(result, what);
    } catch (caught) {
      const conflict = conflictOf(caught);
      if (conflict) setPending(onConflict(conflict));
      else setError(caught instanceof Error ? caught.message : "Not saved");
    } finally {
      setBusy(false);
    }
  };

  const saveFields = (event: FormEvent) => {
    event.preventDefault();
    void run(
      () => patchTicket(fetcher, detail.id, base.hash, edits),
      "Saved",
      (conflict) => ({ kind: "fields", fields: edits, conflict }),
    );
  };
  const saveText = () =>
    run(
      () => saveRaw(fetcher, detail.id, base.hash, raw),
      "Saved the file",
      (conflict) => ({ kind: "raw", content: raw, conflict }),
    );

  const set = (key: keyof Form) => (value: string) =>
    setForm((current) => ({ ...current, [key]: value }));
  const knownWorkstream = workstreams.some(
    (item) => item.slug === form.workstream,
  );

  return (
    <div className="flex flex-col gap-4">
      <SegmentedControl<"fields" | "raw">
        label="Edit as"
        value={mode}
        onChange={setMode}
        options={[
          { value: "fields", label: "Fields" },
          { value: "raw", label: "Raw file" },
        ]}
      />
      {mode === "fields" ? (
        <form onSubmit={saveFields} className="flex flex-col gap-3">
          <FormField label="Title">
            <TextInput
              value={form.title}
              onChange={set("title")}
              required
              maxLength={300}
            />
          </FormField>
          <div className="grid grid-cols-3 gap-3">
            <FormField label="Column">
              <Select value={form.status} onChange={set("status")}>
                {COLUMNS.map((column) => (
                  <option key={column.id} value={column.id}>
                    {column.title}
                  </option>
                ))}
              </Select>
            </FormField>
            <FormField label="Type">
              <Select value={form.type} onChange={set("type")}>
                {[...new Set([...TICKET_TYPES, form.type].filter(Boolean))].map(
                  (item) => (
                    <option key={item}>{item}</option>
                  ),
                )}
              </Select>
            </FormField>
            <FormField label="Priority">
              <Select value={form.priority} onChange={set("priority")}>
                {[
                  ...new Set([...PRIORITIES, form.priority].filter(Boolean)),
                ].map((item) => (
                  <option key={item}>{item}</option>
                ))}
              </Select>
            </FormField>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <FormField label="Workstream">
              <Select value={form.workstream} onChange={set("workstream")}>
                <option value="">None</option>
                {workstreams.map((item) => (
                  <option key={item.slug} value={item.slug}>
                    {item.title}
                  </option>
                ))}
                {form.workstream && !knownWorkstream ? (
                  <option value={form.workstream}>
                    {form.workstream} (missing)
                  </option>
                ) : null}
              </Select>
            </FormField>
            <FormField label="Created">
              <TextInput
                type="date"
                value={form.created}
                onChange={set("created")}
              />
            </FormField>
          </div>
          <FormField label="Depends on" hint="Ticket ids, separated by commas.">
            <TextInput
              value={form["depends-on"]}
              onChange={set("depends-on")}
              spellCheck={false}
              placeholder="FH-12, NG-3"
            />
          </FormField>
          <FormField
            label="Tags"
            hint="Separated by commas; later-possibility marks ideas."
          >
            <TextInput
              value={form.tags}
              onChange={set("tags")}
              spellCheck={false}
            />
          </FormField>
          <FormField label="Branch">
            <TextInput
              value={form.branch}
              onChange={set("branch")}
              spellCheck={false}
              className="font-mono text-xs"
            />
          </FormField>
          <div className="flex items-center gap-3">
            <Button variant="primary" type="submit" disabled={!dirty || busy}>
              {busy ? "Saving…" : "Save changes"}
            </Button>
            <Button
              variant="quiet"
              disabled={!dirty || busy}
              onClick={() => setForm(base.form)}
            >
              Discard
            </Button>
          </div>
        </form>
      ) : (
        <div className="flex flex-col gap-3">
          <TextArea
            value={raw}
            onChange={setRaw}
            rows={22}
            mono
            spellCheck={false}
            aria-label="Ticket file"
          />
          <p className="text-xs text-ink-muted">
            The whole file as it is on disk. A save that breaks the frontmatter
            leaves the ticket needing repair until it is fixed.
          </p>
          <div className="flex items-center gap-3">
            <Button
              variant="primary"
              disabled={!rawDirty || busy}
              onClick={() => void saveText()}
            >
              {busy ? "Saving…" : "Save file"}
            </Button>
            <Button
              variant="quiet"
              disabled={!rawDirty || busy}
              onClick={() => setRaw(base.raw)}
            >
              Discard
            </Button>
          </div>
        </div>
      )}
      {error ? (
        <p role="alert" className="text-sm text-danger">
          {error}
        </p>
      ) : null}
      {pending ? (
        <ConflictDialog
          id={detail.id}
          mine={
            pending.kind === "raw"
              ? pending.content
              : Object.entries(pending.fields)
                  .map(
                    ([key, value]) =>
                      `${key}: ${Array.isArray(value) ? `[${value.join(", ")}]` : value}`,
                  )
                  .join("\n")
          }
          current={pending.conflict.content}
          onCancel={() => setPending(null)}
          onReload={() => {
            setPending(null);
            // Take the version on disk now, and any newer one the reload
            // brings.
            reset(detail);
            adopt.current = true;
            onSaved(undefined, "Reloaded");
          }}
          onOverwrite={() => {
            const current = pending;
            setPending(null);
            if (current.kind === "raw") {
              void run(
                () =>
                  saveRaw(
                    fetcher,
                    detail.id,
                    current.conflict.hash,
                    current.content,
                  ),
                "Saved the file",
                (conflict) => ({ ...current, conflict }),
              );
            } else {
              void run(
                () =>
                  patchTicket(
                    fetcher,
                    detail.id,
                    current.conflict.hash,
                    current.fields,
                  ),
                "Saved",
                (conflict) => ({ ...current, conflict }),
              );
            }
          }}
        />
      ) : null}
    </div>
  );
}
