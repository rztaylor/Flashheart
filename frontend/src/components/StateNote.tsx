import { Icon } from "./Icon";

type Kind = "blocked" | "waiting" | "repair" | "warning";

const icons = {
  blocked: "diamond",
  repair: "repair",
  warning: "warning",
} as const;
const labels = {
  blocked: "Blocked",
  waiting: "Waiting",
  repair: "Needs repair",
  warning: "Warning",
};

// StateNote pairs a ticket state with its proof ("Blocked: Depends on …").
// States are ink, shape and words, never a hue. A wait on an earlier station
// of the ticket's own line is quiet: no mark, muted words.
export function StateNote({
  kind,
  children,
  compact,
}: {
  kind: Kind;
  children: string;
  compact?: boolean;
}) {
  const icon = kind === "waiting" ? undefined : icons[kind];
  return (
    <p
      className={`flex items-start gap-1.5 ${compact ? "text-xs" : "text-sm"} ${
        kind === "waiting"
          ? "text-ink-faint"
          : kind === "warning"
            ? "text-ink-muted"
            : "text-ink"
      }`}
    >
      {icon ? (
        <Icon name={icon} size={compact ? 12 : 14} className="mt-[0.2em]" />
      ) : null}
      <span className={compact ? "line-clamp-3" : undefined}>
        <span className="sr-only">{labels[kind]}: </span>
        {children}
      </span>
    </p>
  );
}
