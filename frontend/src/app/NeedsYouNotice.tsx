import { Button } from "../components/Button";
import { AttentionDisc } from "../components/NeedsYou";
import { needsYouPhrase } from "../model/runs";

// NeedsYouNotice sits above the Board and Table while the Needs you filter
// is on and agents need you with no ticket on the board (FH-44): no filter
// can show them, so it points to the Overview, whose Needs your decision
// lists them (D32).
export function NeedsYouNotice({
  count,
  onOpenOverview,
}: {
  count: number;
  onOpenOverview(): void;
}) {
  if (count === 0) return null;
  return (
    <section
      aria-label="Agents without a ticket"
      className="mx-4 mb-3 flex flex-wrap items-center gap-x-3 gap-y-1.5 rounded-card border border-rule bg-well px-4 py-2.5 text-sm text-ink md:mx-6"
    >
      <AttentionDisc />
      <p>
        {`${count} ${needsYouPhrase(count, true)} with no ticket on the board.`}
      </p>
      <Button
        variant="quiet"
        className="h-8 px-1.5 text-xs"
        onClick={onOpenOverview}
      >
        Open Overview
      </Button>
    </section>
  );
}
