// Package hooks handles one agent hook invocation without knowing any
// agent's payload schema (HOOK-1–HOOK-4): it reads the payload, has the
// agent's Adapter turn it into events, resolves the project, branch and
// worktree from the working directory, scrubs and bounds what will be
// stored, appends the events, renders the recovery note at session start
// (after calling the caller's skill refresh and noting an update),
// delivers waiting answers with a prompt (HOOK-5), marks a session's turn
// end or end when its worktree changed after its last checkpoint through
// shell commands (asking gitchange, under a deadline, only when it ran one;
// agent-protocol §4), blocks a stop once for a checkpoint when the project
// enforces handoffs (HOOK-6), and prints
// adapter-only replies such as run stamping without opening the board.
// Every failure is logged to hook-errors.log and swallowed. In a repository
// whose project is archived (PRJ-5) a hook records nothing and stays quiet.
//
// Payload schemas and output formats belong to the adapters
// (hooks/claude); event files to events and store; run derivation to runs;
// git commands to gitchange; note wording to protocol. Hooks never use index, api, app or webui.
package hooks
