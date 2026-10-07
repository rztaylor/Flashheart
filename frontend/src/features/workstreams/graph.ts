// layoutGraph lays a workstream's tickets out as a railway graph (VIEW-4,
// ui-layout.md §4): a ticket's depends-on links to other tickets on the line
// are track. A ticket's layer (column) is one past the longest chain it
// depends on; it carries on along the track of a dependency where it can and
// starts a new track where the work branches, and every other dependency's
// track joins it. Tickets with no links on the line are free stations.
// Pure: no React, no DOM.

export interface GraphTicket {
  id: string;
  dependsOn: string[];
}

export interface Station {
  id: string;
  layer: number;
  lane: number;
}

// An edge runs from a dependency to the ticket that needs it. straight: on
// one track. branch: leaves the dependency's track just after it and runs on
// the ticket's. merge: runs on the dependency's track and joins just before
// the ticket. detour: both tracks are in the way, so it runs on track via.
export interface Edge {
  from: string;
  to: string;
  route: "straight" | "branch" | "merge" | "detour";
  via?: number;
}

export interface Layout {
  stations: Station[];
  edges: Edge[];
  free: string[];
  layers: number;
  lanes: number;
}

export function layoutGraph(tickets: GraphTicket[]): Layout {
  const order = new Map<string, number>();
  const deps = new Map<string, string[]>();
  for (const ticket of tickets) {
    if (order.has(ticket.id)) continue;
    order.set(ticket.id, order.size);
    deps.set(ticket.id, []);
  }
  for (const ticket of tickets) {
    const own = deps.get(ticket.id);
    if (!own || own.length > 0) continue;
    for (const id of ticket.dependsOn)
      if (id !== ticket.id && order.has(id) && !own.includes(id)) own.push(id);
  }

  // Layers by the longest chain; an edge that closes a cycle is dropped.
  const layer = new Map<string, number>();
  const visiting = new Set<string>();
  const depth = (id: string): number => {
    const known = layer.get(id);
    if (known !== undefined) return known;
    if (visiting.has(id)) return -1;
    visiting.add(id);
    let value = 0;
    for (const dep of deps.get(id) ?? [])
      value = Math.max(value, depth(dep) + 1);
    visiting.delete(id);
    layer.set(id, value);
    return value;
  };
  for (const id of order.keys()) depth(id);
  for (const [id, list] of deps)
    deps.set(
      id,
      list.filter((dep) => (layer.get(dep) ?? 0) < (layer.get(id) ?? 0)),
    );

  const linked = new Set<string>();
  for (const [id, list] of deps)
    if (list.length > 0) {
      linked.add(id);
      for (const dep of list) linked.add(dep);
    }
  const ids = [...order.keys()];
  const free = ids.filter((id) => !linked.has(id));
  const placing = ids
    .filter((id) => linked.has(id))
    .sort(
      (a, b) =>
        (layer.get(a) ?? 0) - (layer.get(b) ?? 0) ||
        (order.get(a) ?? 0) - (order.get(b) ?? 0),
    );

  // Each track remembers its last station.
  const tracks: { id: string; layer: number }[] = [];
  const lane = new Map<string, number>();
  for (const id of placing) {
    const at = layer.get(id) ?? 0;
    const own = deps.get(id) ?? [];
    // Carry on along the track of the latest dependency that ends there,
    // the topmost of equals, so the trunk stays on the top track.
    const carry = own
      .filter((dep) => tracks[lane.get(dep) ?? -1]?.id === dep)
      .sort(
        (a, b) =>
          (layer.get(b) ?? 0) - (layer.get(a) ?? 0) ||
          (lane.get(a) ?? 0) - (lane.get(b) ?? 0),
      )[0];
    let chosen = carry === undefined ? -1 : (lane.get(carry) ?? -1);
    if (chosen < 0) {
      // A new track, clear from the earliest dependency onward.
      const from = own.length
        ? Math.min(...own.map((dep) => layer.get(dep) ?? 0))
        : at - 1;
      chosen = tracks.findIndex((track) => track.layer <= from);
      if (chosen < 0) chosen = tracks.length;
    }
    tracks[chosen] = { id, layer: at };
    lane.set(id, chosen);
  }

  const stations = ids
    .filter((id) => linked.has(id))
    .map((id) => ({ id, layer: layer.get(id) ?? 0, lane: lane.get(id) ?? 0 }));
  const occupied = new Set(stations.map((s) => `${s.lane}:${s.layer}`));
  let lanes = tracks.length;
  const clear = (track: number, from: number, to: number) => {
    for (let at = from + 1; at < to; at++)
      if (occupied.has(`${track}:${at}`)) return false;
    return true;
  };
  const straight = new Set<string>();
  for (const [id, list] of deps)
    for (const dep of list)
      if (lane.get(dep) === lane.get(id)) straight.add(dep);

  const edges: Edge[] = [];
  for (const id of ids)
    for (const from of deps.get(id) ?? []) {
      const [ls, lt] = [layer.get(from) ?? 0, layer.get(id) ?? 0];
      const [source, target] = [lane.get(from) ?? 0, lane.get(id) ?? 0];
      if (source === target && clear(source, ls, lt)) {
        edges.push({ from, to: id, route: "straight" });
        continue;
      }
      if (source !== target) {
        // A track that carries on past its station branches off near it; one
        // that ends there flows into the ticket that needs it.
        const routes = straight.has(from)
          ? (["branch", "merge"] as const)
          : (["merge", "branch"] as const);
        const route = routes.find((item) =>
          clear(item === "branch" ? target : source, ls, lt),
        );
        if (route) {
          edges.push({ from, to: id, route });
          continue;
        }
      }
      let via = 0;
      while (via === source || via === target || !clear(via, ls, lt)) via++;
      lanes = Math.max(lanes, via + 1);
      edges.push({ from, to: id, route: "detour", via });
    }

  const layers = stations.reduce((max, s) => Math.max(max, s.layer + 1), 0);
  return { stations, edges, free, layers, lanes };
}

// moveFree moves the independent ticket at position from to position to
// among the free tickets, which take each other's places in the full ticket
// list, so tickets on the graph keep theirs (EDIT-4).
export function moveFree(
  all: string[],
  free: string[],
  from: number,
  to: number,
): string[] {
  const moved = [...free];
  const [item] = moved.splice(from, 1);
  if (item === undefined) return all;
  moved.splice(to, 0, item);
  let next = 0;
  return all.map((id) =>
    free.includes(id) && next < moved.length ? (moved[next++] ?? id) : id,
  );
}
