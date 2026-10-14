import { type ReactNode, useCallback, useEffect } from "react";

import {
  fetchWorkstream,
  type TicketRef,
  type WorkstreamDetail,
} from "../../api/board";
import type { AuthenticatedFetch } from "../../api/client";
import {
  nameWorkstream,
  type Saved,
  saveWorkstreamSection,
} from "../../api/edit";
import { Icon } from "../../components/Icon";
import { LineBullet } from "../../components/LineBullet";
import { LoadError } from "../../components/LoadError";
import { Markdown } from "../../components/Markdown";
import { StaleNote } from "../../components/StaleNote";
import { TicketLinksInPlace } from "../../components/TicketLink";
import type { Line } from "../../model/lines";
import type { LinkContext } from "../../model/markdown";
import {
  type WorkstreamSection,
  workstreamSections,
} from "../../model/workstreamSections";
import { useResource } from "../../state/useResource";
import {
  AddItem,
  EditPencil,
  editGroup,
  InPlaceEditing,
  useInPlaceItem,
} from "../editing/InPlace";
import { InPlaceSection } from "../editing/InPlaceSection";
import type { EditPlace } from "../editing/lateSaves";
import type { Editing } from "../editing/useEditing";
import { TransitLine } from "./TransitLine";
import {
  type Editable,
  type Named,
  type SendName,
  WorkstreamName,
} from "./WorkstreamName";
import { LineNotes, LineStatus, routeSurface, Served } from "./WorkstreamsView";

interface WorkstreamPageProps {
  workstream: { project: string; slug: string };
  fetcher: AuthenticatedFetch;
  // lines are the line colours per project and workstream slug.
  lines: Map<string, Map<string, Line>>;
  keys: Set<string>;
  // revision reloads the workstream when the board changes (LIFE-3).
  revision: number;
  // editing enables the in-place edits; absent when read-only.
  editing?: Editing;
  onOpen(ticket: TicketRef): void;
  // onRenamed hears the workstream's new slug after its name was saved
  // with one (FH-167), for the route to follow it.
  onRenamed?(slug: string): void;
  // backLink is the project's Workstreams view and the project's name.
  backLink: { href: string; label: string };
}

// WorkstreamPage is a workstream's page (FH-139, ui-layout.md §4): its
// head and line as on its route card, then its sections (Goal, Scope,
// Success Criteria, Notes and any of its own), each edited in place in the
// formatted editor where the board can save. The title is the page's h1
// and each section an h2; the browser tab is named after the workstream.
export function WorkstreamPage({
  workstream: ref,
  fetcher,
  lines,
  keys,
  revision,
  editing,
  onOpen,
  onRenamed,
  backLink,
}: WorkstreamPageProps) {
  const load = useCallback(
    (signal: AbortSignal) =>
      fetchWorkstream(fetcher, ref.project, ref.slug, signal),
    [fetcher, ref.project, ref.slug],
  );
  const resource = useResource(load, `${ref.project}/${ref.slug}`, revision);
  const detail = resource.status === "ready" ? resource.data : undefined;
  const title = detail?.title ?? ref.slug;
  useEffect(() => {
    const before = document.title;
    document.title = `${title} · Flashheart`;
    return () => {
      document.title = before;
    };
  }, [title]);
  const saved = (result: Saved | undefined, what: string) => {
    resource.reload();
    if (result && editing)
      editing.notify({ text: `${what} ${title}.`, details: result.warnings });
  };
  const save =
    editing && detail?.hash
      ? (base: string, section: string, content: string) =>
          saveWorkstreamSection(
            fetcher,
            ref.project,
            ref.slug,
            base,
            section,
            content,
          )
      : undefined;
  const sendName: SendName | undefined =
    editing && detail?.hash
      ? (base, name) =>
          nameWorkstream(fetcher, ref.project, ref.slug, base, name)
      : undefined;
  // named follows a saved name: to the new slug's page, which loads it,
  // or a reload of this one.
  const named = (result?: Named) => {
    if (!result || result.slug === ref.slug) resource.reload();
    if (result && editing)
      editing.notify({ text: `Renamed ${title} to ${result.name}.` });
    if (result && result.slug !== ref.slug) onRenamed?.(result.slug);
  };
  return (
    <TicketLinksInPlace>
      <div className="mx-auto w-full max-w-[80rem] px-4 pt-4 pb-[30vh] md:px-8">
        <nav aria-label="Workstream" className="flex h-9 items-center text-sm">
          <a
            href={backLink.href}
            className="inline-flex items-center gap-1.5 rounded-control font-medium text-ink-muted hover:text-ink hover:underline"
          >
            <Icon name="lines" size={14} />
            Show in the {backLink.label} workstreams
          </a>
        </nav>
        {resource.status === "loading" ? <WorkstreamPageSkeleton /> : null}
        {resource.status === "error" ? (
          <>
            <h1 className="sr-only">{ref.slug}</h1>
            <LoadError
              what="This workstream could not be loaded"
              error={resource.error}
              onRetry={resource.reload}
              className="py-6"
            />
          </>
        ) : null}
        {detail ? (
          <>
            {resource.error ? (
              <StaleNote error={resource.error} className="pb-4" />
            ) : null}
            <LoadedPage
              place={{
                kind: "workstream",
                project: ref.project,
                slug: ref.slug,
              }}
              detail={detail}
              line={lines.get(ref.project)?.get(ref.slug)}
              context={{ project: ref.project, ticket: "" }}
              keys={keys}
              onOpen={onOpen}
              save={save}
              onSaved={saved}
              sendName={sendName}
              onNamed={named}
            />
          </>
        ) : null}
      </div>
    </TicketLinksInPlace>
  );
}

type SaveSection = (
  base: string,
  section: string,
  content: string,
) => Promise<Saved>;

function LoadedPage({
  place,
  detail,
  line,
  context,
  keys,
  onOpen,
  save,
  onSaved,
  sendName,
  onNamed,
}: {
  // place is the page, for an edit whose save failed after it was left
  // (FH-156).
  place: EditPlace;
  detail: WorkstreamDetail;
  line?: Line;
  context: LinkContext;
  keys: Set<string>;
  onOpen(ticket: TicketRef): void;
  save?: SaveSection;
  onSaved(saved: Saved | undefined, what: string): void;
  sendName?: SendName;
  onNamed(named?: Named): void;
}) {
  const editable: Editable = { ...detail, id: detail.title };
  return (
    <InPlaceEditing place={place}>
      <header className="flex flex-wrap items-center gap-x-4 gap-y-1 pt-1 pb-6">
        <LineBullet line={line} size="xl" />
        <div className="flex min-w-0 flex-1 basis-64 flex-col gap-0.5">
          <WorkstreamName detail={editable} send={sendName} onNamed={onNamed} />
          <LineStatus workstream={detail} />
        </div>
        <span className="ml-auto">
          <Served workstream={detail} />
        </span>
      </header>
      <section
        aria-label="Stations"
        style={routeSurface(line)}
        className="rounded-panel border border-rule bg-(--route-surface) px-5 pt-3 pb-3 shadow-card"
      >
        <LineNotes workstream={detail} />
        <TransitLine workstream={detail} line={line} onOpen={onOpen} />
      </section>
      <div className="mt-10 flex max-w-3xl flex-col gap-8">
        {workstreamSections(detail.body)
          .filter((section) => !section.empty || save)
          .map((section) => (
            <SectionView
              key={section.title}
              section={section}
              detail={editable}
              context={context}
              keys={keys}
              save={save}
              onSaved={onSaved}
            />
          ))}
      </div>
    </InPlaceEditing>
  );
}

// itemName is a section in its edit's words: a template section's in
// lower case ("Edit goal", "Add success criteria"), one of the
// workstream's own as written.
const itemName = (section: WorkstreamSection) =>
  section.template ? section.title.toLowerCase() : section.title;

// SectionView is one section under its h2, its pencil beside the heading;
// open, the formatted editor in its place. An empty section shows as Add
// <section>.
function SectionView({
  section,
  detail,
  context,
  keys,
  save,
  onSaved,
}: {
  section: WorkstreamSection;
  detail: Editable;
  context: LinkContext;
  keys: Set<string>;
  save?: SaveSection;
  onSaved(saved: Saved | undefined, what: string): void;
}) {
  const name = itemName(section);
  const item = useInPlaceItem(`section-${section.title}`, name);
  const edit = save && item.editable ? save : undefined;
  const headingId = `ws-section-${section.title.toLowerCase().replace(/[^a-z0-9]+/g, "-")}`;
  const labelled = (body: ReactNode, action?: ReactNode) => (
    <section aria-labelledby={headingId} className={action ? editGroup : ""}>
      <div className="mb-2 flex items-center gap-2">
        <h2 id={headingId} className="flex-1 text-md heading-cut">
          {section.title}
        </h2>
        {action ? <span className="-my-1.5">{action}</span> : null}
      </div>
      {body}
    </section>
  );
  if (edit && item.open)
    return labelled(
      <InPlaceSection
        item={item}
        name={name}
        title={section.title}
        heading={section.heading}
        kind="workstream"
        detail={detail}
        body={(workstream: Editable) => workstream.body}
        send={edit}
        onSaved={onSaved}
        context={context}
        keys={keys}
      />,
    );
  if (edit && section.empty) return <AddItem item={item} name={name} />;
  return labelled(
    edit ? (
      // biome-ignore lint/a11y/noStaticElementInteractions: a double-click is a shortcut; the pencil is the control.
      <div onDoubleClick={item.onDoubleClick}>
        <Markdown context={context} keys={keys}>
          {section.markdown}
        </Markdown>
      </div>
    ) : (
      <Markdown context={context} keys={keys}>
        {section.markdown}
      </Markdown>
    ),
    edit ? <EditPencil item={item} name={name} /> : undefined,
  );
}

// WorkstreamPageSkeleton holds the page's place while it loads
// (ui-layout.md §8): the head, a track and two sections.
function WorkstreamPageSkeleton() {
  return (
    <div
      aria-hidden="true"
      data-skeleton="workstream-page"
      className="flex flex-col gap-6 pt-1"
    >
      <div className="flex items-center gap-4">
        <div className="size-10 shrink-0 animate-pulse rounded-full bg-well" />
        <div className="h-8 w-1/3 animate-pulse rounded-card bg-well" />
      </div>
      <div className="h-32 animate-pulse rounded-panel bg-well" />
      <div className="h-5 w-24 animate-pulse rounded-card bg-well" />
      <div className="h-16 animate-pulse rounded-card bg-well" />
    </div>
  );
}
