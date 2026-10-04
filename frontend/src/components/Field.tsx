import type { ChangeEvent, ReactNode } from "react";

import { Icon } from "./Icon";

interface SelectFieldProps {
  label: string;
  value: string;
  onChange(value: string): void;
  children: ReactNode;
}

// SelectField is a labelled native select in the board's control style.
export function SelectField({
  label,
  value,
  onChange,
  children,
}: SelectFieldProps) {
  return (
    <label className="relative flex items-center gap-1.5 text-xs text-ink-muted">
      <span>{label}</span>
      <select
        value={value}
        onChange={(event: ChangeEvent<HTMLSelectElement>) =>
          onChange(event.target.value)
        }
        className={`h-7 appearance-none rounded-control border bg-card py-0 pr-7 pl-2 text-xs text-ink transition-colors hover:border-ink-muted ${
          value ? "border-rule-strong" : "border-rule"
        }`}
      >
        {children}
      </select>
      <Icon
        name="chevronDown"
        size={12}
        className="pointer-events-none absolute right-2 text-ink-muted"
      />
    </label>
  );
}

interface SearchFieldProps {
  // "band" sits on the black signage band; "plain" on the page ground.
  tone?: "band" | "plain";
  label: string;
  value: string;
  onChange(value: string): void;
  placeholder?: string;
}

// SearchField sits on the signage band.
const searchTones = {
  band: {
    icon: "text-on-band-muted",
    input:
      "w-64 border-transparent bg-band-field text-on-band placeholder:text-on-band-muted focus-visible:border-on-band-muted focus-visible:outline-on-band",
  },
  plain: {
    icon: "text-ink-muted",
    input:
      "w-full border-rule bg-card text-ink placeholder:text-ink-muted hover:border-ink-muted",
  },
};

export function SearchField({
  tone = "band",
  label,
  value,
  onChange,
  placeholder,
}: SearchFieldProps) {
  const style = searchTones[tone];
  return (
    <label className="relative flex min-w-0 flex-1 items-center">
      <span className="sr-only">{label}</span>
      <Icon
        name="search"
        size={14}
        className={`pointer-events-none absolute left-2.5 ${style.icon}`}
      />
      <input
        type="search"
        value={value}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
        className={`h-8 rounded-control border pr-2 pl-8 text-sm ${style.input}`}
      />
    </label>
  );
}

export function CheckboxField({
  label,
  title,
  checked,
  onChange,
}: {
  label: string;
  title?: string;
  checked: boolean;
  onChange(checked: boolean): void;
}) {
  return (
    <label
      title={title}
      className="flex cursor-pointer items-center gap-1.5 text-xs whitespace-nowrap text-ink-muted hover:text-ink"
    >
      <input
        type="checkbox"
        checked={checked}
        onChange={(event) => onChange(event.target.checked)}
        className="size-3.5 accent-[var(--fh-ink)]"
      />
      {label}
    </label>
  );
}
