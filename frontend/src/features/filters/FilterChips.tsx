import {
  type ReactNode,
  useCallback,
  useLayoutEffect,
  useRef,
  useState,
} from "react";

import type { Card, WorkstreamBrief } from "../../api/board";
import { FilterChip } from "../../components/FilterChip";
import { LineBullet } from "../../components/LineBullet";
import { RunStateMark } from "../../components/RunState";
import {
  type ChoiceKey,
  chipWorkstreams,
  choiceState,
  type Filters,
  fittingCount,
  toggleChoice,
  toggleState,
} from "../../model/filters";
import type { Line } from "../../model/lines";
import {
  PAINT_MODES,
  type Paint,
  type PaintMode,
  paintVars,
} from "../../model/paint";

interface FilterChipsProps {
  // workstreams are the project's lines; absent in the all scope.
  workstreams?: WorkstreamBrief[];
  lines: Map<string, Line>;
  // cards are the scope's tickets before filtering, which order the lines.
  cards: Card[];
  paint: PaintMode;
  // paints are the Colour by values present before filtering.
  paints: Paint[];
  filters: Filters;
  onChange(filters: Filters): void;
}

// The chip gap, gap-2 in the rows below.
const GAP = 8;

// FilterChips is the Board's chip row (FH-39): a Needs you chip while a
// ticket needs you (FH-44), the project's workstreams, busiest first, and
// the colour key for the Colour by setting, all as filters. A click shows only a value, Cmd or Ctrl click hides it, and a
// click on a chosen chip restores it. The row keeps to one line: chips that
// do not fit stay out of view, colour chips first; the toolbar menus list
// every value.
export function FilterChips({
  workstreams = [],
  lines,
  cards,
  paint,
  paints,
  filters,
  onChange,
}: FilterChipsProps) {
  const chosen = [...filters.workstream.include, ...filters.workstream.exclude];
  const shown = chipWorkstreams(workstreams, cards, chosen);
  const colourKey: ChoiceKey | undefined = paint === "none" ? undefined : paint;
  // Needs you stays while chosen, so it can be restored once nothing does.
  const needsYou =
    filters.needsYou !== "idle" || cards.some((card) => card.needsYou);
  if (!needsYou && shown.length === 0 && (!colourKey || paints.length === 0))
    return null;
  const toggle = (key: ChoiceKey, value: string, exclude: boolean) =>
    onChange({ ...filters, [key]: toggleChoice(filters[key], value, exclude) });
  const modeName = PAINT_MODES.find((mode) => mode.value === paint)?.label;
  return (
    // Both groups keep to one line. Workstreams keep their width up to the
    // row's; only the colour chips shrink, so they give way first.
    <div className="flex items-center gap-x-6 overflow-hidden border-t border-rule px-4 py-2.5 md:px-6">
      {needsYou ? (
        <fieldset
          aria-label="Filter by tickets that need you"
          className="-m-1 flex shrink-0 p-1"
        >
          <FilterChip
            name="Needs you"
            bullet={<NeedsYouBullet />}
            state={filters.needsYou}
            onToggle={(exclude) =>
              onChange({
                ...filters,
                needsYou: toggleState(filters.needsYou, exclude),
              })
            }
          />
        </fieldset>
      ) : null}
      {shown.length > 0 ? (
        <fieldset className="flex max-w-full min-w-0 shrink-0 items-center gap-2">
          <legend className="sr-only">
            Filter by workstream: click to show only one, Cmd or Ctrl click to
            hide it
          </legend>
          <span
            aria-hidden="true"
            className="mr-1 shrink-0 text-sm heading-cut"
          >
            Workstreams
          </span>
          <OneLine count={shown.length}>
            {(fits) =>
              shown.map((workstream, index) => (
                <FilterChip
                  key={workstream.slug}
                  name={workstream.title}
                  bullet={
                    <LineBullet line={lines.get(workstream.slug)} size="sm" />
                  }
                  state={choiceState(filters.workstream, workstream.slug)}
                  hidden={index >= fits}
                  onToggle={(exclude) =>
                    toggle("workstream", workstream.slug, exclude)
                  }
                />
              ))
            }
          </OneLine>
        </fieldset>
      ) : null}
      {colourKey && paints.length > 0 ? (
        <fieldset
          aria-label={`Filter by ${modeName?.toLowerCase()}`}
          className="ml-auto flex min-w-0 items-center"
        >
          <OneLine count={paints.length}>
            {(fits) =>
              paints.map((entry, index) => (
                <FilterChip
                  key={entry.token + entry.value}
                  name={entry.label}
                  bullet={<PaintBullet paint={entry} />}
                  state={choiceState(filters[colourKey], entry.value)}
                  hidden={index >= fits}
                  onToggle={(exclude) =>
                    toggle(colourKey, entry.value, exclude)
                  }
                />
              ))
            }
          </OneLine>
        </fieldset>
      ) : null}
    </div>
  );
}

// PaintBullet is a colour chip's bullet: the round mark of a workstream
// chip, filled with the Colour by paint instead of a line colour.
function PaintBullet({ paint }: { paint: Paint }) {
  return (
    <span
      aria-hidden="true"
      className="inline-block size-5 shrink-0 rounded-full bg-(--paint) shadow-[inset_0_0_0_1px_var(--fh-casing)]"
      style={paintVars(paint)}
    />
  );
}

// NeedsYouBullet is the Needs you chip's bullet: the run mark on the
// attention colour, as the band's Needs you pill shows it.
function NeedsYouBullet() {
  return (
    <span
      aria-hidden="true"
      className="inline-flex size-5 shrink-0 items-center justify-center rounded-full bg-attention text-on-attention"
    >
      <RunStateMark state="needs-you" size={8} />
    </span>
  );
}

// OneLine lays a group of chips out on a single line, as wide as its chips
// when there is room, and tells them how many fit,
// measuring after every render and whenever the row's width changes. The
// row pads by 4px (p-1, undone by -m-1) so focus and selection rings are not
// clipped.
function OneLine({
  count,
  children,
}: {
  count: number;
  children(fits: number): ReactNode;
}) {
  const row = useRef<HTMLDivElement>(null);
  const [fits, setFits] = useState(count);
  const measure = useCallback(() => {
    const element = row.current;
    if (!element) return;
    const widths = [...element.children].map(
      (child) => (child as HTMLElement).offsetWidth,
    );
    setFits(fittingCount(widths, element.clientWidth - 8, GAP));
  }, []);
  useLayoutEffect(measure);
  useLayoutEffect(() => {
    if (!row.current) return;
    const observer = new ResizeObserver(measure);
    observer.observe(row.current);
    return () => observer.disconnect();
  }, [measure]);
  return (
    <div
      ref={row}
      className="-m-1 flex min-w-0 flex-nowrap items-center gap-2 overflow-hidden p-1"
    >
      {children(fits)}
    </div>
  );
}
