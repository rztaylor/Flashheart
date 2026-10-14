import type { ProjectSummary } from "../api/board";
import { FormField, Select } from "./Field";

// ProjectField chooses the project a new ticket or workstream goes in
// (EDIT-5, EDIT-14). It shows only where the dialog was opened without one
// (the all scope) and there is more than one to choose from; the caller
// resets what belongs to the old project in onChange.
export function ProjectField({
  projects,
  scope,
  value,
  onChange,
}: {
  projects: ProjectSummary[];
  // scope is the project the dialog was opened in; empty in the all scope.
  scope: string;
  value: string;
  onChange(name: string): void;
}) {
  if (scope || projects.length < 2) return null;
  return (
    <FormField label="Project">
      <Select value={value} onChange={onChange}>
        {projects.map((item) => (
          <option key={item.name} value={item.name}>
            {item.displayName}
          </option>
        ))}
      </Select>
    </FormField>
  );
}
