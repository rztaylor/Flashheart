import type { Route, Scope } from "./route";

// NeedsYouPress is what a press of the band's Needs you plate does: the
// filter's new state, where to go, and the scope to return to.
export interface NeedsYouPress {
  on: boolean;
  go?: Partial<Route>;
  cameFrom?: Scope;
}

// pressNeedsYou toggles the Needs you filter (VIEW-2, FH-44). The plate
// counts runs across every project, so turning it on shows All projects:
// the Board, or the Table when that is open. Turning it off returns to the
// project it was turned on from, unless another scope was chosen since.
export function pressNeedsYou(
  route: Pick<Route, "scope" | "view">,
  on: boolean,
  cameFrom?: Scope,
): NeedsYouPress {
  if (!on)
    return {
      on: true,
      go: {
        scope: { kind: "all" },
        view: route.view === "table" ? "table" : "board",
        ticket: undefined,
      },
      cameFrom: route.scope.kind === "all" ? undefined : route.scope,
    };
  return {
    on: false,
    go:
      cameFrom && route.scope.kind === "all" ? { scope: cameFrom } : undefined,
    cameFrom: undefined,
  };
}
