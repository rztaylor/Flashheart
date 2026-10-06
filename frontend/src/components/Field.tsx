import {
  type ChangeEvent,
  cloneElement,
  type ReactElement,
  type ReactNode,
  useId,
} from "react";

import { Icon } from "./Icon";

interface SelectFieldProps {
  label: string;
  // plain drops the "applied" ring, for settings (Colour by) that always
  // have a value, as opposed to filters.
  plain?: boolean;
  value: string;
  onChange(value: string): void;
  children: ReactNode;
}

// SelectField is a labelled native select in the board's control style.
export function SelectField({
  label,
  plain,
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
        className={`h-8 appearance-none rounded-control border bg-card py-0 pr-7 pl-2.5 text-xs font-medium text-ink transition-colors hover:border-ink-muted ${
          value && !plain ? "border-select ring-1 ring-select" : "border-rule"
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
      "w-72 border-band-rule bg-band-field text-on-band placeholder:text-on-band-muted focus-visible:border-on-band-muted focus-visible:outline-on-band",
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
        className={`h-9 rounded-control border pr-2 pl-8 text-sm ${style.input}`}
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
        className="size-3.5 accent-[var(--fh-select)]"
      />
      {label}
    </label>
  );
}

const formControl =
  "w-full rounded-control border border-rule bg-card px-2.5 text-sm text-ink transition-colors placeholder:text-ink-muted hover:border-ink-muted";

// FormField is a stacked label and control for dialog and edit forms. The
// hint below the control is linked to it with aria-describedby, so it never
// becomes part of the control's name.
export function FormField({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: ReactElement<{ id?: string; "aria-describedby"?: string }>;
}) {
  const id = useId();
  const control = cloneElement(children, {
    id,
    "aria-describedby": hint ? `${id}-hint` : undefined,
  });
  return (
    <div className="flex flex-col gap-1 text-xs text-ink-muted">
      <label htmlFor={id} className="font-medium text-ink">
        {label}
      </label>
      {control}
      {hint ? <span id={`${id}-hint`}>{hint}</span> : null}
    </div>
  );
}

export function TextInput({
  value,
  onChange,
  ...props
}: {
  value: string;
  onChange(value: string): void;
  id?: string;
  "aria-describedby"?: string;
  type?: "text" | "date";
  required?: boolean;
  placeholder?: string;
  maxLength?: number;
  autoFocus?: boolean;
  spellCheck?: boolean;
  className?: string;
}) {
  return (
    <input
      {...props}
      value={value}
      onChange={(event) => onChange(event.target.value)}
      className={`h-8 ${formControl} ${props.className ?? ""}`}
    />
  );
}

export function TextArea({
  value,
  onChange,
  rows = 4,
  mono,
  ...props
}: {
  value: string;
  onChange(value: string): void;
  rows?: number;
  mono?: boolean;
  maxLength?: number;
  autoFocus?: boolean;
  placeholder?: string;
  spellCheck?: boolean;
  "aria-label"?: string;
  id?: string;
  "aria-describedby"?: string;
}) {
  return (
    <textarea
      {...props}
      rows={rows}
      value={value}
      onChange={(event) => onChange(event.target.value)}
      className={`py-1.5 leading-snug ${formControl} ${mono ? "font-mono text-xs" : ""}`}
    />
  );
}

export function Select({
  value,
  onChange,
  children,
  ...props
}: {
  value: string;
  onChange(value: string): void;
  children: ReactNode;
  id?: string;
  "aria-describedby"?: string;
  "aria-label"?: string;
}) {
  return (
    <span className="relative flex items-center">
      <select
        {...props}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className={`h-8 appearance-none pr-8 ${formControl}`}
      >
        {children}
      </select>
      <Icon
        name="chevronDown"
        size={12}
        className="pointer-events-none absolute right-2.5 text-ink-muted"
      />
    </span>
  );
}
