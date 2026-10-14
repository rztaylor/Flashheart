import type { Reason, TicketDetail, TicketRef } from "../../api/board";
import { Aside } from "../../components/Aside";
import { Icon } from "../../components/Icon";
import { Markdown } from "../../components/Markdown";
import { AttentionDisc } from "../../components/NeedsYou";
import { BlockerPill } from "../../components/Pill";
import { QuestionCard } from "../../components/QuestionCard";
import { StateNote } from "../../components/StateNote";
import { LinkedText } from "../../components/TicketLink";
import { ticketBody } from "../../model/markdown";
import { counted } from "../../model/runs";
import { useNow } from "../../state/useNow";
import { Checklist } from "./Checklist";

// HeadingLevel is a section's heading level: 3 under the panel's h2
// title, 2 under the full page's h1 title.
export type HeadingLevel = 2 | 3;

const headingTag = (level: HeadingLevel) => (level === 2 ? "h2" : "h3");

// ReasonList explains why a ticket cannot start (CARD-4), each reason a
// blocked pill.
export function ReasonList({
  id,
  title,
  reasons,
  keys,
  onOpen,
  level = 3,
}: {
  id: string;
  title: string;
  reasons: Reason[];
  keys: Set<string>;
  onOpen(ticket: TicketRef): void;
  level?: HeadingLevel;
}) {
  if (reasons.length === 0) return null;
  const Heading = headingTag(level);
  return (
    <section aria-labelledby={id}>
      <Heading
        id={id}
        className="mb-2 flex items-center gap-1.5 text-md heading-cut text-ink"
      >
        <Icon
          name="blocked"
          size={15}
          className="text-(--fh-state-blocked-ink)"
        />
        {title}
      </Heading>
      <ul className="flex flex-col gap-1.5">
        {reasons.map((reason) => (
          <li
            key={reason.text}
            className="flex items-start justify-between gap-3 text-sm"
          >
            <BlockerPill title={reason.text}>
              <LinkedText text={reason.text} keys={keys} />
            </BlockerPill>
            {reason.ticket && !reason.missing ? (
              <button
                type="button"
                onClick={() =>
                  reason.ticket && onOpen({ id: reason.ticket.id })
                }
                aria-label={`Open ${reason.ticket.id}`}
                className="mt-1 shrink-0 text-xs font-medium text-ink-muted underline decoration-ink-faint hover:text-ink"
              >
                Open
              </button>
            ) : null}
          </li>
        ))}
      </ul>
    </section>
  );
}

// RepairNotice shows why the ticket file does not parse, dashed and
// hatched, with its frontmatter (ui-layout.md §8).
export function RepairNotice({
  detail,
  level = 3,
}: {
  detail: TicketDetail;
  level?: HeadingLevel;
}) {
  if (detail.needsRepair.length === 0) return null;
  const Heading = headingTag(level);
  return (
    <section
      aria-labelledby="repair-heading"
      className="relative overflow-hidden rounded-card border border-dashed border-ink-muted p-3 pt-4"
    >
      <span
        aria-hidden="true"
        className="hatched-strong absolute inset-x-0 top-0 h-1.5"
      />
      <Heading id="repair-heading" className="mb-1.5 text-md heading-cut">
        Needs repair
      </Heading>
      {detail.needsRepair.map((problem) => (
        <StateNote key={problem} kind="repair">
          {problem}
        </StateNote>
      ))}
      <p className="mt-2 text-xs text-ink-muted">
        Flashheart never rewrites a broken file. Fix it in your editor; the
        board updates when it parses.
      </p>
      {detail.frontmatter ? (
        <pre className="mt-2 overflow-x-auto rounded-card bg-well p-2 font-mono text-xs">
          {detail.frontmatter}
        </pre>
      ) : null}
    </section>
  );
}

// QuestionsCallout holds the agents' open questions about the ticket, each
// with its answer box or option buttons (CARD-6).
export function QuestionsCallout({
  detail,
  onAnswer,
  level = 3,
}: {
  detail: TicketDetail;
  onAnswer?(question: string, answer: string): Promise<string | undefined>;
  level?: HeadingLevel;
}) {
  const now = useNow();
  if (detail.questions.length === 0) return null;
  const Heading = headingTag(level);
  return (
    <section
      aria-labelledby="questions-heading"
      className="flex flex-col gap-3 rounded-card border border-callout-question-edge/30 bg-callout-question p-4"
    >
      <Heading
        id="questions-heading"
        className="flex items-center gap-2 text-md heading-cut"
      >
        <AttentionDisc>
          <Icon name="warning" size={12} />
        </AttentionDisc>
        {detail.questions.length === 1
          ? "Question for you"
          : `${detail.questions.length} questions for you`}
      </Heading>
      {detail.questions.map((question) => (
        <QuestionCard
          key={question.id}
          question={question}
          asker={question.runShort}
          now={now}
          onAnswer={
            onAnswer ? (text) => onAnswer(question.id, text) : undefined
          }
        />
      ))}
    </section>
  );
}

// HandoffCallout is the handoff (CARD-5): the next steps large, the whole
// handoff behind a disclosure, and a warning when the run has edited since.
// Without a handoff, an in-progress ticket may earn a marginal remark.
export function HandoffCallout({
  detail,
  keys,
  level = 3,
}: {
  detail: TicketDetail;
  keys: Set<string>;
  level?: HeadingLevel;
}) {
  const stale = staleHandoff(detail);
  const Heading = headingTag(level);
  if (!detail.handoff)
    return detail.column === "in-progress" ? (
      <Aside placement="handoff-missing" />
    ) : null;
  return (
    <section
      aria-labelledby="handoff-heading"
      className="rounded-card border border-callout-handoff-edge/30 bg-callout-handoff p-4"
    >
      <Heading
        id="handoff-heading"
        className="mb-2 flex items-center gap-2 text-md heading-cut"
      >
        <Icon name="next" size={16} className="text-callout-handoff-edge" />
        Handoff
      </Heading>
      {stale ? (
        <div className="mb-3">
          <StateNote kind="warning">{stale}</StateNote>
        </div>
      ) : null}
      {detail.handoff.next.length > 0 ? (
        <div className="mb-3">
          <p className="text-xs text-ink-muted">Next</p>
          <ul className="mt-1 flex flex-col gap-1">
            {detail.handoff.next.map((item) => (
              <li
                key={item}
                className="text-lg leading-snug font-semibold text-ink"
              >
                <LinkedText text={item} keys={keys} />
              </li>
            ))}
          </ul>
        </div>
      ) : null}
      <details className="group text-sm">
        <summary className="flex cursor-pointer list-none items-center gap-1 text-xs font-medium text-ink-muted hover:text-ink [&::-webkit-details-marker]:hidden">
          <Icon
            name="chevronDown"
            size={12}
            className="-rotate-90 transition-transform group-open:rotate-0"
          />
          Full handoff
        </summary>
        <div className="mt-2">
          <Markdown
            context={{ project: detail.project, ticket: detail.id }}
            keys={keys}
          >
            {detail.handoff.markdown}
          </Markdown>
        </div>
      </details>
      {!stale && detail.handoff.next.length > 0 ? (
        <Aside placement="handoff" className="mt-3" />
      ) : null}
    </section>
  );
}

// FormatWarnings lists what in the ticket file does not follow the format.
export function FormatWarnings({ warnings }: { warnings: string[] }) {
  if (warnings.length === 0) return null;
  return (
    <section aria-label="Format warnings" className="flex flex-col gap-1">
      {warnings.map((warning) => (
        <StateNote key={warning} kind="warning">
          {warning}
        </StateNote>
      ))}
    </section>
  );
}

// TicketTab, in the order of ui-layout.md §3: needs repair, questions for
// the user, the handoff, blockers, criteria, format warnings and
// the ticket's own markdown.
export function TicketTab({
  detail,
  keys,
  onOpen,
  onToggle,
  onAnswer,
}: {
  detail: TicketDetail;
  keys: Set<string>;
  onOpen(ticket: TicketRef): void;
  // onToggle ticks or unticks a criterion (CARD-3); absent when read-only.
  onToggle?(index: number, checked: boolean): Promise<boolean>;
  // onAnswer answers an agent's question (CARD-6); absent when read-only.
  onAnswer?(question: string, answer: string): Promise<string | undefined>;
}) {
  return (
    <div className="flex flex-col gap-6">
      <RepairNotice detail={detail} />
      <QuestionsCallout detail={detail} onAnswer={onAnswer} />
      <HandoffCallout detail={detail} keys={keys} />
      <ReasonList
        id="blocked-heading"
        title="Blocked by"
        reasons={detail.blockedBy}
        keys={keys}
        onOpen={onOpen}
      />
      {detail.criteriaItems.length > 0 ? (
        <Checklist
          id="criteria-heading"
          title="Acceptance criteria"
          items={detail.criteriaItems}
          render={(text) => <LinkedText text={text} keys={keys} />}
          onToggle={onToggle}
        />
      ) : null}
      <FormatWarnings warnings={detail.warnings} />
      <section aria-label="Ticket text">
        <Markdown
          context={{ project: detail.project, ticket: detail.id }}
          keys={keys}
        >
          {ticketBody(detail.body)}
        </Markdown>
      </section>
    </div>
  );
}

// staleHandoff warns when the run working on the ticket has edited files
// since its last checkpoint, so the handoff may be behind (CARD-5).
function staleHandoff(detail: TicketDetail): string {
  const run = detail.runs.find((item) => !item.parent && item.dirty);
  if (!run) return "";
  const edits = counted(run.edits, "edit");
  return run.state === "ended"
    ? `Run ${run.short} ended after ${edits} without updating this handoff.`
    : `Run ${run.short} has made ${edits} since this handoff.`;
}
