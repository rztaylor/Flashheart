import type { Saved } from "../../api/edit";
import type { LinkContext } from "../../model/markdown";
import { sameSection, sectionSource } from "../../model/sectionEdit";
import type { Upload } from "../../model/uploads";
import { MarkdownField } from "../editor/MarkdownField";
import { InPlaceEditor, type InPlaceItem } from "./InPlace";

// InPlaceSection edits one level-2 section of a file's markdown body, by
// its heading (an empty or missing section starts empty), in the
// formatted editor (EDIT-12): the text it saves is the text as edited, so
// an untouched section saves nothing, and a save that meets a newer file
// which left this section as it was is saved again on it. A ticket's
// sections (FH-16) and a workstream's (FH-139) both edit through it.
export function InPlaceSection<D extends { id: string; hash: string }>({
  item,
  name,
  title,
  heading,
  detail,
  body,
  send,
  onSaved,
  context,
  keys,
  upload,
  kind,
}: {
  item: InPlaceItem;
  // name is the item in its edit's words ("description"); title is the
  // editor's accessible name ("Description").
  name: string;
  title: string;
  // heading is the section's name in the file.
  heading: string;
  detail: D;
  // body is the markdown body of a version of the file.
  body(detail: D): string;
  send(base: string, heading: string, content: string): Promise<Saved>;
  onSaved(saved: Saved | undefined, what: string): void;
  context: LinkContext;
  keys: Set<string>;
  // upload attaches files dropped or pasted into a ticket (FH-18).
  upload?: Upload;
  kind?: "ticket" | "workstream";
}) {
  return (
    <InPlaceEditor
      item={item}
      name={name}
      kind={kind}
      detail={detail}
      read={(version: D) => sectionSource(body(version), heading)}
      readFile={(file) => sectionSource(file, heading)}
      isDirty={(draft, base) => !sameSection(draft, base)}
      send={(base, value) => send(base, heading, value)}
      mine={(value) => value}
      theirs={(file) => sectionSource(file, heading)}
      onSaved={onSaved}
    >
      {(value, set) => (
        <MarkdownField
          label={title}
          value={value}
          onChange={set}
          context={context}
          keys={keys}
          upload={upload}
        />
      )}
    </InPlaceEditor>
  );
}
