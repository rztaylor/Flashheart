import type { TicketDetail } from "../../api/board";
import { Markdown } from "../../components/Markdown";
import { isImage } from "./AttachmentGrid";
import { Attachments } from "./Attachments";
import { Checklist } from "./Checklist";

// ReviewTab shows the ticket's screenshots above its review, with the How
// to Verify steps as a checklist (REV-3).
export function ReviewTab({
  detail,
  keys,
  onVerify,
}: {
  detail: TicketDetail;
  keys: Set<string>;
  // onVerify ticks or clears a step; absent when read-only.
  onVerify?(index: number, checked: boolean): Promise<boolean>;
}) {
  const screenshots = detail.attachmentFiles.filter(isImage);
  const review = detail.review;
  const markdown = (text: string, inline = false) =>
    text.trim() ? (
      <Markdown
        context={{ project: detail.project, ticket: detail.id }}
        keys={keys}
        inline={inline}
      >
        {text}
      </Markdown>
    ) : null;
  return (
    <div className="flex flex-col gap-6">
      {screenshots.length > 0 ? (
        <section aria-labelledby="screenshots-heading">
          <h3 id="screenshots-heading" className="mb-2 text-md heading-cut">
            Screenshots
          </h3>
          <Attachments attachments={screenshots} />
        </section>
      ) : null}
      {review && review.steps.length > 0 ? (
        <>
          {markdown(review.before)}
          <section
            aria-labelledby="verify-heading"
            className="flex flex-col gap-3"
          >
            <h2 id="verify-heading" className="text-lg heading-cut">
              How to Verify
            </h2>
            {markdown(review.intro)}
            <Checklist
              id="steps-heading"
              title="Steps"
              items={review.steps}
              render={(text) => markdown(text, true)}
              onToggle={onVerify}
            />
            {markdown(review.outro)}
          </section>
          {markdown(review.after)}
        </>
      ) : review ? (
        markdown(review.markdown)
      ) : null}
    </div>
  );
}
