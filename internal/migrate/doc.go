// Package migrate converts a board root from format v1 (kanban-tracker
// column folders) to format v2 (a folder per ticket, MIG-1, D15).
//
// It owns the only v1 reader: planning (keys, numbering in creation order,
// targets), the reference rewrites (id and status fields, dependency and
// workstream lists, links), applying a plan through store's confined
// primitives under store's root lock, and rendering the plan. Frontmatter
// and YAML edits go through mdfile, which keeps line endings and byte-order
// marks; link rewriting is its own. It never deletes: replaced v1 files
// move into the root's backup folder. Parsing v2 belongs to board and store;
// flags and output to cli.
package migrate
