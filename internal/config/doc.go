// Package config reads the board root's global settings,
// <root>/.flashheart/config.yaml, applying documented defaults and validation.
//
// It owns setting names, defaults, allowed values and the board format version
// check. It does not write yet and never creates the root; per-project
// project.yaml settings, preference writes and every other board file belong
// to later owners (store).
package config
