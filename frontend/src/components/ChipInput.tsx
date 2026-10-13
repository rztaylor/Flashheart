import { type ReactNode, useId, useState } from "react";

import { Icon } from "./Icon";

// ChipList is a list edited as chips, and the text typed but not yet
// added: the caller holds both, so a save can include the typed text.
export interface ChipList {
  values: string[];
  text: string;
}

// withTyped is the list with its typed text added (normalized), as it is
// saved.
export function withTyped(
  list: ChipList,
  normalize: (text: string) => string = (text) => text.trim(),
): string[] {
  const typed = normalize(list.text);
  return typed && !list.values.includes(typed)
    ? [...list.values, typed]
    : list.values;
}

// ChipInput edits a list as chips (tags, ticket ids): each chip removable
// by its button, then a field where Enter or a comma adds what was typed.
// suggestions offer values to pick, as the field's list. check refuses a
// value, saying why; normalize tidies one before it is added.
export function ChipInput({
  label,
  list,
  onChange,
  addLabel,
  removeLabel,
  suggestions,
  normalize = (text) => text.trim(),
  check,
  renderChip = (value) => value,
  placeholder,
  mono,
}: {
  label: string;
  list: ChipList;
  onChange(list: ChipList): void;
  addLabel: string;
  removeLabel(value: string): string;
  suggestions?: { value: string; label: string }[];
  normalize?(text: string): string;
  check?(value: string): string | undefined;
  renderChip?(value: string): ReactNode;
  placeholder?: string;
  mono?: boolean;
}) {
  const id = useId();
  const [problem, setProblem] = useState("");
  // add adds text as a chip, or keeps it typed and says why it cannot be.
  const add = (text: string) => {
    const value = normalize(text);
    const wrong = value ? check?.(value) : undefined;
    setProblem(wrong ?? "");
    if (!value || wrong) {
      onChange({ ...list, text });
      return;
    }
    onChange({
      values: list.values.includes(value)
        ? list.values
        : [...list.values, value],
      text: "",
    });
  };
  return (
    <fieldset className="flex min-w-0 flex-col gap-1.5">
      <legend className="sr-only">{label}</legend>
      {list.values.length > 0 ? (
        <ul className="flex flex-wrap gap-1.5">
          {list.values.map((value) => (
            <li
              key={value}
              className="flex items-center gap-0.5 rounded-pill border border-rule bg-card py-0.5 pr-0.5 pl-2 text-xs text-ink"
            >
              <span className={mono ? "font-mono" : ""}>
                {renderChip(value)}
              </span>
              <button
                type="button"
                aria-label={removeLabel(value)}
                title={removeLabel(value)}
                onClick={() =>
                  onChange({
                    ...list,
                    values: list.values.filter((item) => item !== value),
                  })
                }
                className="grid size-5 place-items-center rounded-pill text-ink-muted hover:bg-well hover:text-ink"
              >
                <Icon name="close" size={10} />
              </button>
            </li>
          ))}
        </ul>
      ) : null}
      <input
        aria-label={addLabel}
        aria-describedby={problem ? `${id}-problem` : undefined}
        list={suggestions ? `${id}-list` : undefined}
        value={list.text}
        placeholder={placeholder}
        spellCheck={false}
        autoComplete="off"
        onChange={(event) => {
          const text = event.target.value;
          // A comma typed adds the value before it.
          if (text.endsWith(",")) add(text.slice(0, -1));
          else {
            setProblem("");
            onChange({ ...list, text });
          }
        }}
        onKeyDown={(event) => {
          if (event.key !== "Enter" || event.ctrlKey || event.metaKey) return;
          event.preventDefault();
          add(list.text);
        }}
        className={`h-8 w-full rounded-control border border-rule bg-card px-2.5 text-sm text-ink transition-colors placeholder:text-ink-muted hover:border-ink-muted ${mono ? "font-mono text-xs" : ""}`}
      />
      {suggestions ? (
        <datalist id={`${id}-list`}>
          {suggestions.map((item) => (
            <option key={item.value} value={item.value}>
              {item.label}
            </option>
          ))}
        </datalist>
      ) : null}
      {problem ? (
        <p id={`${id}-problem`} className="text-xs text-danger">
          {problem}
        </p>
      ) : null}
    </fieldset>
  );
}
