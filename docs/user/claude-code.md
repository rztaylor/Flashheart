# Claude Code

Flashheart works with Claude Code in two ways:

- **Hooks** record what every session does, with no effort from the model:
  starts and ends, prompts (never their text), tools, edited files, its task
  list as a plan, permission prompts and subagents. The board shows every
  session live in the **Agents** view, badges the ticket it works on, and
  lifts sessions that need you into **Needs you**.
- **The `flashheart` MCP server** lets the model do what only it can: find
  and claim tickets, record handoffs, ask you questions, write reviews and
  finish work. The **Flashheart skill** tells it when and how.

Works in the Claude Code CLI, the IDE extensions and the Claude desktop
app's Code tab, which runs the same Claude Code. Ordinary chats in the Claude
app do not run Claude Code and have no hooks. Checked against Claude Code
2.1.288 (2026-10-06).

## Set it up

```sh
flashheart setup claude            # show what would change
flashheart setup claude --write    # apply it, with backups
```

`setup claude` changes nothing until you add `--write`. It shows, as a diff:

- the hooks it merges into `~/.claude/settings.json`: every event Flashheart
  handles, each running `<path to flashheart> hook claude <Event>`, and
  `PreToolUse` limited to Flashheart's own tools. Your other hooks and
  settings are kept exactly as they are;
- the MCP server it registers for every project, by running
  `claude mcp add-json --scope user flashheart …` (when no `claude` command
  is found it prints the command for you to run);
- the Flashheart skill it installs as `~/.claude/skills/flashheart/SKILL.md`;
- moving the `kanban-tracker` skill, if you have it, into the backup: the
  Flashheart skill replaces it, and agents should not have two sets of
  ticket rules.

`--write` first copies everything it changes into
`~/.claude/flashheart-backup/<time>/`. Commands use the absolute path of the
`flashheart` you ran, and pass `--root` only when your board is not at
`~/reports/Kanban`. Run setup again after moving the binary; it replaces its
own entries and leaves the rest alone.

Start a new Claude Code session afterwards: hooks, MCP servers and skills
are read when a session starts.

To undo it:

```sh
flashheart setup claude --uninstall           # show what would change
flashheart setup claude --uninstall --write   # undo it
```

If `settings.json` has not changed since setup wrote it, uninstalling puts
back the file exactly as it was; otherwise it removes only Flashheart's
hooks. It also removes the skill, puts `kanban-tracker` back and unregisters
the MCP server.

## Working with tickets

Ask in plain words: "tackle the two top-priority bugs", "look at FH-42",
"pick up where the last session left off". With the skill, the model:

- reads the **recovery note** Flashheart adds when a session starts (its
  run, project, the ticket this worktree is on and its last handoff), or
  calls `board_context`;
- in a new project, chooses the project's ticket key (two to five letters,
  like `FH`) before creating the first ticket;
- **claims** a ticket before working on it: the ticket moves to In
  progress, records the branch, and is held for that session while it is
  active (30 minutes after its last activity, `lease_minutes`);
- **checkpoints** as it goes: done, next, files and open questions replace
  the ticket's `## Handoff`, which the next session resumes from;
- groups work that spans several dependent tickets into a **workstream**,
  joining one the board already has or creating one in the order the
  tickets should be done; single tickets stay out of workstreams;
- finishes by writing a review, linking screenshots by path (Flashheart
  copies them into the ticket, so they survive the agent cleaning up), and
  moving the ticket to **Ready to review**. Agents never move tickets to
  Done.

## Questions

When the model needs a decision it calls `ask_human`. The session shows as
**Needs you** with its question, on the ticket's card and in the Agents
view. Answer in the card panel or in the run's detail: pick one of the
model's choices or write your own, then **Send answer**. The answer is added
to the ticket's notes and wakes the session: `ask_human` gives the model a
`flashheart await` command, which it runs in the background, and when you
send your answer that command prints it and exits, so Claude Code resumes
the session with it. If the model did not run it, the answer reaches the
session with its next prompt (send any prompt, such as "go on"); a session
that is resumed gets it in its recovery note.

A question the model asks only in its chat reply, without `ask_human`,
leaves the session in **Waiting**: Flashheart never reads what the model
writes, so it cannot tell a question from a summary. The protocol skill
tells the model to ask with `ask_human` whenever its turn ends on a
question.

## Handoff enforcement (optional)

To make sure a session never stops after editing files without updating its
ticket's handoff, turn on enforcement for a project in its `project.yaml`:

```yaml
settings:
  enforce_handoff: true
```

or for every project with `enforce_handoff: true` in
`<root>/.flashheart/config.yaml`. When a session linked to a ticket tries to
stop with edits since its last checkpoint, Flashheart asks it once to
checkpoint first. It never asks twice in a row.

## What gets recorded

Into `<root>/<project>/.flashheart/events/<date>.jsonl`, where the project is
named after the repository's main checkout (worktrees share it) and is
created on first contact with a `project.yaml` but no ticket key yet:

- session and subagent starts and ends, prompts submitted (never their
  text), turns finished and compaction;
- tool names, success or failure, and for edits the file path relative to
  the repository;
- the session's own task list (`TodoWrite` and the task tools), as its plan;
- permission requests and notifications, by type only;
- the model's claims, checkpoints, ticket changes and questions, and your
  answers.

Prompts, commands, tool inputs and outputs, and message text are never
stored. Text the model writes is scrubbed of anything that looks like a
secret.

## Linking sessions to tickets

A session is linked to the ticket it claimed. Without a claim, it is linked
when exactly one **In progress** ticket in its project has `branch:` set to
the session's branch. Subagents share their session's ticket. Linked
sessions show on the ticket's card and in its **Runs** tab.

## When something goes wrong

The hooks never interrupt Claude Code: they always exit 0 and print nothing
except the recovery note, answers and Flashheart's own tool stamping.
Problems are logged to `<root>/.flashheart/hook-errors.log`. MCP tool errors
come back to the model with a fix, such as passing its `run` when two
sessions work in the same worktree.
