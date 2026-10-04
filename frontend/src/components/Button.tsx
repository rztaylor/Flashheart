import type { ButtonHTMLAttributes } from "react";

type Variant = "primary" | "secondary" | "quiet" | "band";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
}

const base =
  "inline-flex items-center justify-center gap-2 rounded-control px-3 py-1.5 text-sm font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50";

const variants: Record<Variant, string> = {
  primary: "bg-ink text-ground hover:enabled:bg-ink/85",
  secondary:
    "border border-rule bg-card text-ink hover:enabled:border-ink-muted",
  quiet: "text-ink-muted hover:enabled:text-ink",
  band: "border border-on-band-muted/40 text-on-band hover:enabled:border-on-band hover:enabled:bg-band-field focus-visible:outline-on-band",
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
