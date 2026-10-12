import type { ProjectSummary } from "../api/board";
import { SearchField, SelectField } from "../components/Field";
import type { Scope } from "./route";

// NarrowControls take the rail's place below 48rem (ui-layout.md §1):
// search on the Board and Table, the Project picker and, on the Board, the
// one column a phone shows.
export function NarrowControls({
  search,
  project,
  projects,
  onScope,
  column,
}: {
  search?: { query: string; onChange(query: string): void };
  // project is the scope's project, "" in All projects.
  project: string;
  projects: ProjectSummary[];
  onScope(scope: Scope): void;
  column?: {
    value: string;
    columns: { id: string; title: string }[];
    onChange(column: string): void;
  };
}) {
  return (
    <div className="flex flex-wrap items-center gap-3 border-b border-rule px-4 py-2 md:hidden">
      {search ? (
        <div className="flex w-full">
          <SearchField
            tone="plain"
            label="Search tickets"
            placeholder="Search tickets"
            value={search.query}
            onChange={search.onChange}
          />
        </div>
      ) : null}
      <SelectField
        label="Project"
        value={project}
        onChange={(name) =>
          onScope(name ? { kind: "project", project: name } : { kind: "all" })
        }
      >
        <option value="">All projects</option>
        {projects.map((item) => (
          <option key={item.name} value={item.name}>
            {item.displayName}
          </option>
        ))}
      </SelectField>
      {column ? (
        <SelectField
          label="Column"
          value={column.value}
          onChange={column.onChange}
        >
          {column.columns.map((item) => (
            <option key={item.id} value={item.id}>
              {item.title}
            </option>
          ))}
        </SelectField>
      ) : null}
    </div>
  );
}
