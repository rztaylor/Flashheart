import { Icon } from "./Icon";
import { TailText } from "./TailText";

// BranchLabel is a git branch in one line: the branch mark and its name in
// mono, cut from the start by TailText so the end that tells branches apart
// stays visible. It names itself "Branch" for assistive technology, since
// the mark is decorative. An empty branch renders nothing.
export function BranchLabel({
  branch,
  className,
}: {
  branch: string;
  className?: string;
}) {
  if (!branch) return null;
  return (
    <span className={`flex min-w-0 items-center gap-1 ${className ?? ""}`}>
      <Icon name="branch" size={12} />
      <span className="sr-only">Branch </span>
      <TailText className="font-mono">{branch}</TailText>
    </span>
  );
}
