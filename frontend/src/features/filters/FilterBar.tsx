import type { ButtonHTMLAttributes, ReactNode } from "react";

import { Button } from "../../components/Button";
import { CheckboxField } from "../../components/Field";
import { filterHandlers, filterHint } from "../../components/FilterChip";
import { Popover } from "../../components/Popover";
import { SegmentedControl } from "../../components/SegmentedControl";
import {
  setColumnShown,
  toggleVirtual,
  VIRTUAL_COLUMNS,
} from "../../model/columns";
import {
  type Choice,
  choiceCount,
  choiceState,
  emptyFilters,
  type Filters,
  isFiltered,
  NO_WORKSTREAM,
  type StateFilter,
  toggleChoice,
} from "../../model/filters";
import { PAINT_MODES, type PaintMode } from "../../model/paint";
import { priorityLabel } from "../../model/status";

export type { Density } from "../../api/preferences";

import type {
  Density,
  HideableColumn,
  VirtualColumn,
} from "../../api/preferences";
import { Icon } from "../../components/Icon";

interface Option {
  value: string;
  label: string;
}

// ViewSettings are the Board's view options (VIEW-2, VIEW-6), kept behind
// the View options menu.
interface ViewSettings {
  density: Density;
  onDensity(density: Density): void;
  paint: PaintMode;
  onPaint(paint: PaintMode): void;
  // virtualColumns are the virtual columns shown on the board (VIEW-2).
  virtualColumns: VirtualColumn[];
  onVirtualColumns(columns: VirtualColumn[]): void;
  // hiddenColumns are the real columns hidden from the board (FH-41).
  hiddenColumns: HideableColumn[];
  onHiddenColumns(columns: HideableColumn[]): void;
}

interface FilterBarProps {
  filters: Filters;
  options: { types: string[]; priorities: string[]; workstreams: Option[] };
  onChange(filters: Filters): void;
  // view is the Board's view settings; absent on the Table.
  view?: ViewSettings;
}

const STATES: { value: StateFilter; label: string }[] = [
  { value: "all", label: "All states" },
  { value: "blocked", label: "Blocked" },
  { value: "unblocked", label: "Not blocked" },
  { value: "repair", label: "Needs repair" },
  { value: "working", label: "Agent working" },
];

// FilterBar is the view toolbar (ui-layout.md §1, FH-39): one button per
// filter, each named after what it filters and badged with how many values
// it holds, Clear filters while any filter applies, and on the Board one
// View options menu for the virtual columns, Colour by and density. The
// Board's chip row (FilterChips) edits the same filters. Search lives in
// the band, New ticket and the ticket count in the page header.
export function FilterBar({
  filters,
  options,
  onChange,
  view,
}: FilterBarProps) {
  const set = (patch: Partial<Filters>) => onChange({ ...filters, ...patch });
  const state = STATES.find((option) => option.value === filters.state);
  return (
    <div className="flex flex-wrap items-center gap-2 px-4 pb-3 md:px-6">
      <ChoiceMenu
        label="Type"
        options={options.types.map((type) => ({ value: type, label: type }))}
        choice={filters.type}
        onChange={(type) => set({ type })}
      />
      <ChoiceMenu
        label="Priority"
        options={options.priorities.map((priority) => ({
          value: priority,
          label: priorityLabel(priority),
        }))}
        choice={filters.priority}
        onChange={(priority) => set({ priority })}
      />
      <ChoiceMenu
        label="Workstream"
        options={[
          { value: NO_WORKSTREAM, label: "No workstream" },
          ...options.workstreams,
        ]}
        choice={filters.workstream}
        onChange={(workstream) => set({ workstream })}
      />
      <Popover
        label={filters.state === "all" ? "State" : `State: ${state?.label}`}
        active={filters.state !== "all"}
        button={
          <>
            <span>
              {filters.state === "all" ? "State" : `State: ${state?.label}`}
            </span>
            <Icon name="chevronDown" size={12} className="text-ink-muted" />
          </>
        }
      >
        {(close) =>
          STATES.map((option) => (
            <MenuItem
              key={option.value}
              pressed={filters.state === option.value}
              onClick={() => {
                set({ state: option.value });
                close();
              }}
            >
              {option.label}
            </MenuItem>
          ))
        }
      </Popover>
      {isFiltered(filters) ? (
        <Button
          variant="quiet"
          className="h-8 px-1.5 text-xs"
          onClick={() => onChange(emptyFilters)}
        >
          Clear filters
        </Button>
      ) : null}
      {view ? (
        <div className="ml-auto">
          <Popover
            label="View options"
            iconOnly
            align="end"
            button={<Icon name="more" size={18} />}
          >
            {() => <ViewOptions {...view} />}
          </Popover>
        </div>
      ) : null}
    </div>
  );
}

// ChoiceMenu is one filter's button and its list of values; a value toggles
// as its chip does.
function ChoiceMenu({
  label,
  options,
  choice,
  onChange,
}: {
  label: string;
  options: Option[];
  choice: Choice;
  onChange(choice: Choice): void;
}) {
  const count = choiceCount(choice);
  return (
    <Popover
      label={count > 0 ? `${label}: ${count} chosen` : label}
      active={count > 0}
      button={
        <>
          <span>{label}</span>
          {count > 0 ? (
            <span
              aria-hidden="true"
              className="rounded-full bg-select px-1.5 text-2xs font-semibold text-on-action tabular-nums"
            >
              {count}
            </span>
          ) : null}
          <Icon name="chevronDown" size={12} className="text-ink-muted" />
        </>
      }
    >
      {() => (
        <>
          {options.length === 0 ? (
            <p className="px-2 py-1.5 text-xs text-ink-faint">None here</p>
          ) : null}
          {options.map((option) => {
            const state = choiceState(choice, option.value);
            return (
              <MenuItem
                key={option.value}
                pressed={state === "included"}
                excluded={state === "excluded"}
                title={filterHint(option.label, state)}
                {...filterHandlers((exclude) =>
                  onChange(toggleChoice(choice, option.value, exclude)),
                )}
              >
                {option.label}
              </MenuItem>
            );
          })}
          {count > 0 ? (
            <button
              type="button"
              onClick={() => onChange({ include: [], exclude: [] })}
              className="mt-1 w-full border-t border-rule px-2 pt-1.5 pb-0.5 text-left text-xs text-ink-muted hover:text-ink"
            >
              Show all
            </button>
          ) : null}
        </>
      )}
    </Popover>
  );
}

function MenuItem({
  pressed,
  excluded,
  children,
  ...props
}: {
  pressed: boolean;
  excluded?: boolean;
  children: ReactNode;
} & Omit<ButtonHTMLAttributes<HTMLButtonElement>, "type">) {
  return (
    <button
      type="button"
      aria-pressed={pressed}
      className={`flex w-full items-center gap-2 rounded-inner px-2 py-1.5 text-left text-xs transition-colors hover:bg-ink/8 ${
        excluded ? "text-ink-faint line-through" : "text-ink"
      }`}
      {...props}
    >
      <Icon
        name={excluded ? "close" : "check"}
        size={14}
        className={pressed || excluded ? "" : "invisible"}
      />
      <span className="truncate">{children}</span>
      {excluded ? <span className="sr-only">, hidden</span> : null}
    </button>
  );
}

function ViewOptions({
  density,
  onDensity,
  paint,
  onPaint,
  virtualColumns,
  onVirtualColumns,
  hiddenColumns,
  onHiddenColumns,
}: ViewSettings) {
  return (
    <div className="flex w-max flex-col gap-3 p-1.5 text-xs">
      <fieldset className="flex flex-col gap-1.5">
        <legend className="mb-1.5 font-semibold text-ink">Show columns</legend>
        <CheckboxField
          label="Backlog"
          title="Show the Backlog column; hidden, the other columns get its width"
          checked={!hiddenColumns.includes("backlog")}
          onChange={(checked) =>
            onHiddenColumns(setColumnShown(hiddenColumns, "backlog", checked))
          }
        />
        {VIRTUAL_COLUMNS.map((column) => (
          <CheckboxField
            key={column.id}
            label={column.title}
            title={`Show a ${column.title} column that mirrors tickets from their real columns`}
            checked={virtualColumns.includes(column.id)}
            onChange={(checked) =>
              onVirtualColumns(
                toggleVirtual(virtualColumns, column.id, checked),
              )
            }
          />
        ))}
      </fieldset>
      <div className="flex flex-col gap-1.5">
        <span aria-hidden="true" className="font-semibold text-ink">
          Colour by
        </span>
        <SegmentedControl
          label="Colour by"
          value={paint}
          onChange={onPaint}
          options={PAINT_MODES}
        />
      </div>
      <div className="flex flex-col gap-1.5">
        <span aria-hidden="true" className="font-semibold text-ink">
          Density
        </span>
        <SegmentedControl
          label="Density"
          value={density}
          onChange={onDensity}
          options={[
            { value: "compact", label: "Compact" },
            { value: "normal", label: "Normal" },
            { value: "detailed", label: "Detailed" },
          ]}
        />
      </div>
    </div>
  );
}
