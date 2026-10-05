import { Button } from "../../components/Button";
import { CheckboxField, SelectField } from "../../components/Field";
import { SegmentedControl } from "../../components/SegmentedControl";
import {
  type Filters,
  isFiltered,
  NO_WORKSTREAM,
  type StateFilter,
} from "../../model/filters";
import { PAINT_MODES, type PaintMode } from "../../model/paint";

export type Density = "compact" | "normal" | "detailed";

interface FilterBarProps {
  filters: Filters;
  options: { types: string[]; priorities: string[]; workstreams: string[] };
  onChange(filters: Filters): void;
  shown: number;
  total: number;
  density?: Density;
  onDensity?(density: Density): void;
  paint?: PaintMode;
  onPaint?(paint: PaintMode): void;
}

// FilterBar holds the board and table filters (VIEW-7), card density and
// what card colour shows (VIEW-6). Search lives in the header.
export function FilterBar({
  filters,
  options,
  onChange,
  shown,
  total,
  density,
  onDensity,
  paint,
  onPaint,
}: FilterBarProps) {
  const set = (patch: Partial<Filters>) => onChange({ ...filters, ...patch });
  const filtered = isFiltered(filters);
  return (
    <div className="flex flex-wrap items-center gap-x-4 gap-y-2 border-b border-rule bg-ground px-4 py-2">
      <SelectField
        label="Type"
        value={filters.type}
        onChange={(type) => set({ type })}
      >
        <option value="">All</option>
        {options.types.map((type) => (
          <option key={type} value={type}>
            {type}
          </option>
        ))}
      </SelectField>
      <SelectField
        label="Priority"
        value={filters.priority}
        onChange={(priority) => set({ priority })}
      >
        <option value="">All</option>
        {options.priorities.map((priority) => (
          <option key={priority} value={priority}>
            {priority}
          </option>
        ))}
      </SelectField>
      <SelectField
        label="Workstream"
        value={filters.workstream}
        onChange={(workstream) => set({ workstream })}
      >
        <option value="">All</option>
        <option value={NO_WORKSTREAM}>None</option>
        {options.workstreams.map((workstream) => (
          <option key={workstream} value={workstream}>
            {workstream}
          </option>
        ))}
      </SelectField>
      <SelectField
        label="State"
        value={filters.state === "all" ? "" : filters.state}
        onChange={(state) => set({ state: (state || "all") as StateFilter })}
      >
        <option value="">All</option>
        <option value="blocked">Blocked</option>
        <option value="unblocked">Not blocked</option>
        <option value="repair">Needs repair</option>
      </SelectField>
      <CheckboxField
        label="Hide later"
        title="Hide tickets tagged later-possibility"
        checked={filters.hideLater}
        onChange={(hideLater) => set({ hideLater })}
      />
      {filtered ? (
        <Button
          variant="quiet"
          className="h-7 px-1.5 text-xs"
          onClick={() =>
            onChange({
              ...filters,
              type: "",
              priority: "",
              workstream: "",
              state: "all",
              hideLater: false,
              query: "",
            })
          }
        >
          Clear filters
        </Button>
      ) : null}
      <span className="text-xs text-ink-muted" aria-live="polite">
        {filtered ? `${shown} of ${total} tickets` : `${total} tickets`}
      </span>
      {(paint && onPaint) || (density && onDensity) ? (
        <div className="ml-auto flex flex-wrap items-center gap-x-4 gap-y-2">
          {paint && onPaint ? (
            <SelectField
              label="Colour by"
              value={paint === "none" ? "" : paint}
              onChange={(value) => onPaint((value || "none") as PaintMode)}
            >
              {PAINT_MODES.map((mode) => (
                <option
                  key={mode.value}
                  value={mode.value === "none" ? "" : mode.value}
                >
                  {mode.label}
                </option>
              ))}
            </SelectField>
          ) : null}
          {density && onDensity ? (
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
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
