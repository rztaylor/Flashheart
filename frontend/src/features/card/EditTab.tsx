import { type FormEvent, useEffect, useRef, useState } from "react";
import {
  STATUSES,
  type TicketDetail,
  type WorkstreamBrief,
} from "../../api/board";
import type { AuthenticatedFetch } from "../../api/client";
import { patchTicket, type Saved, saveRaw } from "../../api/edit";
import { Button } from "../../components/Button";
import { FormField, Select, TextArea, TextInput } from "../../components/Field";
import { SegmentedControl } from "../../components/SegmentedControl";
import { priorityMark } from "../../components/TicketMarks";
import {
  changes,
  fieldsText,
  formOf,
  type TicketForm,
} from "../../model/ticketForm";
import { PRIORITIES, TICKET_TYPES } from "../../model/tickets";
import { ConflictDialog } from "../editing/ConflictDialog";
import { useHeldDraft, useLateDraft } from "../editing/InPlace";
import { notSaved } from "../editing/lateSaves";
import { type DraftLeft, useTicketDraft } from "../editing/useTicketDraft";

// Draft is both editors' text: the typed fields and the raw file. They share
// one base, so a save from either adopts the saved version in both.
interface Draft {
  form: TicketForm;
  raw: string;
}

const draftOf = (detail: TicketDetail): Draft => ({
  form: formOf(detail),
  raw: detail.raw,
});

const fieldEdits = (draft: Draft, base: Draft) =>
  changes(draft.form, base.form);

const isDirty = (draft: Draft, base: Draft) =>
  Object.keys(fieldEdits(draft, base)).length > 0 || draft.raw !== base.raw;

// firstField is the tab's first text field or select, its Title.
const firstField = (frame: HTMLElement | null) =>
  frame?.querySelector<HTMLElement>(
    "input:not([type=radio]), select, textarea",
  );

// EditTab edits a ticket's frontmatter with typed controls, or the whole
// file in a raw editor (EDIT-6). Every save carries the hash of the version
// the draft started from (useTicketDraft); a stale hash opens the conflict
// dialog (EDIT-7). Unsaved changes are a held draft of the panel's in-place
// editing (FH-138): leaving the panel asks about them, Discard drops them,
// and Keep editing shows the tab (onShow) with focus back in it. A save on
// its way is not unsaved; one that fails after the panel closed is a late
// save, which Back to the edit brings back to this tab (FH-156).
export function EditTab({
  detail,
  fetcher,
  workstreams,
  onSaved,
  onShow,
  autoFocus = false,
}: {
  detail: TicketDetail;
  fetcher: AuthenticatedFetch;
  workstreams: WorkstreamBrief[];
  onSaved(saved: Saved | undefined, what: string): void;
  onShow(): void;
  // autoFocus puts focus in the first field as the tab mounts: the panel
  // opened ready to edit, after New ticket (FH-17).
  autoFocus?: boolean;
}) {
  const draft = useTicketDraft({
    detail,
    read: draftOf,
    isDirty,
    onSaved,
    // A save that fails after the panel closed is held, and the toast
    // offers the way back (FH-156).
    left: (left) =>
      holdLate(
        notSaved(
          `Your changes to ${detail.id} were not saved`,
          detail.id,
          left.conflict ? { conflict: true } : { error: left.error },
        ),
        left,
      ),
  });
  const [mode, setMode] = useState<"fields" | "raw">("fields");
  const frame = useRef<HTMLDivElement>(null);
  // lastFocus is the control last focused in the tab, where Keep editing
  // returns; returning counts the returns asked for, each focusing once
  // the tab shows.
  const lastFocus = useRef<HTMLElement | null>(null);
  const [returning, setReturning] = useState(0);
  const showTab = () => {
    onShow();
    setReturning((count) => count + 1);
  };
  // A save waiting for its answer is not unsaved (FH-156): closing the
  // panel then asks nothing.
  useHeldDraft(
    "edit-tab",
    "ticket",
    draft.dirty && !draft.busy,
    () => draft.set(draft.base.value),
    showTab,
  );
  const holdLate = useLateDraft<DraftLeft<Draft>>(
    "edit-tab",
    "ticket",
    (left) => {
      draft.restore(left);
      showTab();
    },
    "edit",
  );
  // A conflict needs a decision in its dialog, and a failure its reason,
  // which a hidden tab would hide: a save that meets one after another tab
  // was chosen shows the Edit tab again.
  const needsLook = draft.conflict !== null || draft.error !== "";
  // biome-ignore lint/correctness/useExhaustiveDependencies: shows the tab when a conflict or failure arrives.
  useEffect(() => {
    if (needsLook) onShow();
  }, [needsLook]);
  useEffect(() => {
    const element = frame.current;
    const remember = (event: FocusEvent) => {
      if (event.target instanceof HTMLElement) lastFocus.current = event.target;
    };
    element?.addEventListener("focusin", remember);
    return () => element?.removeEventListener("focusin", remember);
  }, []);
  // biome-ignore lint/correctness/useExhaustiveDependencies: focuses once, as the tab mounts.
  useEffect(() => {
    if (autoFocus) firstField(frame.current)?.focus();
  }, []);
  useEffect(() => {
    if (returning === 0) return;
    const last = lastFocus.current;
    const target =
      last?.isConnected && frame.current?.contains(last)
        ? last
        : firstField(frame.current);
    target?.focus();
  }, [returning]);
  const { form, raw } = draft.value;
  const base = draft.base.value;
  const edits = fieldEdits(draft.value, base);
  const dirty = Object.keys(edits).length > 0;
  const rawDirty = raw !== base.raw;
  const { busy, error } = draft;

  const saveFields = (event: FormEvent) => {
    event.preventDefault();
    void draft.save({
      send: (hash) => patchTicket(fetcher, detail.id, hash, edits),
      what: "Saved",
      mine: fieldsText(edits),
    });
  };
  const saveText = () =>
    draft.save({
      send: (hash) => saveRaw(fetcher, detail.id, hash, raw),
      what: "Saved the file",
      mine: raw,
    });

  const set = (key: keyof TicketForm) => (value: string) =>
    draft.set((current) => ({
      ...current,
      form: { ...current.form, [key]: value },
    }));
  const knownWorkstream = workstreams.some(
    (item) => item.slug === form.workstream,
  );

  return (
    <div ref={frame} className="flex flex-col gap-4">
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
            <FormField label="Status">
              <Select value={form.status} onChange={set("status")}>
                {STATUSES.map((status) => (
                  <option key={status.id} value={status.id}>
                    {status.title}
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
              <Select
                value={form.priority}
                onChange={set("priority")}
                leading={priorityMark(form.priority)}
              >
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
          <FormField
            label="Pull request"
            hint="Its web address; an http(s) address shows as a link."
          >
            <TextInput
              value={form.pr}
              onChange={set("pr")}
              spellCheck={false}
              placeholder="https://github.com/owner/repo/pull/42"
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
              onClick={() =>
                draft.set((current) => ({ ...current, form: base.form }))
              }
            >
              Discard
            </Button>
          </div>
        </form>
      ) : (
        <div className="flex flex-col gap-3">
          <TextArea
            value={raw}
            onChange={(text) =>
              draft.set((current) => ({ ...current, raw: text }))
            }
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
              onClick={() =>
                draft.set((current) => ({ ...current, raw: base.raw }))
              }
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
      {draft.conflict ? (
        <ConflictDialog
          id={detail.id}
          mine={draft.conflict.mine}
          current={draft.conflict.current.content}
          onCancel={draft.cancel}
          onReload={draft.reload}
          onOverwrite={draft.overwrite}
        />
      ) : null}
    </div>
  );
}
