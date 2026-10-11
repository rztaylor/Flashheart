import { Icon } from "../../components/Icon";
import { stateToneClass, stateToneIcon } from "../../components/Pill";
import type { PipelineStage } from "../../model/overview";

const tickets = (count: number) =>
  `${count} ${count === 1 ? "ticket" : "tickets"}`;

// PipelineHeadline opens the Overview (VIEW-3, FH-63): the scope's board
// columns as a row of interlocking chevron cards that reads left to right,
// each a large count over the column's word in its state colour, linking
// to the Board. The cards are clipped, so their shadow and focus ring are
// filters on the wrapper that follow the shape.
export function PipelineHeadline({
  stages,
  boardHref,
}: {
  stages: PipelineStage[];
  boardHref: string;
}) {
  return (
    <nav
      aria-label="Pipeline"
      data-pipeline
      className="pt-1 [--chevron-point:0.75rem] sm:[--chevron-point:1.375rem] md:pt-0"
    >
      <ol className="flex [filter:var(--fh-shadow-chevron)]">
        {stages.map((stage, index) => (
          <li
            key={stage.column}
            className="relative min-w-0 flex-1 transition-transform duration-200 ease-out-expo not-first:-ml-[calc(var(--chevron-point)-0.3125rem)] focus-within:z-10 hover:z-10 hover:-translate-y-0.5 has-[:focus-visible]:chevron-ring"
          >
            <a
              href={boardHref}
              aria-label={`${stage.title}: ${tickets(stage.count)}`}
              data-stage={stage.column}
              data-tone={stage.tone}
              className={`flex h-full flex-col justify-start gap-1.5 py-3 pr-[calc(var(--chevron-point)+0.25rem)] outline-none sm:gap-2 sm:py-4 ${index === 0 ? "rounded-l-card pl-3 chevron-start sm:pl-5" : "pl-[calc(var(--chevron-point)+0.5rem)] chevron sm:pl-[calc(var(--chevron-point)+0.875rem)]"} ${stateToneClass[stage.tone]}`}
            >
              <span className="text-2xl leading-none tabular-nums display-cut sm:text-4xl lg:text-5xl">
                {stage.count}
              </span>
              <span className="flex items-start gap-1 text-2xs leading-tight font-semibold sm:items-center sm:text-sm">
                <Icon
                  name={stateToneIcon[stage.tone]}
                  size={14}
                  className="hidden sm:block"
                />
                {stage.title}
              </span>
            </a>
          </li>
        ))}
      </ol>
    </nav>
  );
}
