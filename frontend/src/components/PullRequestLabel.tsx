import { Icon } from "./Icon";
import { TailText } from "./TailText";

const web = /^https?:\/\//i;
const numbered = /\/(?:pull|pulls|merge_requests)\/(\d+)\/?$/;

// PullRequestLabel is a ticket's pull request in one line (FH-114): the
// pull request mark, then "PR #7" for an address ending in its number, or
// the address itself on TailText. The value is free text an agent or person
// wrote, so only an http(s) address becomes a link, opening in a new tab;
// anything else shows as plain text. It names itself "Pull request" for
// assistive technology. An empty value renders nothing.
export function PullRequestLabel({
  pr,
  className,
}: {
  pr: string;
  className?: string;
}) {
  const value = pr.trim();
  if (!value) return null;
  const number = numbered.exec(value)?.[1];
  const text = number ? `PR #${number}` : value.replace(web, "");
  const label = (
    <>
      <Icon name="pullRequest" size={12} />
      <span className="sr-only">Pull request </span>
      <TailText className="font-mono">{text}</TailText>
    </>
  );
  const box = `flex min-w-0 items-center gap-1 ${className ?? ""}`;
  if (!web.test(value)) return <span className={box}>{label}</span>;
  return (
    <a
      href={value}
      target="_blank"
      rel="noopener noreferrer"
      title={value}
      className={`${box} text-ink underline decoration-ink-faint hover:decoration-ink`}
    >
      {label}
    </a>
  );
}
