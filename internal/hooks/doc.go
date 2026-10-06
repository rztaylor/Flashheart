// Package hooks handles one agent hook invocation without knowing any
// agent's payload schema (HOOK-1–HOOK-4): it reads the payload, has the
// agent's Adapter turn it into events, resolves the project, branch and
// worktree from the working directory, scrubs and bounds what will be
// stored, appends the events, renders the recovery note at session start,
// delivers waiting answers with a prompt (HOOK-5), blocks a stop once for a
// checkpoint when the project enforces handoffs (HOOK-6), and prints
// adapter-only replies such as run stamping without opening the board.
// Every failure is logged to hook-errors.log and swallowed.
//
// Payload schemas and output formats belong to the adapters
// (hooks/claude); event files to events and store; run derivation to runs;
// note wording to protocol. Hooks never use index, api, app or webui.
package hooks
