// Package config reads the board root's global settings,
// <root>/.flashheart/config.yaml, applying documented defaults and validation.
//
// It owns setting names, defaults, allowed values and the board format version
// check (version 2 current, version 1 accepted so it can be migrated), and
// rewriting the ui preferences in config.yaml while keeping everything else
// (CFG-2). It never creates the root; writing the file belongs to store, and
// per-project project.yaml settings are not read yet.
package config
