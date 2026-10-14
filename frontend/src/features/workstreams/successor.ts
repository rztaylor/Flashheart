import type { Workstream } from "../../api/board";
import { workstreamTitleKey } from "../../model/lines";

// successorOf is the workstream among a project's that last, a workstream
// whose slug is gone, became (FH-174, D80): the one with its name, as a
// rename by slug alone keeps the name and names are unique in a project
// (D76), or else the one every one of its tickets is now in, as a rename
// keeps the members whatever it does to the name. None when neither
// finds one, or when the two find different ones.
export function successorOf(
  last: Pick<Workstream, "slug" | "title" | "tickets">,
  workstreams: Workstream[],
): Workstream | undefined {
  const others = workstreams.filter((item) => item.slug !== last.slug);
  const key = workstreamTitleKey(last.title);
  const byName = others.find((item) => workstreamTitleKey(item.title) === key);
  const members = last.tickets
    .filter((ticket) => !ticket.missing)
    .map((ticket) => ticket.id);
  const byMembers =
    members.length > 0
      ? others.find((item) => {
          const ids = new Set(
            item.tickets
              .filter((ticket) => !ticket.missing)
              .map((ticket) => ticket.id),
          );
          return members.every((id) => ids.has(id));
        })
      : undefined;
  if (byName && byMembers && byName !== byMembers) return undefined;
  return byName ?? byMembers;
}
