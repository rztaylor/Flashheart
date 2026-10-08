import type { ReactNode } from "react";
import ReactMarkdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";

import {
  type LinkContext,
  linkTicketIds,
  resolveLink,
} from "../model/markdown";
import { TicketLink } from "./TicketLink";

interface MarkdownProps {
  children: string;
  context: LinkContext;
  // Project keys whose ticket ids in text become links (KEY-3).
  keys: Set<string>;
  // inline renders one line of markdown inside a span, without paragraphs.
  inline?: boolean;
}

// Markdown renders GitHub-flavoured markdown without raw HTML (CARD-2,
// SEC-4). Ticket links open the ticket's full page (TicketLink), web links
// open in a new tab, images
// load only from the attachment endpoint, and anything else renders as text.
export function Markdown({
  children,
  context,
  keys,
  inline = false,
}: MarkdownProps) {
  const components: Components = {
    a({ href = "", children: label }) {
      const target = resolveLink(href, context);
      switch (target.kind) {
        case "ticket":
          return (
            <TicketLink
              id={target.id}
              className="font-medium text-ink underline decoration-rule-strong/40 hover:decoration-rule-strong"
            >
              {label}
            </TicketLink>
          );
        case "external":
          return (
            <a
              href={target.href}
              target="_blank"
              rel="noopener noreferrer"
              className="text-ink underline decoration-ink-faint hover:decoration-ink"
            >
              {label}
            </a>
          );
        default:
          return <span>{label as ReactNode}</span>;
      }
    },
    input({ type, checked }) {
      if (type !== "checkbox") return null;
      return (
        <input
          type="checkbox"
          checked={Boolean(checked)}
          disabled
          aria-label={checked ? "Done" : "Not done"}
        />
      );
    },
    img({ src, alt }) {
      const target =
        typeof src === "string"
          ? resolveLink(src, context)
          : { kind: "none" as const };
      if (target.kind !== "attachment") {
        return (
          <span className="text-ink-muted">
            [image: {alt || "unavailable"}]
          </span>
        );
      }
      return (
        <a
          href={target.href}
          target="_blank"
          rel="noopener noreferrer"
          className="block"
        >
          <img
            src={target.href}
            alt={alt ?? ""}
            loading="lazy"
            className="max-w-full rounded-card border border-rule"
          />
        </a>
      );
    },
  };
  if (inline) components.p = ({ children: text }) => <>{text}</>;
  const rendered = (
    <ReactMarkdown
      remarkPlugins={[remarkGfm, () => linkTicketIds(keys)]}
      skipHtml
      components={components}
    >
      {children}
    </ReactMarkdown>
  );
  return inline ? (
    <span className="markdown">{rendered}</span>
  ) : (
    <div className="markdown">{rendered}</div>
  );
}
