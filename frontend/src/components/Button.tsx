import type { ButtonHTMLAttributes } from "react";

type Variant =
  | "primary"
  | "secondary"
  | "quiet"
  | "band"
  | "danger"
  | "danger-quiet";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
}

const base =
  "inline-flex items-center justify-center gap-2 rounded-control px-3 py-1.5 text-sm font-semibold transition-colors disabled:cursor-not-allowed disabled:opacity-50";

// primary is the action colour (raspberry in light, lime in dark); band
// sits on the frame. danger is red for an action that cannot be undone (a
// permanent delete's confirmation); danger-quiet is red text for the button
// that leads to one.
const variants: Record<Variant, string> = {
  primary: "bg-action text-on-action hover:enabled:bg-action-hover",
  secondary:
    "border border-rule bg-card text-ink shadow-card hover:enabled:border-ink-muted",
  quiet: "text-ink-muted hover:enabled:text-ink",
  band: "border border-on-band-muted/50 text-on-band hover:enabled:border-on-band hover:enabled:bg-band-field focus-visible:outline-on-band",
  danger: "bg-danger text-on-danger hover:enabled:bg-danger-hover",
  "danger-quiet": "text-danger hover:enabled:bg-danger-surface",
};

export function Button({
  variant = "secondary",
  type = "button",
  className,
  ...props
}: ButtonProps) {
  return (
    <button
      type={type}
      className={[base, variants[variant], className].filter(Boolean).join(" ")}
      {...props}
    />
  );
}
