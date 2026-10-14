import type { Run, TimelineEntry } from "../../api/runs";
import { QuestionCard } from "../../components/QuestionCard";
import { TailText } from "../../components/TailText";
import { compactTimeline, describeEntry } from "../../model/runs";
import { absoluteTime, runningTime } from "../../model/time";

// MAX_TIMELINE bounds the activity lines shown for one run.
const MAX_TIMELINE = 40;

// RunDetail is what a run did: its open questions, the files it edited and
// its recent activity, newest first. A ticket's plan is its criteria and
// handoff (D51), so a run shows none. Agent text is shown as data.
export function RunDetail({
  run,
  timeline,
  now,
  headingLevel = 3,
  onAnswer,
}: {
  run: Run;
  // timeline is the run's activity, when it has been loaded.
  timeline?: TimelineEntry[];
  now: Date;
  // headingLevel places the section heads under the caller's heading.
  headingLevel?: 3 | 4 | 5;
  // onAnswer answers one of the run's questions (CARD-6); questions are
  // shown only where they can be answered.
  onAnswer?(question: string, answer: string): Promise<string | undefined>;
}) {
  const activity = timeline
    ? compactTimeline(timeline).slice(0, MAX_TIMELINE)
    : [];
  const questions = onAnswer
    ? run.questions.filter((question) => !question.delivered)
    : [];
  return (
    <div className="@container flex flex-col gap-5">
      {questions.length > 0 ? (
        <DetailSection level={headingLevel} title="Questions for you">
          <div className="flex max-w-2xl flex-col gap-2">
            {questions.map((question) => (
              <QuestionCard
                key={question.id}
                question={question}
                asker={question.runShort}
                now={now}
                headingLevel={headingLevel === 3 ? 4 : 5}
                onAnswer={
                  onAnswer ? (text) => onAnswer(question.id, text) : undefined
                }
              />
            ))}
          </div>
        </DetailSection>
      ) : null}
      <div className="grid gap-x-8 gap-y-5 text-xs @2xl:grid-cols-[minmax(0,1fr)_minmax(0,1.2fr)]">
        <div className="flex min-w-0 flex-col gap-5">
          <DetailSection
            level={headingLevel}
            title={
              run.dirty
                ? `Edited since the last checkpoint (${run.edits})`
                : "Edited files"
            }
          >
            {run.files.length === 0 ? (
              <p className="text-ink-muted">No files edited.</p>
            ) : (
              <ul className="flex flex-col gap-0.5 font-mono text-2xs text-ink">
                {run.files.map((file) => (
                  <TailText key={file} as="li">
                    {file}
                  </TailText>
                ))}
              </ul>
            )}
          </DetailSection>
          <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-0.5 text-2xs text-ink-muted">
            <dt>Run</dt>
            <TailText as="dd" className="text-ink">
              {run.id}
            </TailText>
            {run.branch ? (
              <>
                <dt>Branch</dt>
                <TailText as="dd" className="font-mono">
                  {run.branch}
                </TailText>
              </>
            ) : null}
            {run.worktree ? (
              <>
                <dt>Worktree</dt>
                <TailText as="dd" className="font-mono">
                  {run.worktree}
                </TailText>
              </>
            ) : null}
            <dt>Started</dt>
            <dd>{absoluteTime(run.started)}</dd>
            {run.ended ? (
              <>
                <dt>Ended</dt>
                <dd>{absoluteTime(run.ended)}</dd>
              </>
            ) : null}
            <dt>Tools</dt>
            <dd>{run.tools}</dd>
          </dl>
        </div>
        <DetailSection level={headingLevel} title="Activity">
          {!timeline ? (
            <p className="text-ink-muted">Loading activity…</p>
          ) : activity.length === 0 ? (
            <p className="text-ink-muted">No activity recorded.</p>
          ) : (
            <ol className="relative flex flex-col">
              {activity.map(({ entry, count }, index) => (
                <li
                  // biome-ignore lint/suspicious/noArrayIndexKey: entries are folded and newest first, but hold no state, so index keys are harmless.
                  key={`${entry.time}-${index}`}
                  className="relative flex items-baseline gap-3 py-[3px] pl-4 before:absolute before:top-0 before:bottom-0 before:left-[3px] before:w-px before:bg-rule last:before:bottom-1/2 first:before:top-1/2"
                >
                  <span
                    aria-hidden="true"
                    className={`absolute top-1/2 left-0 size-[7px] -translate-y-1/2 rounded-full border border-ink ${
                      entry.kind === "permission.requested" ||
                      entry.kind === "turn.start"
                        ? "bg-ink"
                        : "bg-card"
                    }`}
                  />
                  <span
                    className={`min-w-0 flex-1 truncate ${entry.failed ? "text-ink-muted line-through decoration-ink-faint" : "text-ink"}`}
                    title={describeEntry(entry)}
                  >
                    {describeEntry(entry)}
                    {count > 1 ? (
                      <span className="text-ink-muted"> ×{count}</span>
                    ) : null}
                  </span>
                  <time
                    dateTime={entry.time}
                    title={absoluteTime(entry.time)}
                    className="shrink-0 text-2xs text-ink-muted"
                  >
                    {runningTime(entry.time, now)}
                  </time>
                </li>
              ))}
            </ol>
          )}
        </DetailSection>
      </div>
    </div>
  );
}

function DetailSection({
  title,
  level,
  children,
}: {
  title: string;
  level: 3 | 4 | 5;
  children: React.ReactNode;
}) {
  const Heading = level === 3 ? "h3" : level === 4 ? "h4" : "h5";
  return (
    <section className="min-w-0">
      <Heading className="mb-2 text-sm heading-cut">{title}</Heading>
      {children}
    </section>
  );
}
