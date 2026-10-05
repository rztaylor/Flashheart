import { useState } from "react";

import { Button } from "../../components/Button";
import { Dialog } from "../../components/Dialog";
import { FormField, TextArea } from "../../components/Field";
import { StateNote } from "../../components/StateNote";
import type { BlockedMove } from "./useEditing";

// BlockedMoveDialog asks for a reason before a blocked ticket starts
// (EDIT-2); the reason is recorded in the ticket's Notes.
export function BlockedMoveDialog({
  move,
  onConfirm,
  onCancel,
}: {
  move: BlockedMove;
  onConfirm(reason: string): void;
  onCancel(): void;
}) {
  const [reason, setReason] = useState("");
  const ready = reason.trim() !== "";
  return (
    <Dialog
      title={`Start ${move.ticket.id} while it is blocked?`}
      onClose={onCancel}
      actions={
        <>
          <Button onClick={onCancel}>Keep it where it is</Button>
          <Button
            variant="primary"
            disabled={!ready}
            onClick={() => onConfirm(reason.trim())}
          >
            Start anyway
          </Button>
        </>
      }
    >
      <p className="mb-3 font-medium">{move.ticket.title}</p>
      <div className="mb-4 flex flex-col gap-1.5">
        {move.reasons.map((reason) => (
          <StateNote key={reason} kind="blocked">
            {reason}
          </StateNote>
        ))}
      </div>
      <FormField
        label="Why start it now?"
        hint="Recorded in the ticket's Notes so the next session knows."
      >
        <TextArea value={reason} onChange={setReason} rows={3} autoFocus />
      </FormField>
    </Dialog>
  );
}
