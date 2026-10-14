package config

import (
	"maps"
	"slices"
)

// RenameWorkstream returns the preferences with workstream from of project
// named to (FH-170): the project's remembered workstream filter, included
// and excluded, and its expanded finished workstreams. All projects' and
// other projects' choices stay, as a bare slug there may name another
// project's workstream. A slug already listed is not doubled. changed
// reports whether anything was rewritten; u itself is left as it was.
func (u UI) RenameWorkstream(project, from, to string) (next UI, changed bool) {
	next = u
	if scope, ok := u.Scopes[project]; ok {
		include, in := renameIn(scope.Workstream.Include, from, to)
		exclude, ex := renameIn(scope.Workstream.Exclude, from, to)
		if in || ex {
			scope.Workstream = Choice{Include: include, Exclude: exclude}
			next.Scopes = maps.Clone(u.Scopes)
			next.Scopes[project] = scope
			changed = true
		}
	}
	if expanded, ok := renameIn(u.ExpandedWorkstreams[project], from, to); ok {
		next.ExpandedWorkstreams = maps.Clone(u.ExpandedWorkstreams)
		next.ExpandedWorkstreams[project] = expanded
		changed = true
	}
	return next, changed
}

// renameIn is values with from replaced by to, once, in a new list; ok
// reports whether from was there.
func renameIn(values []string, from, to string) ([]string, bool) {
	if !slices.Contains(values, from) {
		return values, false
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == from {
			value = to
		}
		if !slices.Contains(out, value) {
			out = append(out, value)
		}
	}
	return out, true
}
