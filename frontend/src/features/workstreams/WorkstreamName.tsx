import { useRef } from "react";

import type { WorkstreamBrief, WorkstreamDetail } from "../../api/board";
import type { Renamed, Saved } from "../../api/edit";
import { takenWorkstream } from "../../model/lines";
import { titleLine } from "../../model/sectionEdit";
import {
  EditPencil,
  editGroup,
  InPlaceEditor,
  useInPlaceItem,
} from "../editing/InPlace";

// nameIn is a workstream's name in its file's text: its H1.
const nameIn = (file: string) => titleLine(file).replace(/^#\s*/, "").trim();

// Editable is the workstream as its in-place items take it, named by its
// title, which the conflict dialog says.
export type Editable = WorkstreamDetail & { id: string };

// SendName saves a new name on base and answers with the slug after it.
export type SendName = (base: string, name: string) => Promise<Renamed>;

// Siblings are the project's workstreams, by which a name another of them
// has is named at the field before it is sent (FH-163).
export interface Siblings {
  displayName: string;
  workstreams: WorkstreamBrief[];
}

// Named is a name saved: the name, the slug after it and the save.
export interface Named {
  name: string;
  slug: string;
  saved: Saved;
}

// WorkstreamName is the workstream page's h1, its name; where the board
// can save, the name edits in place (FH-167) like a ticket's title: its
// pencil, a double-click, Save or Ctrl/Cmd+Enter, Escape. Saving renames
// the slug to the one the name makes (the server's), so every reference
// follows; onNamed hears the name saved and the slug after it, for the
// page's route to follow, or nothing when the page reloaded instead (the
// conflict dialog's Reload). A name another workstream's slug or name has
// is refused with the reason beside the field, which stays open; one
// that another of siblings has is named there without sending it.
export function WorkstreamName({
  detail,
  send,
  onNamed,
  siblings,
}: {
  detail: Editable;
  send?: SendName;
  onNamed(named?: Named): void;
  siblings?: Siblings;
}) {
  const item = useInPlaceItem("name", "name");
  const edit = send && item.editable ? send : undefined;
  // renamed is the last save's answer, for onSaved to follow its slug.
  const renamed = useRef<{ slug: string; name: string } | null>(null);
  const heading = "text-3xl leading-tight display-cut";
  if (edit && item.open)
    return (
      <>
        <h1 className="sr-only">{detail.title}</h1>
        <InPlaceEditor
          item={item}
          name="name"
          kind="workstream"
          className="mt-1 w-full"
          detail={detail}
          read={(workstream: Editable) => workstream.title}
          isDirty={(draft, base) => draft.trim() !== base.trim()}
          check={(value) => {
            if (!value.trim()) return "A workstream needs a name.";
            const taken =
              siblings &&
              takenWorkstream(siblings.workstreams, value, detail.slug);
            return taken
              ? `${siblings.displayName} already has a workstream called ${taken.title}. Choose another name.`
              : undefined;
          }}
          send={async (base, value) => {
            const name = value.trim();
            const saved = await edit(base, name);
            renamed.current = { slug: saved.slug, name };
            return saved;
          }}
          mine={(value) => `# ${value.trim()}`}
          theirs={titleLine}
          readFile={nameIn}
          unsaved={`Your new name for ${detail.title} was not saved`}
          onSaved={(saved) => {
            const answer = renamed.current;
            renamed.current = null;
            onNamed(saved && answer ? { ...answer, saved } : undefined);
          }}
        >
          {(value, set) => (
            <input
              aria-label="Name"
              value={value}
              maxLength={300}
              onChange={(event) => set(event.target.value)}
              className="w-full rounded-control border border-rule bg-card px-2.5 py-1 text-2xl leading-tight text-ink display-cut transition-colors hover:border-ink-muted"
            />
          )}
        </InPlaceEditor>
      </>
    );
  if (!edit) return <h1 className={heading}>{detail.title}</h1>;
  return (
    <div className={`${editGroup} flex min-w-0 items-start gap-2`}>
      {/* A double-click is a shortcut; the pencil is the control. */}
      <h1 className={`min-w-0 ${heading}`} onDoubleClick={item.onDoubleClick}>
        {detail.title}
      </h1>
      <span className="mt-1">
        <EditPencil item={item} name="name" />
      </span>
    </div>
  );
}
