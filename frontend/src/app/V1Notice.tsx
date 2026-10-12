// V1Notice names the board format v1 projects, which are not shown until
// they are migrated, and the command that migrates them (MIG-1).
export function V1Notice({
  projects,
  command,
}: {
  projects: string[];
  command: string;
}) {
  const names = projects.join(", ");
  return (
    <section
      aria-label="Older board format"
      className="mx-4 mt-1 mb-3 rounded-card border border-rule bg-well px-4 py-3 text-sm text-ink-muted md:mx-6"
    >
      <p>
        {projects.length === 1 ? "The project " : "The projects "}
        <span className="text-ink">{names}</span>{" "}
        {projects.length === 1 ? "uses" : "use"} the older board format and{" "}
        {projects.length === 1 ? "is" : "are"} hidden until migrated. Preview
        the change, then add <code className="font-mono">--write</code> to apply
        it:
      </p>
      <pre className="mt-2 overflow-x-auto font-mono text-xs text-ink">
        {command}
      </pre>
    </section>
  );
}
