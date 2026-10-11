import type { Workstream } from "../../api/board";

// isFinished reports whether a workstream collapses on the Workstreams view
// (FH-68): it has tickets, every one Done (Rejected included) or Archived,
// and nothing to repair. Stricter than the derived Completed status, which
// Ready to review already reaches, so work awaiting review stays in view.
export function isFinished(workstream: Workstream): boolean {
  return (
    workstream.tickets.length > 0 &&
    workstream.needsRepair.length === 0 &&
    workstream.tickets.every(
      (ticket) => ticket.column === "done" || ticket.column === "archived",
    )
  );
}

// keptExpanded is the remembered expanded slugs less those of workstreams
// that are no longer finished or no longer exist, so the choice lasts only
// while a workstream stays finished. It returns slugs itself when nothing is
// dropped.
export function keptExpanded(
  workstreams: Workstream[],
  slugs: string[],
): string[] {
  const finished = new Set(
    workstreams.filter(isFinished).map((workstream) => workstream.slug),
  );
  const kept = slugs.filter((slug) => finished.has(slug));
  return kept.length === slugs.length ? slugs : kept;
}
