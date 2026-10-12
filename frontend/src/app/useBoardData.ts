import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import {
  type Card,
  fetchAllBoard,
  fetchProjectBoard,
  fetchProjects,
  type Status,
} from "../api/board";
import type { AuthenticatedFetch } from "../api/client";
import type { ToastMessage } from "../components/Toast";
import { type Movable, useEditing } from "../features/editing/useEditing";
import { linesByProject } from "../model/lines";
import { placeCard } from "../model/order";
import { moveRemark, nextRemark } from "../model/remarks";
import { useResource } from "../state/useResource";
import { useRevision } from "../state/useRevision";
import type { Scope, View } from "./route";

type BoardData = { cards: Card[]; doneTotal: number; doneShown: number };

// useBoardData loads the projects and, for the Board and Table, the scope's
// board, and reloads both when the board revision moves (live updates,
// LIFE-3). It wires editing to them: moves show in their new column and
// place at once, before the save returns, and a move to Done may earn a
// remark in its toast (FH-22).
export function useBoardData({
  fetcher,
  ready,
  scope,
  view,
}: {
  fetcher: AuthenticatedFetch;
  ready: boolean;
  scope: Scope;
  view: View;
}) {
  // doneAll shows every Done ticket rather than the most recent.
  const [doneAll, setDoneAll] = useState(false);
  const loadProjects = useCallback(
    (signal: AbortSignal) => fetchProjects(fetcher, signal),
    [fetcher],
  );
  const revision = useRevision(fetcher, ready);
  const projects = useResource(
    ready ? loadProjects : undefined,
    "projects",
    revision,
  );

  const scopeKey = scope.kind === "all" ? "all" : `p:${scope.project}`;
  const scopeProject = scope.kind === "project" ? scope.project : "";
  const loadBoard = useCallback(
    (signal: AbortSignal): Promise<BoardData> =>
      scopeProject
        ? fetchProjectBoard(fetcher, scopeProject, doneAll, signal)
        : fetchAllBoard(fetcher, doneAll, signal),
    [fetcher, scopeProject, doneAll],
  );
  const board = useResource(
    ready && (view === "board" || view === "table") ? loadBoard : undefined,
    `${scopeKey}:${doneAll}`,
    revision,
  );
  const reloadAll = useCallback(() => {
    projects.reload();
    board.reload();
  }, [projects.reload, board.reload]);

  // The remark is judged on the board as shown when the save returns.
  const shownCards = useRef<Card[]>([]);
  const lastProgress = useRef<number | undefined>(undefined);
  const remark = useCallback(
    (ticket: Movable, to: Status): ToastMessage["aside"] => {
      const now = Date.now();
      const placement = moveRemark(
        shownCards.current,
        ticket,
        to,
        lastProgress.current,
        now,
      );
      if (placement === "progress") lastProgress.current = now;
      return placement && { placement, text: nextRemark(placement).text };
    },
    [],
  );
  const editing = useEditing(fetcher, reloadAll, remark);
  const { reconcile } = editing;

  const loadedCards = board.status === "ready" ? board.data.cards : undefined;
  useEffect(() => {
    if (loadedCards) reconcile(loadedCards);
  }, [loadedCards, reconcile]);
  const cards: Card[] = useMemo(
    () =>
      Object.entries(editing.pending).reduce(
        (shown, [id, entry]) => placeCard(shown, id, entry.column, entry.after),
        loadedCards ?? [],
      ),
    [loadedCards, editing.pending],
  );
  shownCards.current = cards;

  const summaries = projects.status === "ready" ? projects.data.projects : [];
  const lines = useMemo(
    () =>
      linesByProject(
        summaries.map((project) => ({
          project: project.name,
          workstreams: project.workstreams,
        })),
      ),
    [summaries],
  );
  const workstreams = useMemo(
    () =>
      new Map(summaries.map((project) => [project.name, project.workstreams])),
    [summaries],
  );
  const keys = useMemo(
    () => new Set(summaries.map((project) => project.key)),
    [summaries],
  );

  return {
    revision,
    projects,
    board,
    reloadAll,
    editing,
    doneAll,
    setDoneAll,
    // cards are the board's cards with pending moves placed.
    cards,
    summaries,
    // current is the scope's project, absent in All projects or when the
    // project is not on the board.
    current: summaries.find((project) => project.name === scopeProject),
    scopeProject,
    lines,
    workstreams,
    keys,
  };
}

export type BoardDataState = ReturnType<typeof useBoardData>;
