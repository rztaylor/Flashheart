import { PAINT_MODES, type Paint, type PaintMode } from "../../model/paint";

interface ColourKeyProps {
  mode: PaintMode;
  paints: Paint[];
}

// ColourKey names what each card colour means for the board's current
// "Colour by" setting, so colour is never the only carrier of meaning.
export function ColourKey({ mode, paints }: ColourKeyProps) {
  if (mode === "none" || paints.length === 0) return null;
  const name = PAINT_MODES.find((option) => option.value === mode)?.label;
  return (
    <ul
      aria-label={`Card colours by ${name?.toLowerCase()}`}
      className="flex items-center gap-x-3 text-xs whitespace-nowrap text-ink-muted"
    >
      {paints.map((paint) => (
        <li key={paint.token} className="flex items-center gap-1.5">
          <span
            aria-hidden="true"
            className="h-2.5 w-3.5 rounded-[2px] shadow-[inset_0_0_0_1px_var(--fh-casing)]"
            style={{
              background: `linear-gradient(var(--fh-paint-${paint.token}) 0 40%, var(--fh-paint-${paint.token}-tint) 40%)`,
            }}
          />
          {paint.label}
        </li>
      ))}
    </ul>
  );
}
