import type { ButtonHTMLAttributes } from "react";

type Variant = "primary" | "secondary" | "quiet";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
}

const base =
  "inline-flex items-center justify-center gap-2 rounded-control px-3 py-1.5 text-sm font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-60";

const variants: Record<Variant, string> = {
  primary: "bg-accent text-on-accent hover:enabled:bg-accent-hover",
  secondary:
    "border border-border bg-surface text-text hover:enabled:border-text-muted",
  quiet: "text-text-muted hover:enabled:text-text",
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
