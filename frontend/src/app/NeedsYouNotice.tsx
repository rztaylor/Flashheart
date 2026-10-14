import { Button } from "../components/Button";
import { EmptyState } from "../components/EmptyState";
import { AttentionDisc } from "../components/NeedsYou";
import { needsYouPhrase } from "../model/runs";

// Agents that need the user with no ticket on the board (FH-44): they asked
// about a project or workstream, not a ticket, so no card and no filter can
// show them. The Overview's Needs your decision lists them (D32), and the
// Board and Table say so wherever the Needs you filter is on (FH-157).

const LABEL = "Agents without a ticket";

// why says why no card shows them.
const why = (count: number) =>
  count === 1
    ? "It asked about the project or a workstream, so no card can show it."
    : "They asked about a project or a workstream, so no card can show them.";

// AnswerButton says what pressing it gets, on its own (FH-157): it opens
// the Overview at Needs your decision.
function AnswerButton({
  count,
  onAnswer,
  className,
}: {
  count: number;
  onAnswer(): void;
  className?: string;
}) {
  return (
    <Button variant="primary" className={className} onClick={onAnswer}>
      {count === 1
        ? "Answer it in the Overview"
        : "Answer them in the Overview"}
    </Button>
  );
}

// NeedsYouNotice sits above the Board's cards or the Table's rows while the
// Needs you filter shows some tickets and more agents need you with none.
export function NeedsYouNotice({
  count,
  onAnswer,
}: {
  count: number;
  onAnswer(): void;
}) {
  if (count === 0) return null;
  return (
    <section
      aria-label={LABEL}
      className="mx-4 mb-3 flex flex-wrap items-center gap-x-3 gap-y-2 rounded-card border border-rule bg-well px-4 py-2.5 text-sm text-ink md:mx-6"
    >
      <AttentionDisc />
      <p className="min-w-0 flex-1 basis-60">
        <span className="font-semibold">
          {`${count} more ${needsYouPhrase(count, true)}, not about a ticket.`}
        </span>{" "}
        <span className="text-ink-muted">{why(count)}</span>
      </p>
      <AnswerButton count={count} onAnswer={onAnswer} className="h-8" />
    </section>
  );
}

// NeedsYouEmpty takes the place of "No tickets match these filters" when
// the Needs you filter is why nothing is listed: what waits, why no card
// shows it and where to answer it; or that nothing needs the user.
// otherFilters says other filters are on too and may hide tickets.
export function NeedsYouEmpty({
  count,
  otherFilters,
  onAnswer,
}: {
  count: number;
  otherFilters: boolean;
  onAnswer(): void;
}) {
  if (count === 0)
    return (
      <EmptyState title="Nothing needs you right now">
        When an agent asks you about a ticket, its card shows here.
      </EmptyState>
    );
  return (
    <section aria-label={LABEL}>
      <EmptyState
        mark={<AttentionDisc size="lg" />}
        title={`${count} ${needsYouPhrase(count, true)}, not about a ticket`}
      >
        <p>
          {why(count)}{" "}
          {count === 1
            ? "It is waiting in the Overview, under Needs your decision."
            : "They are waiting in the Overview, under Needs your decision."}
        </p>
        <AnswerButton count={count} onAnswer={onAnswer} className="mt-4" />
        {otherFilters ? (
          <p className="mt-4">
            Other filters are on too: clear them to see any tickets that need
            you.
          </p>
        ) : null}
      </EmptyState>
    </section>
  );
}
