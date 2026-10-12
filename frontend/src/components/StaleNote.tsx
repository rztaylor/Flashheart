// StaleNote says that a reload failed and the last good copy stays visible
// (ui-layout.md §8), as a quiet status line. className places it: inline
// in a view, or as a strip under the band.
export function StaleNote({
  error,
  className = "",
}: {
  error: string;
  className?: string;
}) {
  return (
    <p role="status" className={`text-xs text-ink-muted ${className}`}>
      Showing the last good copy: {error}
    </p>
  );
}
