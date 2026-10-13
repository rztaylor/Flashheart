import { type ReactNode, useEffect, useRef, useState } from "react";

import type { Attachment, TicketDetail, TicketRef } from "../../api/board";
import { Button } from "../../components/Button";
import { Icon } from "../../components/Icon";
import { LoadError } from "../../components/LoadError";
import { Markdown } from "../../components/Markdown";
import { SegmentedControl } from "../../components/SegmentedControl";
import { StaleNote } from "../../components/StaleNote";
import { LinkedText, TicketLinksInPlace } from "../../components/TicketLink";
import {
  ACTIVITY_FILTERS,
  type ActivityEntry,
  type ActivityFilter,
  activityOf,
  filterActivity,
} from "../../model/activity";
import { workstreamTitle } from "../../model/lines";
import {
  type ImageRef,
  type LinkContext,
  localFile,
  resolveLink,
} from "../../model/markdown";
import { countRequests, requestsOf, requestWords } from "../../model/requests";
import {
  type PageSection,
  pageSections,
  unreferencedFiles,
} from "../../model/ticketSections";
import {
  AddItem,
  EditPencil,
  editGroup,
  InPlaceEditing,
  useInPlaceItem,
} from "../editing/InPlace";
import { Attachments } from "./Attachments";
import type { CardPanelProps } from "./CardPanel";
import { ChangeRequestList, RequestChangesDialog } from "./ChangeRequests";
import { Checklist } from "./Checklist";
import { CriteriaEditor, SectionEditor, TitleEditor } from "./PageEdits";
import { PropertiesPanel } from "./PropertiesPanel";
import { JumpTo, SectionOutline } from "./SectionOutline";
import { TicketComments } from "./TicketComments";
import { TicketIdentity } from "./TicketHeader";
import { TicketSkeleton } from "./TicketSections";
import {
  FormatWarnings,
  HandoffCallout,
  QuestionsCallout,
  ReasonList,
  RepairNotice,
} from "./TicketTab";
import {
  focusSection,
  sectionElementId,
  useSectionInView,
} from "./useSectionInView";
import { type TicketState, useTicket } from "./useTicket";

interface TicketPageProps
  extends Omit<CardPanelProps, "onStep" | "initialTab"> {
  // boardLink is the ticket open beside its project's board, and the
  // project's name.
  boardLink(project: string): { href: string; label: string };
}

// TicketPage is a ticket's full page (CARD-7, FH-65): from 1280px a
// section outline on the left, the whole ticket as one continuous page in
// the centre and its properties on the right, both staying in view. Below
// 1280px the outline is a Jump to menu, and on narrow screens the
// properties stack above the page. The title is the page's h1, each ticket
// section an h2 and the review's sections h3 under its Review h2. Ticket
// links on it stay in this tab; the browser tab is named after the ticket.
export function TicketPage({
  boardLink,
  ticket: ref,
  fetcher,
  lines,
  workstreams,
  keys,
  onOpen,
  onClose,
  revision,
  editing,
  answers = false,
}: TicketPageProps) {
  const ticket = useTicket({
    fetcher,
    id: ref.id,
    revision,
    editing,
    answers,
  });
  const { detail, resource } = ticket;
  const board = detail ? boardLink(detail.project) : undefined;
  const title = detail ? `${detail.id} · ${detail.title}` : ref.id;
  useEffect(() => {
    const before = document.title;
    document.title = `${title} · Flashheart`;
    return () => {
      document.title = before;
    };
  }, [title]);
  return (
    <TicketLinksInPlace>
      <div className="mx-auto w-full max-w-[94rem] px-4 pt-4 md:px-8">
        <div className="xl:pl-[15.5rem]">
          <nav aria-label="Ticket" className="flex h-9 items-center text-sm">
            {board ? (
              <a
                href={board.href}
                className="inline-flex items-center gap-1.5 rounded-control font-medium text-ink-muted hover:text-ink hover:underline"
              >
                <Icon name="board" size={14} />
                Show on the {board.label} board
              </a>
            ) : null}
          </nav>
          {resource.status === "loading" ? <TicketSkeleton /> : null}
          {resource.status === "error" ? (
            <>
              {/* The page keeps its heading while the ticket cannot load. */}
              <h1 className="sr-only">{ref.id}</h1>
              <LoadError
                what="This ticket could not be loaded"
                error={resource.error}
                onRetry={resource.reload}
                className="py-6"
              />
            </>
          ) : null}
        </div>
        {detail ? (
          <LoadedPage
            detail={detail}
            ticket={ticket}
            lines={lines}
            workstreams={workstreams}
            keys={keys}
            onOpen={onOpen}
            onArchive={
              editing && ticket.editable
                ? () => {
                    void editing.archive(detail);
                    onClose();
                  }
                : undefined
            }
            onMove={
              editing && ticket.editable
                ? (to) => void editing.move(detail, to)
                : undefined
            }
          />
        ) : null}
      </div>
    </TicketLinksInPlace>
  );
}

// LoadedPage lays out a loaded ticket: its header, properties, outline and
// sections.
function LoadedPage({
  detail,
  ticket,
  lines,
  workstreams,
  keys,
  onOpen,
  onMove,
  onArchive,
}: {
  detail: TicketDetail;
  ticket: TicketState;
  onMove?: Parameters<typeof PropertiesPanel>[0]["onMove"];
  onArchive?(): void;
} & Pick<TicketPageProps, "lines" | "workstreams" | "keys" | "onOpen">) {
  // Activity shows All, Comments or History (FH-125); the outline counts
  // what it shows.
  const [filter, setFilter] = useState<ActivityFilter>("all");
  // Request changes (REV-6) opens its dialog from the properties or the
  // change requests' heading, on a ticket in review.
  const [requesting, setRequesting] = useState(false);
  const onRequestChanges =
    ticket.requestChanges && detail.column === "review"
      ? () => setRequesting(true)
      : undefined;
  const activity = filterActivity(activityOf(detail), filter);
  const sections = pageSections(detail, activity.length);
  const shown = sections.filter((section) => !section.empty);
  const page = useRef<HTMLDivElement>(null);
  const [current, setCurrent] = useSectionInView(
    page,
    shown.map((section) => section.id),
  );
  const choose = (id: string) => {
    setCurrent(id);
    focusSection(id);
  };
  const context = { project: detail.project, ticket: detail.id };
  const show = (section: PageSection) => (
    <PageSectionView
      key={section.id}
      section={section}
      detail={detail}
      ticket={ticket}
      keys={keys}
      onOpen={onOpen}
      context={context}
      activity={activity}
      filter={filter}
      onFilter={setFilter}
      onRequestChanges={onRequestChanges}
    />
  );
  const inGroup = (group: PageSection["group"]) =>
    shown.filter((section) => section.group === group);
  const [review, ...reviewSections] = inGroup("review");
  // An empty template section shows in its place as Add <section> where
  // the page can save (FH-66); otherwise only the outline lists it.
  const ticketSections = sections.filter(
    (section) =>
      section.group === "ticket" &&
      (!section.empty || (ticket.edit && section.heading !== undefined)),
  );
  return (
    <InPlaceEditing>
      <div className="grid grid-cols-1 gap-x-10 pb-[30vh] lg:grid-cols-[minmax(0,1fr)_19rem] xl:grid-cols-[13rem_minmax(0,1fr)_20rem]">
        <header className="pt-1 pb-6 lg:col-span-2 xl:col-span-2 xl:col-start-2">
          <PageTitle detail={detail} ticket={ticket} />
        </header>
        <div className="mb-6 lg:col-start-2 lg:row-start-2 lg:mb-0 xl:col-start-3">
          <div className="lg:sticky lg:top-6">
            <PropertiesPanel
              detail={detail}
              line={lines.get(detail.project)?.get(detail.workstream)}
              workstreamTitle={workstreamTitle(
                workstreams,
                detail.project,
                detail.workstream,
              )}
              workstreams={workstreams.get(detail.project)}
              onMove={onMove}
              onArchive={onArchive}
              onRequestChanges={onRequestChanges}
              editing={
                ticket.edit
                  ? { detail, edit: ticket.edit, onSaved: ticket.saved }
                  : undefined
              }
            />
          </div>
        </div>
        <div className="hidden xl:col-start-1 xl:row-start-2 xl:block">
          <div className="sticky top-6 max-h-[calc(100vh-7rem)] overflow-y-auto pb-4">
            <SectionOutline
              sections={sections}
              current={current}
              onChoose={choose}
            />
          </div>
        </div>
        <div
          ref={page}
          className="min-w-0 lg:col-start-1 lg:row-start-2 xl:col-start-2"
        >
          <div className="sticky top-0 z-20 -mx-4 mb-4 border-b border-rule bg-ground px-4 py-2 md:-mx-8 md:px-8 lg:mx-0 lg:px-0 xl:hidden">
            <JumpTo sections={sections} current={current} onChoose={choose} />
          </div>
          {ticket.resource.error ? (
            <StaleNote error={ticket.resource.error} className="pb-4" />
          ) : null}
          <div className="flex flex-col gap-8">
            {ticketSections.map(show)}
            <FormatWarnings warnings={detail.warnings} />
          </div>
          {review ? (
            <section
              aria-labelledby={`${sectionElementId("review")}-heading`}
              className="mt-12 flex flex-col gap-8 border-t border-rule pt-6"
            >
              {show(review)}
              {reviewSections.map(show)}
            </section>
          ) : null}
          {inGroup("files").map((section) => (
            <div key={section.id} className="mt-12 border-t border-rule pt-6">
              {show(section)}
            </div>
          ))}
          {inGroup("activity").map((section) => (
            <div key={section.id} className="mt-12 border-t border-rule pt-6">
              {show(section)}
            </div>
          ))}
        </div>
      </div>
      {requesting && ticket.requestChanges ? (
        <RequestChangesDialog
          ticket={detail.id}
          comments={detail.comments}
          onSubmit={ticket.requestChanges}
          onClose={() => setRequesting(false)}
        />
      ) : null}
    </InPlaceEditing>
  );
}

// PageTitle is the ticket's id and title, the page's h1; where the page
// can save, the title edits in place (FH-66).
function PageTitle({
  detail,
  ticket,
}: {
  detail: TicketDetail;
  ticket: TicketState;
}) {
  const item = useInPlaceItem("title", "title");
  const edit = ticket.edit && item.editable ? ticket.edit : undefined;
  if (edit && item.open)
    return (
      <TicketIdentity
        detail={detail}
        level={1}
        title={
          <>
            <h1 className="sr-only">{detail.title}</h1>
            <div className="mt-1">
              <TitleEditor
                item={item}
                detail={detail}
                edit={edit}
                onSaved={ticket.saved}
              />
            </div>
          </>
        }
      />
    );
  return (
    <TicketIdentity
      detail={detail}
      level={1}
      onDoubleClick={edit ? item.onDoubleClick : undefined}
      action={edit ? <EditPencil item={item} name="title" /> : undefined}
    />
  );
}

// SectionHeading is a page section's heading, with a count beside it and
// its pencil after it; the section's element id names it.
function SectionHeading({
  id,
  level,
  children,
  aside,
  action,
  className = "",
}: {
  id: string;
  level: 2 | 3;
  children: ReactNode;
  aside?: ReactNode;
  action?: ReactNode;
  className?: string;
}) {
  const Heading = level === 2 ? "h2" : "h3";
  const heading = (
    <Heading
      id={`${sectionElementId(id)}-heading`}
      className={`flex items-baseline justify-between gap-3 text-md heading-cut ${action ? "flex-1" : "mb-2"} ${className}`}
    >
      {children}
      {aside ? (
        <span className="text-sm font-normal text-ink-muted">{aside}</span>
      ) : null}
    </Heading>
  );
  return action ? (
    <div className="mb-2 flex items-center gap-2">
      {heading}
      <span className="-my-1.5">{action}</span>
    </div>
  ) : (
    heading
  );
}

// The template's sections are named in lower case in their edit's words
// ("Edit test plan", "Add context"); a section of the ticket's own keeps
// its heading.
const itemName = (section: PageSection) =>
  section.template ? section.title.toLowerCase() : section.title;

// activityWords says what Activity lists under each filter.
const activityWords: Record<ActivityFilter, string> = {
  all: "Comments, notes, history, handoff updates and questions, oldest first",
  comments: "People's comments, oldest first",
  history: "Changes to the ticket, oldest first",
};

// PageSectionView is one section of the page, in its element (marked for
// the outline) and under its heading, with its in-place edit (FH-66)
// where it has one.
function PageSectionView({
  section,
  detail,
  ticket,
  keys,
  onOpen,
  context,
  activity,
  filter,
  onFilter,
  onRequestChanges,
}: {
  section: PageSection;
  detail: TicketDetail;
  ticket: TicketState;
  keys: Set<string>;
  onOpen(ticket: TicketRef): void;
  context: LinkContext;
  activity: ActivityEntry[];
  filter: ActivityFilter;
  onFilter(filter: ActivityFilter): void;
  onRequestChanges?(): void;
}) {
  const markdown = (text = "", inline = false) =>
    text.trim() ? (
      <Markdown context={context} keys={keys} inline={inline}>
        {text}
      </Markdown>
    ) : null;
  const labelled = (
    level: 2 | 3,
    body: ReactNode,
    aside?: ReactNode,
    action?: ReactNode,
    // pencil: the action is the section's pencil, shown while the section
    // is hovered or holds focus (not Activity's filter, whose comments
    // show their own).
    pencil = !!action,
  ) => (
    <section
      aria-labelledby={`${sectionElementId(section.id)}-heading`}
      className={pencil ? editGroup : undefined}
    >
      <SectionHeading
        id={section.id}
        level={level}
        aside={aside}
        action={action}
      >
        {section.title}
      </SectionHeading>
      {body}
    </section>
  );
  // A ticket section named in the file edits in place (FH-66), on a page
  // that can save: the criteria and text sections, never Handoff, the
  // review or Activity.
  const name = itemName(section);
  const item = useInPlaceItem(`section-${section.id}`, name);
  const editor =
    ticket.edit && section.heading !== undefined && item.editable
      ? { item, detail, edit: ticket.edit, onSaved: ticket.saved }
      : undefined;
  const content = (() => {
    switch (section.kind) {
      case "repair":
        return <RepairNotice detail={detail} level={2} />;
      case "requests":
        return labelled(
          2,
          <ChangeRequestList
            requests={requestsOf(detail.comments)}
            render={(text) => markdown(text)}
            onState={ticket.setRequestState}
          />,
          requestWords(countRequests(detail.comments)) || "All resolved",
          onRequestChanges ? (
            <Button
              variant="secondary"
              className="h-8 py-0 text-xs"
              onClick={onRequestChanges}
            >
              Request changes
            </Button>
          ) : null,
          false,
        );
      case "questions":
        return (
          <QuestionsCallout
            detail={detail}
            onAnswer={ticket.answer}
            level={2}
          />
        );
      case "handoff":
        return <HandoffCallout detail={detail} keys={keys} level={2} />;
      case "blocked":
        return (
          <ReasonList
            id="blocked-heading"
            title="Blocked by"
            reasons={detail.blockedBy}
            keys={keys}
            onOpen={onOpen}
            level={2}
          />
        );
      case "criteria":
        if (editor && item.open)
          return labelled(
            2,
            <CriteriaEditor {...editor} heading={section.heading ?? ""} />,
          );
        if (editor && section.empty) return <AddItem item={item} name={name} />;
        return (
          <Checklist
            id="criteria-heading"
            title="Acceptance criteria"
            items={detail.criteriaItems}
            render={(text) => <LinkedText text={text} keys={keys} />}
            onToggle={ticket.toggle}
            level={2}
            className={editGroup}
            onDoubleClick={editor ? item.onDoubleClick : undefined}
            action={editor ? <EditPencil item={item} name={name} /> : null}
          />
        );
      case "text":
        if (editor && item.open)
          return labelled(
            2,
            <SectionEditor
              {...editor}
              heading={section.heading ?? ""}
              title={section.title}
            />,
          );
        if (editor && section.empty) return <AddItem item={item} name={name} />;
        return labelled(
          2,
          editor ? (
            // biome-ignore lint/a11y/noStaticElementInteractions: a double-click is a shortcut; the pencil is the control.
            <div onDoubleClick={item.onDoubleClick}>
              {markdown(section.markdown)}
            </div>
          ) : (
            markdown(section.markdown)
          ),
          undefined,
          editor ? <EditPencil item={item} name={name} /> : null,
        );
      case "review":
        return (
          <div>
            <h2
              id={`${sectionElementId("review")}-heading`}
              className="flex items-center gap-2 text-xl display-cut"
            >
              <Icon
                name="review"
                size={18}
                className="text-(--fh-state-review-ink)"
              />
              Review
            </h2>
            {section.markdown ? (
              <div className="mt-2 text-sm">{markdown(section.markdown)}</div>
            ) : null}
          </div>
        );
      case "review-text":
        return labelled(3, markdown(section.markdown));
      case "evidence":
        return labelled(
          3,
          <>
            {section.markdown?.trim() ? (
              <div className="mb-3">{markdown(section.markdown)}</div>
            ) : null}
            <EvidenceGallery
              images={section.images ?? []}
              files={detail.attachmentFiles}
              context={context}
              keys={keys}
            />
          </>,
          section.count
            ? `${section.count} screenshot${section.count === "1" ? "" : "s"}`
            : undefined,
        );
      case "verify": {
        const review = detail.review;
        if (!review) return null;
        return (
          <Checklist
            id={`${sectionElementId(section.id)}-heading`}
            title={section.title}
            items={review.steps}
            render={(text) => markdown(text, true)}
            onToggle={ticket.verify}
            level={3}
            intro={markdown(review.intro)}
            outro={markdown(review.outro)}
          />
        );
      }
      case "files":
        return labelled(
          2,
          <Attachments attachments={unreferencedFiles(detail)} wide />,
        );
      case "activity":
        return labelled(
          2,
          <>
            <p className="-mt-1 mb-4 text-xs text-ink-muted">
              {activityWords[filter]}
            </p>
            <TicketComments
              detail={detail}
              ticket={ticket}
              entries={activity}
              filter={filter}
              keys={keys}
            />
          </>,
          undefined,
          <SegmentedControl
            label="Show in Activity"
            options={ACTIVITY_FILTERS}
            value={filter}
            onChange={onFilter}
          />,
          false,
        );
      default:
        return null;
    }
  })();
  return (
    <div
      id={sectionElementId(section.id)}
      data-section={section.id}
      className="scroll-mt-20 xl:scroll-mt-6"
    >
      {content}
    </div>
  );
}

// EvidenceGallery shows the review's Evidence screenshots once, as captioned
// thumbnails that open the lightbox (FH-65): each image's caption is the
// review's words for it, its file the ticket's attachment.
function EvidenceGallery({
  images,
  files,
  context,
  keys,
}: {
  images: ImageRef[];
  files: Attachment[];
  context: LinkContext;
  keys: Set<string>;
}) {
  const shown: Attachment[] = [];
  const unavailable: ImageRef[] = [];
  for (const image of images) {
    const target = resolveLink(image.src, context);
    if (target.kind !== "attachment") {
      unavailable.push(image);
      continue;
    }
    const name = localFile(image.src);
    const file = files.find((item) => item.file === name);
    const key = file?.file ?? target.href;
    if (shown.some((item) => item.file === key)) continue;
    shown.push({
      file: key,
      caption: image.alt || file?.caption || name,
      kind: file?.kind || "screenshot",
      run: file?.run ?? "",
      added: file?.added ?? "",
      url: file?.url ?? target.href,
    });
  }
  return (
    <>
      {shown.length > 0 ? <Attachments attachments={shown} wide /> : null}
      {unavailable.length > 0 ? (
        <div className="mt-2 text-sm">
          <Markdown context={context} keys={keys}>
            {unavailable
              .map((image) => `![${image.alt}](${image.src})`)
              .join("\n\n")}
          </Markdown>
        </div>
      ) : null}
    </>
  );
}
