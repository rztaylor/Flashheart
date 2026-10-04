import { type KeyboardEvent, type ReactNode, useRef } from "react";

export interface TabItem {
  id: string;
  label: ReactNode;
}

// Element ids shared by a tab and its panel.
export const tabId = (prefix: string, id: string) => `${prefix}-tab-${id}`;
export const panelId = (prefix: string, id: string) => `${prefix}-panel-${id}`;

interface TabsProps {
  // idPrefix joins each tab to the panel that renders it (see panelId).
  idPrefix: string;
  label: string;
  items: TabItem[];
  selected: string;
  onSelect(id: string): void;
}

// Tabs is an ARIA tablist with arrow-key movement; pair it with TabPanel.
export function Tabs({
  idPrefix,
  label,
  items,
  selected,
  onSelect,
}: TabsProps) {
  const refs = useRef<(HTMLButtonElement | null)[]>([]);
  const onKey = (event: KeyboardEvent, index: number) => {
    const delta =
      event.key === "ArrowRight" ? 1 : event.key === "ArrowLeft" ? -1 : 0;
    const target =
      event.key === "Home"
        ? 0
        : event.key === "End"
          ? items.length - 1
          : delta
            ? (index + delta + items.length) % items.length
            : -1;
    if (target < 0) return;
    event.preventDefault();
    const item = items[target];
    if (item) {
      onSelect(item.id);
      refs.current[target]?.focus();
    }
  };
  return (
    <div
      role="tablist"
      aria-label={label}
      className="flex gap-5 border-b border-rule"
    >
      {items.map((item, index) => {
        const active = item.id === selected;
        return (
          <button
            key={item.id}
            ref={(node) => {
              refs.current[index] = node;
            }}
            id={tabId(idPrefix, item.id)}
            type="button"
            role="tab"
            aria-selected={active}
            aria-controls={panelId(idPrefix, item.id)}
            tabIndex={active ? 0 : -1}
            onClick={() => onSelect(item.id)}
            onKeyDown={(event) => onKey(event, index)}
            className={`-mb-px border-b-3 py-2 text-sm station-sign transition-colors ${
              active
                ? "border-rule-strong text-ink"
                : "border-transparent text-ink-muted hover:text-ink"
            }`}
          >
            {item.label}
          </button>
        );
      })}
    </div>
  );
}
