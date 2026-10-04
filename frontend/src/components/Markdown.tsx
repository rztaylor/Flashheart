import type { ReactNode } from "react";
import ReactMarkdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";

import type { TicketRef } from "../api/board";
import { type LinkContext, resolveLink } from "../model/markdown";

interface MarkdownProps {
  children: string;
  context: LinkContext;
  onOpenTicket(ticket: TicketRef): void;
}

// Markdown renders GitHub-flavoured markdown without raw HTML (CARD-2,
// SEC-4). Ticket links open the ticket, web links open in a new tab, images
// load only from the attachment endpoint, and anything else renders as text.
export function Markdown({ children, context, onOpenTicket }: MarkdownProps) {
  const components: Components = {
    a({ href = "", children: label }) {
      const target = resolveLink(href, context);
      switch (target.kind) {
        case "ticket":
          return (
            <a
              href={`#ticket-${target.slug}`}
              onClick={(event) => {
                event.preventDefault();
                onOpenTicket({ project: target.project, slug: target.slug });
              }}
              className="font-medium text-ink underline decoration-rule-strong/40 hover:decoration-rule-strong"
            >
              {label}
            </a>
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
  return (
    <div className="markdown">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        skipHtml
        components={components}
      >
        {children}
      </ReactMarkdown>
    </div>
  );
}
