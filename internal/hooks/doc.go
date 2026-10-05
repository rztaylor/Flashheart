// Package hooks handles one agent hook invocation without knowing any
// agent's payload schema (HOOK-1–HOOK-4): it reads the payload, has the
// agent's Adapter turn it into events, resolves the project, branch and
// worktree from the working directory, scrubs and bounds what will be
// stored, appends the events, and renders the recovery note at session
// start. Every failure is logged to hook-errors.log and swallowed.
//
// Payload schemas and output formats belong to the adapters
// (hooks/claude); event files to events and store; run derivation to runs;
// note wording to protocol. Hooks never use index, api, app or webui.
package hooks
