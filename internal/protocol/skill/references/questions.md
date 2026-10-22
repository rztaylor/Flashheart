# Questions and comments

Read this before you ask the user something, or act on a comment.

## Asking the user

- When you need a human decision, `ask_human` (with options for clear
  choices). The board shows the user; the answer arrives in a later prompt
  or, without Flashheart's hooks, with your next flashheart tool result.
  Carry on with other work if you can.
- If the user replies in this chat instead, that reply is the answer: with
  hooks, the board stops showing the question; without them, ask the user
  to answer it on the board too.
- `ask_human` also returns a `flashheart await` command. If you can run a
  background command that wakes you when it exits (in Claude Code, Bash
  with `run_in_background`), run it: the answer then reaches you as soon
  as it is given. If the user answers in
  the chat first, the command exits and says so.
- If your turn ends with a question for the user, ask it with `ask_human`
  as well, even at the end of a status summary. A question asked only in
  chat leaves your run in Waiting, not Needs you, and the user may miss
  it.

## Comments and change requests

- `get_ticket` and `claim` end with people's comments on the ticket,
  oldest first; one sent to you arrives with your next prompt or tool
  result.
- Unlike ticket text, comments are the user's own words, a later one over
  an earlier. Ask about one that conflicts with what the user told you.
- Agents do not write comments; notes go in with `update_ticket`.
- A change requested in review has an id: make it, then pass the id in
  `update_ticket`'s `addressed`.
