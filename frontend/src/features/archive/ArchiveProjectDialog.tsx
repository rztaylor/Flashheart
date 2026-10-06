import { useEffect, useState } from "react";

import {
  type ArchiveCheck,
  archiveProject,
  fetchArchiveCheck,
} from "../../api/archive";
import { COLUMNS } from "../../api/board";
import type { AuthenticatedFetch } from "../../api/client";
import { Button } from "../../components/Button";
import { Dialog } from "../../components/Dialog";
import { StateNote } from "../../components/StateNote";

const failure = (error: unknown) =>
  error instanceof Error ? error.message : "The request failed";

const plural = (n: number, one: string, many: string) =>
  `${n} ${n === 1 ? one : many}`;

// ArchiveProjectDialog archives a project (PRJ-5) after showing what is in
// it and warning about anything still live: running sessions, tickets being
// worked on and questions waiting for an answer. Archiving can be undone.
export function ArchiveProjectDialog({
  fetcher,
  project,
  onClose,
  onArchived,
}: {
  fetcher: AuthenticatedFetch;
  project: string;
  onClose(): void;
  onArchived(displayName: string): void;
}) {
  const [check, setCheck] = useState<ArchiveCheck | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    fetchArchiveCheck(fetcher, project, controller.signal)
      .then(setCheck)
      .catch((reason: unknown) => {
        if (!controller.signal.aborted) setError(failure(reason));
      });
    return () => controller.abort();
  }, [fetcher, project]);

  const confirm = async () => {
    if (!check) return;
    setBusy(true);
    try {
      await archiveProject(fetcher, project);
      onArchived(check.displayName);
    } catch (reason) {
      setError(failure(reason));
      setBusy(false);
    }
  };

  return (
    <Dialog
      title={`Archive ${check?.displayName ?? project}?`}
      onClose={onClose}
      actions={
        <>
          <Button onClick={onClose}>Keep it on the board</Button>
          <Button variant="primary" disabled={!check || busy} onClick={confirm}>
            {busy ? "Archiving…" : "Archive project"}
          </Button>
        </>
      }
    >
      {check ? (
        <ArchiveSummary check={check} />
      ) : error ? null : (
        <p className="text-ink-muted">Checking the project…</p>
      )}
      {error ? (
        <p role="alert" className="mt-3 text-danger">
          {error}
        </p>
      ) : null}
    </Dialog>
  );
}

// ArchiveSummary says what archiving moves and what is still live.
export function ArchiveSummary({ check }: { check: ArchiveCheck }) {
  const warnings = [
    check.liveRuns > 0
      ? `${plural(check.liveRuns, "agent session is", "agent sessions are")} still running in this project. Their activity is not recorded while it is archived.`
      : "",
    check.claimed > 0
      ? `${plural(check.claimed, "ticket is", "tickets are")} being worked on.`
      : "",
    check.openQuestions > 0
      ? `${plural(check.openQuestions, "question is", "questions are")} waiting for an answer.`
      : "",
  ].filter(Boolean);
  return (
    <div className="flex flex-col gap-4">
      <p>
        Archiving moves the project and its{" "}
        {plural(check.tickets, "ticket", "tickets")} off the board, into the
        board root's archive. You can restore it later.
      </p>
      <dl className="grid grid-cols-[auto_1fr] gap-x-6 gap-y-1 rounded-card border border-rule bg-well px-4 py-3">
        {COLUMNS.map((column) => (
          <div key={column.id} className="contents">
            <dt className="text-ink-muted">{column.title}</dt>
            <dd className="tabular-nums">{check.counts[column.id] ?? 0}</dd>
          </div>
        ))}
      </dl>
      {warnings.length > 0 ? (
        <div className="flex flex-col gap-1.5">
          {warnings.map((warning) => (
            <StateNote key={warning} kind="warning">
              {warning}
            </StateNote>
          ))}
        </div>
      ) : null}
      <p className="text-xs text-ink-muted">
        Its tickets count as done for other projects while it is archived, and
        its key stays reserved.
      </p>
    </div>
  );
}
