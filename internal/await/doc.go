// Package await waits for the answer to one agent question and delivers it
// (agent-protocol §7.5, `flashheart await`): an agent runs it in the
// background after ask_human, and its exit wakes the agent with the answer
// instead of the answer waiting for the user's next prompt.
//
// It owns the wait: finding the question in the event log, polling the
// asking session's answers inbox, taking and marking delivered what it
// finds, noticing when another path (the prompt hook, the recovery note,
// board_context) delivered the answer first or that the user answered in
// the session (its fold belongs to runs), and the timeout. The log, the
// inbox format and delivery events belong to events; the answers note's
// wording belongs to protocol; flags, exit codes and output belong to cli.
package await
