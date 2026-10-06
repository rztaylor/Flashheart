import { useId } from "react";

interface SegmentedControlProps<T extends string> {
  label: string;
  options: { value: T; label: string }[];
  value: T;
  onChange(value: T): void;
}

// SegmentedControl is a radio group drawn as joined segments.
export function SegmentedControl<T extends string>({
  label,
  options,
  value,
  onChange,
}: SegmentedControlProps<T>) {
  const name = useId();
  return (
    <fieldset className="flex items-center gap-2">
      <legend className="sr-only">{label}</legend>
      <div className="flex rounded-control border border-rule bg-card p-0.5 shadow-card">
        {options.map((option) => (
          <label
            key={option.value}
            className={`cursor-pointer rounded-[0.375rem] px-2.5 py-1 text-xs transition-colors has-focus-visible:outline-2 has-focus-visible:outline-focus ${
              option.value === value
                ? "bg-select-surface font-semibold text-ink shadow-[inset_0_0_0_1px_var(--fh-select)]"
                : "text-ink-muted hover:text-ink"
            }`}
          >
            <input
              type="radio"
              name={name}
              value={option.value}
              checked={option.value === value}
              onChange={() => onChange(option.value)}
              className="sr-only"
            />
            {option.label}
          </label>
        ))}
      </div>
    </fieldset>
  );
}
