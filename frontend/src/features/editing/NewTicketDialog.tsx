import { type FormEvent, useId, useState } from "react";

import { COLUMNS, type Column, type ProjectSummary } from "../../api/board";
import type { AuthenticatedFetch } from "../../api/client";
import { type Created, createTicket, keysInUse } from "../../api/edit";
import { Button } from "../../components/Button";
import { Dialog } from "../../components/Dialog";
import { FormField, Select, TextArea, TextInput } from "../../components/Field";

const TYPES = ["feature", "bug", "infra", "test", "refactor", "docs", "spike"];
const PRIORITIES = ["high", "medium", "low"];

const ticketCount = (project: ProjectSummary) =>
  COLUMNS.reduce((sum, column) => sum + project.counts[column.id], 0);

// NewTicketDialog creates a ticket from the template with the project's next
// id (EDIT-5). A project with no key and no tickets chooses its key here
// first (KEY-5).
export function NewTicketDialog({
  fetcher,
  projects,
  project: initialProject,
  onCreated,
  onClose,
}: {
  fetcher: AuthenticatedFetch;
  projects: ProjectSummary[];
  project: string;
  onCreated(created: Created): void;
  onClose(): void;
}) {
  const formId = useId();
  const [projectName, setProjectName] = useState(
    initialProject || projects[0]?.name || "",
  );
  const project = projects.find((item) => item.name === projectName);
  const choosingKey = project
    ? project.keyDerived && ticketCount(project) === 0
    : false;
  const [title, setTitle] = useState("");
  const [type, setType] = useState("feature");
  const [priority, setPriority] = useState("medium");
  const [status, setStatus] = useState<Column>("backlog");
  const [workstream, setWorkstream] = useState("");
  const [description, setDescription] = useState("");
  const [criteria, setCriteria] = useState("");
  const [key, setKey] = useState(project?.key ?? "");
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!project || title.trim() === "") return;
    setSaving(true);
    setError("");
    try {
      const created = await createTicket(fetcher, project.name, {
        title: title.trim(),
        type,
        priority,
        status,
        workstream,
        description,
        criteria: criteria
          .split("\n")
          .map((line) => line.trim())
          .filter(Boolean),
        dependsOn: [],
        tags: [],
        key: choosingKey ? key.trim().toUpperCase() : "",
      });
      onCreated(created);
    } catch (caught) {
      const inUse = keysInUse(caught);
      setError(
        inUse
          ? `The key ${key.trim().toUpperCase()} is taken. Keys in use: ${inUse.join(", ")}.`
          : caught instanceof Error
            ? caught.message
            : "The ticket could not be created.",
      );
      setSaving(false);
    }
  };

  return (
    <Dialog
      title="New ticket"
      onClose={onClose}
      actions={
        <>
          <Button onClick={onClose}>Cancel</Button>
          <Button
            variant="primary"
            type="submit"
            form={formId}
            disabled={saving || !project || title.trim() === ""}
          >
            {saving ? "Creating…" : "Create ticket"}
          </Button>
        </>
      }
    >
      <form id={formId} onSubmit={submit} className="flex flex-col gap-3">
        {projects.length > 1 && !initialProject ? (
          <FormField label="Project">
            <Select value={projectName} onChange={setProjectName}>
              {projects.map((item) => (
                <option key={item.name} value={item.name}>
                  {item.displayName}
                </option>
              ))}
            </Select>
          </FormField>
        ) : null}
        {choosingKey ? (
          <FormField
            label="Ticket key for this project"
            hint="2–5 letters people will say in conversation, like FH. It is fixed once the first ticket exists."
          >
            <TextInput
              value={key}
              onChange={(value) => setKey(value.toUpperCase())}
              maxLength={10}
              spellCheck={false}
              className="w-32 font-semibold tracking-[0.02em] uppercase"
            />
          </FormField>
        ) : null}
        <FormField label="Title">
          <TextInput
            value={title}
            onChange={setTitle}
            required
            autoFocus
            maxLength={300}
          />
        </FormField>
        <div className="grid grid-cols-3 gap-3">
          <FormField label="Type">
            <Select value={type} onChange={setType}>
              {TYPES.map((item) => (
                <option key={item}>{item}</option>
              ))}
            </Select>
          </FormField>
          <FormField label="Priority">
            <Select value={priority} onChange={setPriority}>
              {PRIORITIES.map((item) => (
                <option key={item}>{item}</option>
              ))}
            </Select>
          </FormField>
          <FormField label="Column">
            <Select
              value={status}
              onChange={(value) => setStatus(value as Column)}
            >
              {COLUMNS.map((column) => (
                <option key={column.id} value={column.id}>
                  {column.title}
                </option>
              ))}
            </Select>
          </FormField>
        </div>
        {project && project.workstreams.length > 0 ? (
          <FormField label="Workstream">
            <Select value={workstream} onChange={setWorkstream}>
              <option value="">None</option>
              {project.workstreams.map((item) => (
                <option key={item.slug} value={item.slug}>
                  {item.title}
                </option>
              ))}
            </Select>
          </FormField>
        ) : null}
        <FormField label="Description">
          <TextArea value={description} onChange={setDescription} rows={3} />
        </FormField>
        <FormField label="Acceptance criteria" hint="One per line.">
          <TextArea value={criteria} onChange={setCriteria} rows={3} />
        </FormField>
        {error ? (
          <p role="alert" className="text-sm text-danger">
            {error}
          </p>
        ) : null}
      </form>
    </Dialog>
  );
}
