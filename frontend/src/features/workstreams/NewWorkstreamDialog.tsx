import { type FormEvent, useId, useState } from "react";
import type { ProjectSummary } from "../../api/board";
import type { AuthenticatedFetch } from "../../api/client";
import { ApiError } from "../../api/client";
import { createWorkstream } from "../../api/edit";
import { Button } from "../../components/Button";
import { Dialog } from "../../components/Dialog";
import { FormField, Select, TextArea, TextInput } from "../../components/Field";
import { ProjectField } from "../../components/ProjectField";
import { priorityMark } from "../../components/TicketMarks";
import { messageOf } from "../../model/errors";
import { takenWorkstream } from "../../model/lines";
import { PRIORITIES } from "../../model/tickets";

// CreatedWorkstreamRef names the workstream the dialog created.
export interface CreatedWorkstreamRef {
  project: string;
  slug: string;
  title: string;
}

// NewWorkstreamDialog creates a workstream from board-format's template
// (FH-17): a title (its slug is the server's), a goal, a priority and the
// project's workstreams it depends on. Tickets join it from their own
// workstream field. Its keys and focus are New ticket's. A title one of
// the project's workstreams has is named at the field and not sent; the
// server's refusal of one the page has not heard of yet shows there too
// (FH-163, D76).
export function NewWorkstreamDialog({
  fetcher,
  projects,
  project: initialProject,
  onCreated,
  onClose,
}: {
  fetcher: AuthenticatedFetch;
  projects: ProjectSummary[];
  // project is the scope's project; empty in the all scope, which asks.
  project: string;
  onCreated(created: CreatedWorkstreamRef): void;
  onClose(): void;
}) {
  const formId = useId();
  const [projectName, setProjectName] = useState(
    initialProject || projects[0]?.name || "",
  );
  const project = projects.find((item) => item.name === projectName);
  const [title, setTitle] = useState("");
  const [goal, setGoal] = useState("");
  const [priority, setPriority] = useState("medium");
  const [dependsOn, setDependsOn] = useState<string[]>([]);
  const [error, setError] = useState("");
  // refused is the server's refusal of a title, kept while it is unchanged.
  const [refused, setRefused] = useState({ title: "", message: "" });
  const [saving, setSaving] = useState(false);
  const named = title.trim() !== "";
  const taken = project
    ? takenWorkstream(project.workstreams, title)
    : undefined;
  const titleError = taken
    ? `${project?.displayName} already has a workstream called ${taken.title}. Choose another title.`
    : named && refused.title === title
      ? refused.message
      : "";

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (!project || !named || titleError) return;
    setSaving(true);
    setError("");
    try {
      const created = await createWorkstream(fetcher, project.name, {
        title: title.trim(),
        goal,
        priority,
        dependsOnWorkstreams: dependsOn,
      });
      onCreated({
        project: project.name,
        slug: created.slug,
        title: title.trim(),
      });
    } catch (caught) {
      const message = messageOf(caught, "The workstream could not be created.");
      if (caught instanceof ApiError && caught.code === "title_taken") {
        setRefused({ title, message });
      } else {
        setError(message);
      }
      setSaving(false);
    }
  };

  const toggle = (slug: string, on: boolean) =>
    setDependsOn((current) =>
      on ? [...current, slug] : current.filter((item) => item !== slug),
    );

  return (
    <Dialog
      title="New workstream"
      onClose={onClose}
      actions={
        <>
          <Button onClick={onClose}>Cancel</Button>
          <Button
            variant="primary"
            type="submit"
            form={formId}
            disabled={saving || !project || !named || titleError !== ""}
          >
            {saving ? "Creating…" : "Create workstream"}
          </Button>
        </>
      }
    >
      <form id={formId} onSubmit={submit} className="flex flex-col gap-3">
        <ProjectField
          projects={projects}
          scope={initialProject}
          value={projectName}
          onChange={(name) => {
            setProjectName(name);
            // Dependencies belong to the chosen project.
            setDependsOn([]);
          }}
        />
        <FormField label="Title" error={titleError}>
          <TextInput
            value={title}
            onChange={setTitle}
            required
            autoFocus
            maxLength={300}
          />
        </FormField>
        <FormField label="Goal" hint="The shared outcome its tickets deliver.">
          <TextArea value={goal} onChange={setGoal} rows={3} />
        </FormField>
        <div className="grid grid-cols-2 gap-3">
          <FormField label="Priority">
            <Select
              value={priority}
              onChange={setPriority}
              leading={priorityMark(priority)}
            >
              {PRIORITIES.map((item) => (
                <option key={item}>{item}</option>
              ))}
            </Select>
          </FormField>
        </div>
        {project && project.workstreams.length > 0 ? (
          <fieldset className="flex min-w-0 flex-col gap-1 text-xs text-ink-muted">
            <legend className="mb-1 font-medium text-ink">
              Depends on workstreams
            </legend>
            <div className="flex max-h-40 flex-col gap-0.5 overflow-y-auto rounded-control border border-rule bg-card px-2.5 py-1.5">
              {project.workstreams.map((item) => (
                <label
                  key={item.slug}
                  className="flex cursor-pointer items-start gap-2 py-0.5 text-sm text-ink"
                >
                  <input
                    type="checkbox"
                    checked={dependsOn.includes(item.slug)}
                    onChange={(event) =>
                      toggle(item.slug, event.target.checked)
                    }
                    className="mt-1 size-3.5 shrink-0 accent-[var(--fh-select)]"
                  />
                  <span className="min-w-0 break-words">{item.title}</span>
                </label>
              ))}
            </div>
            <span>
              Its tickets wait until every ticket in these is in review or done.
            </span>
          </fieldset>
        ) : null}
        {error ? (
          <p role="alert" className="text-sm text-danger">
            {error}
          </p>
        ) : null}
      </form>
    </Dialog>
  );
}
