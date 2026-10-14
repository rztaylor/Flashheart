// Package config defines the board root's global settings,
// <root>/.flashheart/config.yaml: parsing them over documented defaults
// and validating them.
//
// It owns setting names, defaults, allowed values and the board format version
// check (version 2 only; format 1 is no longer read, D55), and
// rewriting the ui preferences in config.yaml's content while keeping
// everything else (CFG-2), and a renamed workstream's slug in them
// (UI.RenameWorkstream, FH-170). It does no file I/O: store reads and writes the
// file through the root (store.ReadConfig, store.UpdateConfig, SEC-2) and
// owns its location; per-project overrides in project.yaml are read by
// store into board.
package config
