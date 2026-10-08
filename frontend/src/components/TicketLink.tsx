import { createContext, type ReactNode, useContext } from "react";

import { ticketPageHref, ticketRefs } from "../model/markdown";

// idCut is how ticket ids are set everywhere (ui-layout.md "Ticket ids").
export const idCut = "font-semibold tracking-[0.02em] tabular-nums";

// InPlace is true on a ticket's full page, where ticket links move the tab
// to the linked ticket instead of opening another.
const InPlace = createContext(false);

// TicketLinksInPlace makes the ticket links inside it open in the same tab.
export function TicketLinksInPlace({ children }: { children: ReactNode }) {
  return <InPlace.Provider value={true}>{children}</InPlace.Provider>;
}

interface TicketLinkProps {
  id: string;
  // children replaces the id as the link's text (a markdown link's label).
  children?: ReactNode;
  className?: string;
  // label names a link whose content is not the id (an icon).
  label?: string;
  // tabIndex -1 keeps a link off the tab order where its container is the
  // keyboard stop (a board card, a station) and the panel offers it instead.
  tabIndex?: number;
}

// TicketLink is a ticket id as a link to the ticket's full page (KEY-3,
// CARD-7): a new tab from the board and its panel, the same tab on a full
// page. It is a real link, so Cmd or middle click works as anywhere else,
// and it must never sit inside a button. Showing the id, it sets it in the
// id cut (semibold, tabular); callers add size and colour.
export function TicketLink({
  id,
  children,
  className = "",
  label,
  tabIndex,
}: TicketLinkProps) {
  const inPlace = useContext(InPlace);
  const named = label ?? (inPlace ? `Open ${id}` : `Open ${id} in a new tab`);
  return (
    <a
      href={ticketPageHref(id)}
      target={inPlace ? undefined : "_blank"}
      rel={inPlace ? undefined : "noopener"}
      title={named}
      aria-label={label}
      tabIndex={tabIndex}
      className={`rounded-mark underline-offset-2 hover:underline ${children === undefined ? idCut : ""} ${className}`}
    >
      {children ?? id}
    </a>
  );
}

// LinkedText is plain text with the ticket ids of known project keys in it
// as ticket links (KEY-3).
export function LinkedText({
  text,
  keys,
}: {
  text: string;
  keys: Set<string>;
}) {
  return (
    <>
      {ticketRefs(text, keys).map((part, index) =>
        typeof part === "string" ? (
          part
        ) : (
          // biome-ignore lint/suspicious/noArrayIndexKey: parts never reorder.
          <TicketLink key={index} id={part.id} />
        ),
      )}
    </>
  );
}
