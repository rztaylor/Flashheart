# Changelog

All notable changes to this project are documented here. The project follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

## [0.5.0] - 2026-10-24

The first published release. Flashheart is a local kanban board for
projects built with AI coding agents: one `flashheart` binary that serves
the board in your browser, and the MCP server and hook handler that
Claude Code and Codex sessions use to record what they are doing, so you
can see every project at once and a lost session can pick up where it
left off. Everything runs on your computer; nothing is sent anywhere.

Prebuilt archives for macOS (Apple silicon and Intel) and Linux (x86-64
and ARM) are attached, with a `SHA256SUMS` file. Install and setup are in
the
[README](https://github.com/rztaylor/Flashheart#set-up-in-four-steps).

### Added

- **A board of markdown files.** Each project is a folder in your board
  root (`~/reports/Kanban` by default), and each ticket a folder holding
  its markdown file, its review and its attachments, with its column in
  the frontmatter (`docs/dev/specs/board-format.md`). Obsidian or any
  editor can edit them; the board picks up a change within a second.
  Writes are atomic, locked per project and refused over a newer version,
  with a side-by-side conflict dialog when a save loses a race.
- **The board in your browser.** `flashheart` opens a private,
  authenticated board on a loopback address and returns the terminal.
  Views: an Overview of what needs your decision, what is ready for
  review, what is at risk and what is in progress, for one project or all
  of them; a Board in three densities with filters, a Needs you filter,
  drag-and-drop and keyboard moves with Undo; Workstreams drawn as railway
  lines of dependent tickets; and a sortable Table. Light and dark themes
  meet WCAG 2.2 AA; the board can be driven by keyboard and has a phone
  layout.
- **Tickets you can work on.** Every ticket has a full page that edits in
  place, with a formatted markdown editor, pasted screenshots and dropped
  files, acceptance criteria to tick, the agent's handoff and review with
  its evidence, comments, change requests in review, and a lasting
  history of who changed what. A comment can be sent straight to the
  session working on the ticket. Projects and tickets can be archived,
  restored or, after a check of what refers to them, deleted; a project
  can show its own image instead of its key.
- **Claude Code and Codex on the board.** `flashheart setup claude` and
  `flashheart setup codex` show the hooks, MCP server and instructions
  they would add as a diff and apply them only when you say yes;
  `--uninstall` removes only Flashheart's entries. Hooks then record each
  session, its subagents, turns, edited files, permission prompts and
  questions, and tell a new session about its ticket's handoff. Hooks
  never store prompts, commands or tool inputs and outputs, scrub likely
  secrets, finish quickly and fail open: Flashheart failing never breaks
  an agent session. A Codex session without hooks still gets a run of its
  own through the MCP server.
- **MCP tools for agents** (agent protocol 2): find work
  (`board_context`, `list_tickets`, `get_ticket`), hold it (`claim`,
  `release`), record progress (`checkpoint`, `update_ticket`, `move`,
  `write_review`, `attach`), file and group work (`create_ticket`,
  `create_workstream`, `update_workstream`, `set_project_key`) and ask
  you (`ask_human`). Agents never move a ticket to Done. The skill
  installed by setup tells agents when to use each, and asks for tickets
  and handoffs a person can read; it updates itself after an upgrade.
- **Questions and answers.** An agent's question shows as Needs you on
  its ticket and in the Overview; answer it there and the answer reaches
  the session with its next prompt, or at once through `flashheart await`
  when the session waits for it. A question answered in the session's own
  chat leaves Needs you, and the ticket's history keeps the question and
  what became of it.
- **Handoffs.** A checkpoint rewrites the ticket's handoff (what is done,
  what is next, in plain words). A project can opt in to handoff
  enforcement (`enforce_handoff`): a session that tries to stop with
  edits since its last checkpoint is asked, once, to checkpoint first.
  A session that edits files without a ticket is asked once to claim or
  create one.
- **`flashheart doctor`** checks the board root, both agents' hooks, MCP
  registration, skill or instructions, Codex hook trust and recent hook
  errors, with the fix for each, and changes nothing.
- A copy-paste prompt that moves a project's existing to-do lists,
  roadmaps and plans onto the board, with your approval
  (`docs/user/migrating.md`).
- Release archives built and published by GitHub Actions from this
  changelog; `flashheart version` prints the version, commit, build time
  and agent protocol version.

### Compatibility

- Boards stay readable across releases (D88): every later release reads
  the boards this one writes, without a manual step. Commands, flags,
  the HTTP API and the UI may still change before 1.0.
- The agent protocol is version 2. A later release that changes it in a
  way that is not additive raises the version and its notes say to run
  `setup` again.
- Hook adapters are built against payloads recorded from Claude Code
  2.1.288 and 2.1.291 and from Codex 0.160.0 (CLI and the ChatGPT desktop
  app). Flashheart is developed on its own board with Claude Code up to
  2.1.300; later agent releases are expected to work but are not
  recorded.

### Known limits

- macOS binaries are not signed or notarised. Installing with the
  README's `curl` command avoids Gatekeeper's prompt; an archive
  downloaded with a browser needs its quarantine attribute cleared.
- No Windows build and no Homebrew tap yet.
- Codex skips a hook until you trust it with `/hooks`; `doctor` names the
  hooks that need trusting after setup changes them.
- Question outcomes are recorded while the board is running: one that
  happens while it is closed for more than two days is not recovered.
