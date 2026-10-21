// Generates the README's showcase board (FH-180): an invented but realistic
// root about building Flashheart itself, with every column filled, six
// workstreams whose tickets depend on each other, and two smaller projects
// so the project rail looks lived in. All content is invented; nothing is
// read from a real board. Dates written into the files are fixed; file
// times are set relative to `now`, so "changed 2h ago" reads the same on
// every run.
import { mkdir, readdir, utimes, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";

const HOUR = 3_600_000;

// The person who answers agents' questions and comments on tickets.
const PERSON = "Sam";

// Run ids of the sessions the screenshot spec starts through the real hooks
// and MCP server; the board's notes and handoffs name the same runs.
export const RUNS = {
  lanes: "4c1e8f2a-6b3d-4a5e-9c7f-1d2e3f4a5b6c",
  paste: "7a9d2c4e-1f3b-4e6a-8d5c-2b1a0f9e8d7c",
  railway: "019a8f10-2c4e-7a61-9b3d-5e7f1a2c4d6e",
  tides: "2b4d6f8a-0c1e-4a3b-9d5f-7e9a1c3b5d7f",
  review: "9e8d7c6b-5a4f-4e3d-8c2b-1a0f9e8d7c6b",
};

const short = (id) => id.slice(0, 8);

// Each row: [slug, title, type, priority, status, workstream, depends-on
// (slugs), hours since its last change, description, criteria, extra].
// Ids follow row order. Criteria are ticked by status unless extra.ticked
// says how many are ticked.
const flashheart = [
  // Foundation: finished, so its line folds away on the Workstreams view.
  [
    "plain-markdown-tickets",
    "Tickets are plain markdown files",
    "feature",
    "high",
    "done",
    "board-files",
    [],
    640,
    "Every ticket is a markdown file with a short header, in a folder per project, so it reads well in any editor and diffs cleanly in git.",
    [
      "A ticket opens and reads well in a plain text editor",
      "Unknown header fields and sections survive a save",
    ],
  ],
  [
    "stable-ids",
    "Ticket ids that never change or get reused",
    "feature",
    "high",
    "done",
    "board-files",
    ["plain-markdown-tickets"],
    610,
    "Ids like FH-42 are handed out once per project and never renumbered, so links in commits, PRs and other tickets keep working.",
    [
      "Numbers come from the project file under its lock",
      "A deleted ticket's number is never given out again",
    ],
  ],
  [
    "safe-writes",
    "Saves never overwrite someone else's edit",
    "feature",
    "high",
    "done",
    "board-files",
    ["plain-markdown-tickets"],
    600,
    "Every write checks the file is still the one it read, under a project lock, and replaces it atomically. A save that lost the race says so instead of winning.",
    [
      "Two writers at once both land or one is told why",
      "A crash mid-write never leaves half a ticket",
    ],
  ],
  [
    "blocked-reasons",
    "Blocked tickets say what they wait for",
    "feature",
    "medium",
    "done",
    "board-files",
    ["stable-ids"],
    560,
    "A ticket held back by another ticket or a workstream says which one, in words, on its card and its page.",
    [
      "Card and page name every blocker",
      "Review counts as served for blocking",
    ],
  ],
  [
    "hand-edit-repair",
    "Broken hand edits show as needs repair",
    "bug",
    "medium",
    "done",
    "board-files",
    ["safe-writes"],
    530,
    "A header with a typo made the whole project disappear from the board. Now the one ticket shows as needs repair and says what is wrong.",
    [
      "The rest of the project still loads",
      "The card names the line that does not parse",
    ],
  ],
  // Agent sessions: the richest line, with work in every state.
  [
    "claude-hooks",
    "See Claude Code sessions as they work",
    "feature",
    "high",
    "done",
    "agent-sessions",
    [],
    430,
    "Claude Code's hooks record when a session starts, uses tools, waits for permission and stops, so the board can show what each session is doing.",
    [
      "Hooks return in under 50 ms",
      "A hook that fails never breaks the session",
      "Only tool names and file paths are kept, never inputs",
    ],
  ],
  [
    "codex-hooks",
    "See Codex sessions as they work",
    "feature",
    "high",
    "done",
    "agent-sessions",
    [],
    410,
    "Codex gets the same hooks as Claude Code, so both agents show up on one board side by side.",
    [
      "Setup registers the hooks Codex supports",
      "Codex sessions show their branch and subagents",
    ],
  ],
  [
    "session-ticket-link",
    "Show which ticket each session is working on",
    "feature",
    "high",
    "done",
    "agent-sessions",
    ["claude-hooks", "codex-hooks"],
    330,
    "A session that claims a ticket, or works on its branch, appears on that ticket's card with its state: working, waiting for you, or quiet.",
    [
      "A claimed ticket shows its session on the card",
      "A session on a ticket's branch links to it until something claims it",
    ],
  ],
  [
    "recovery-note",
    "Resume a lost session from its ticket",
    "feature",
    "high",
    "done",
    "agent-sessions",
    ["session-ticket-link"],
    260,
    "When a session starts in a project, it is told its ticket's handoff, unticked criteria and waiting answers, so a lost session costs minutes instead of an afternoon.",
    [
      "The note fits in 1,500 tokens",
      "It names the handoff's next steps first",
    ],
  ],
  [
    "agent-questions",
    "Answer agents' questions from the board",
    "feature",
    "high",
    "review",
    "agent-sessions",
    ["session-ticket-link"],
    20,
    "An agent that needs a decision asks on the board instead of stalling in a terminal you are not looking at. The question shows on its card and the Overview, and your answer reaches the session with its next prompt.",
    [
      "A question puts its ticket in Needs you",
      "Choices answer in one click; free text is allowed",
      "The answer is delivered once, with the next prompt",
      "The question and answer are kept in the ticket's notes",
    ],
    { review: true },
  ],
  [
    "overview-lanes",
    "See live sessions on the Overview",
    "feature",
    "medium",
    "in-progress",
    "agent-sessions",
    ["agent-questions"],
    1,
    "The Overview shows each running session under its ticket with its subagents, so you can see at a glance what is moving and what has gone quiet.",
    [
      "In progress lists each session's state in words",
      "Subagents are counted as done and running",
      "A quiet session moves to At risk",
    ],
    { ticked: 1, run: RUNS.lanes },
  ],
  [
    "cross-agent-handoff",
    "Hand work between Claude Code and Codex",
    "feature",
    "medium",
    "up-next",
    "agent-sessions",
    ["recovery-note", "agent-questions"],
    70,
    "Start a ticket in one agent and finish it in the other: the second session gets the first one's handoff and answers, whichever agent wrote them.",
    [
      "A Codex session resumes a Claude Code handoff",
      "Questions asked in one are answered in the other",
    ],
  ],
  [
    "tell-session",
    "Tell a running session something",
    "feature",
    "medium",
    "backlog",
    "agent-sessions",
    ["overview-lanes"],
    90,
    "Send a comment to the session working on a ticket; it arrives with that session's next prompt, and the board shows when it was read.",
    [
      "Only live sessions are offered",
      "The comment says when it was delivered",
    ],
  ],
  [
    "slow-hooks",
    "Hooks slowed every tool call on large boards",
    "bug",
    "high",
    "done",
    "agent-sessions",
    ["claude-hooks"],
    300,
    "With a few thousand tickets each hook read the whole board, adding up to 200 ms to every tool call. Hooks now read only the session they record.",
    [
      "p95 under 50 ms with 50,000 events",
      "Benchmark added to the developer tools",
    ],
  ],
  // Board editing.
  [
    "drag-cards",
    "Drag cards between columns and within one",
    "feature",
    "high",
    "done",
    "editing",
    [],
    500,
    "Move a card by dragging it, or with Shift and the arrow keys. Its place in the column is kept, without renumbering the cards around it.",
    ["Drag and keyboard moves both work", "Order survives a reload"],
  ],
  [
    "edit-in-place",
    "Edit a ticket's sections in place",
    "feature",
    "high",
    "done",
    "editing",
    [],
    470,
    "Each section of a ticket has a pencil: edit just that section and save it, leaving the rest of the file exactly as it was.",
    [
      "Only the edited section's lines change on disk",
      "An unchanged save writes nothing",
    ],
  ],
  [
    "save-conflicts",
    "Show both versions when a save conflicts",
    "feature",
    "medium",
    "done",
    "editing",
    ["edit-in-place"],
    380,
    "If an agent changed the section you were editing, you see both versions side by side and choose, instead of losing either.",
    [
      "The conflict names who changed it",
      "Keeping yours or theirs is one click",
    ],
  ],
  [
    "formatted-editor",
    "Formatted editor with a live preview",
    "feature",
    "high",
    "review",
    "editing",
    ["edit-in-place"],
    6,
    "Write tickets with headings, lists and checkboxes rendered as you type, and switch to plain markdown whenever you like. The file stays ordinary markdown.",
    [
      "Opening and saving without edits changes no byte",
      "A slash menu inserts lists, tables and code",
      "The view you last used is remembered",
    ],
  ],
  [
    "paste-screenshots",
    "Paste screenshots straight into a ticket",
    "feature",
    "medium",
    "in-progress",
    "editing",
    ["formatted-editor"],
    0.5,
    "Paste or drop an image into a ticket and it is copied into the ticket's files and linked where the cursor was.",
    [
      "A pasted image is stored in the ticket's files",
      "Dropped files are judged by their content, not their name",
      "Large files are refused with the limit in words",
    ],
    { ticked: 1, run: RUNS.paste },
  ],
  [
    "comments",
    "Comment on a ticket and ask for changes",
    "feature",
    "high",
    "up-next",
    "editing",
    ["save-conflicts"],
    45,
    "Leave a comment on a ticket, or quote a criterion and ask for a change. The agent working on it sees open requests and marks them addressed.",
    [
      "Comments are kept beside the ticket, not in it",
      "A change request moves the ticket back to In progress",
    ],
  ],
  [
    "undo-move",
    "Undo a move from its toast",
    "feature",
    "low",
    "backlog",
    "editing",
    ["drag-cards"],
    150,
    "A card dropped in the wrong column can be put back from the toast that confirms the move.",
    ["Undo puts the card back in its old place"],
  ],
  [
    "review-checklist",
    "Tick review steps off as you verify them",
    "feature",
    "medium",
    "backlog",
    "editing",
    ["formatted-editor", "comments"],
    120,
    "The How to verify steps of a review become checkboxes, so you can see how far through a review you got.",
    ["Ticking a step saves at once", "The card shows steps checked of total"],
  ],
  // Workstreams as railway lines.
  [
    "group-workstreams",
    "Group tickets into workstreams",
    "feature",
    "high",
    "done",
    "railway-lines",
    [],
    450,
    "A workstream collects the tickets that share a goal, like an epic, and can wait for other workstreams to finish first.",
    [
      "A ticket belongs to at most one workstream",
      "A workstream can depend on others, never in a cycle",
    ],
  ],
  [
    "draw-lines",
    "Draw each workstream as a railway line",
    "feature",
    "high",
    "done",
    "railway-lines",
    ["group-workstreams"],
    280,
    "Each workstream is a coloured line with its tickets as stations: served ones filled, the next stop ringed, and work held from outside dashed.",
    [
      "Every station state has its own mark",
      "Line colours stay put when workstreams are added",
    ],
  ],
  [
    "branch-and-join",
    "Tracks branch and join for parallel work",
    "feature",
    "high",
    "in-progress",
    "railway-lines",
    ["draw-lines"],
    1,
    "Tickets that can be worked on side by side run on parallel tracks, and a ticket that needs several of them is where their tracks join.",
    [
      "A chain keeps to one track",
      "Independent work runs on tracks below",
      "Tracks bend only in the gaps between stations",
    ],
    { ticked: 2 },
  ],
  [
    "workstream-page",
    "A page for each workstream",
    "feature",
    "medium",
    "review",
    "railway-lines",
    ["draw-lines"],
    26,
    "Open a workstream to see its whole line, and read or edit its goal, scope and success criteria in place.",
    [
      "Every section edits in place like a ticket's",
      "Stations open the ticket's own page",
    ],
  ],
  [
    "collapse-finished",
    "Finished lines fold away",
    "feature",
    "low",
    "up-next",
    "railway-lines",
    ["draw-lines"],
    52,
    "A workstream with every ticket done folds to its heading, so the lines still running stay on screen.",
    [
      "Folded lines open with one click",
      "The choice lasts while the line stays finished",
    ],
  ],
  [
    "rename-workstream",
    "Rename a workstream from its page",
    "feature",
    "medium",
    "backlog",
    "railway-lines",
    ["workstream-page"],
    100,
    "Change a workstream's name on its page; every ticket and workstream that refers to it follows.",
    ["Every reference is rewritten in the same save"],
  ],
  [
    "reorder-stations",
    "Reorder independent stations by dragging",
    "feature",
    "low",
    "backlog",
    "railway-lines",
    ["branch-and-join", "collapse-finished"],
    110,
    "Stations with no links on the line can be put in the order you plan to take them.",
    ["Drag and Shift with the arrow keys both reorder"],
  ],
  // Releases: waits for two other workstreams.
  [
    "release-archives",
    "Release archives for macOS and Linux",
    "infra",
    "high",
    "up-next",
    "downloads",
    [],
    30,
    "Every tagged version builds a single binary for each platform, with checksums, from CI.",
    [
      "Archives for macOS and Linux on arm64 and amd64",
      "Checksums published beside them",
    ],
  ],
  [
    "notarise-macos",
    "Signed and notarised macOS builds",
    "infra",
    "high",
    "backlog",
    "downloads",
    ["release-archives"],
    80,
    "The macOS binary is signed and notarised, so it opens without a security warning.",
    ["Gatekeeper accepts the downloaded binary"],
  ],
  [
    "homebrew",
    "Install with Homebrew",
    "infra",
    "medium",
    "backlog",
    "downloads",
    ["notarise-macos"],
    80,
    "brew install puts the latest release on your path, and brew upgrade keeps it current.",
    ["The formula updates on every release"],
  ],
  [
    "install-script",
    "Install script that checks the download",
    "infra",
    "medium",
    "backlog",
    "downloads",
    ["release-archives"],
    80,
    "A short script downloads the right archive for your machine and checks its checksum before installing.",
    ["A tampered download is refused"],
  ],
  [
    "first-run-guide",
    "First-run guide from install to board",
    "docs",
    "medium",
    "backlog",
    "downloads",
    ["homebrew", "install-script"],
    75,
    "Install, connect your agents and open your first board, in one page with nothing to skip.",
    ["Every command in the guide is tested on a clean machine"],
  ],
  // Search: waits for the railway lines.
  [
    "search-text",
    "Search tickets by title, text and tags",
    "feature",
    "high",
    "backlog",
    "search",
    [],
    60,
    "Type in the band to find tickets in every project by their words, with the matching line shown.",
    ["Results appear as you type", "Archived tickets are found too"],
  ],
  [
    "jump-to-id",
    "Jump straight to a ticket by its id",
    "feature",
    "medium",
    "up-next",
    "search",
    [],
    40,
    "Typing an id like FH-42 in search opens that ticket at once.",
    ["An exact id opens the ticket's page"],
  ],
  [
    "saved-filters",
    "Filters remembered per project",
    "feature",
    "low",
    "backlog",
    "search",
    ["search-text"],
    60,
    "Each project remembers its view and filters, so coming back to it shows what you were looking at.",
    ["Filters survive a restart"],
  ],
  // Tickets outside any workstream.
  [
    "long-titles",
    "Long card titles spill out of narrow columns",
    "bug",
    "medium",
    "in-progress",
    "",
    [],
    5,
    "A title with a long unbroken word, like a file path, runs past the card's edge when the window is narrow.",
    ["Long words wrap inside the card", "Titles stay at most three lines"],
    { ticked: 0 },
  ],
  [
    "focus-ring",
    "Focus ring is hard to see on the dark band",
    "bug",
    "high",
    "done",
    "",
    [],
    200,
    "Keyboard users lost track of focus in the dark theme's top band, where the ring had too little contrast.",
    ["The ring passes 3:1 contrast in both themes"],
  ],
  [
    "hook-bench",
    "Measure hook speed on every pull request",
    "test",
    "medium",
    "review",
    "",
    [],
    28,
    "CI times a thousand warm hook calls and fails the build if the slowest five percent take longer than 50 ms.",
    [
      "The benchmark runs in CI on Linux",
      "A slow hook fails the check with its timing",
    ],
  ],
  [
    "split-api",
    "Split the API handlers by resource",
    "refactor",
    "low",
    "backlog",
    "",
    [],
    160,
    "One long file holds every handler. Splitting it by resource makes each change smaller to review.",
    ["No behaviour changes; the API contract tests stay green"],
  ],
  [
    "windows",
    "Could Flashheart run on Windows?",
    "spike",
    "low",
    "backlog",
    "",
    [],
    170,
    "Find out what file locking, hooks and the browser launcher would need on Windows, and what it would cost to keep working.",
    ["A short write-up with a recommendation"],
    { tags: ["later-possibility"] },
  ],
  [
    "handoff-reminder",
    "Remind sessions to leave a handoff",
    "feature",
    "low",
    "backlog",
    "agent-sessions",
    ["cross-agent-handoff"],
    85,
    "A session about to stop after edits, without writing a handoff, is asked to write one first. It is opt-in per project.",
    [
      "Off unless the project turns it on",
      "Never blocks a session more than once per stop",
    ],
  ],
  [
    "drop-files",
    "Drop files on a ticket to attach them",
    "feature",
    "medium",
    "backlog",
    "editing",
    ["paste-screenshots"],
    95,
    "Drop logs, PDFs or images anywhere on a ticket's page and they are added to its files with their names as captions.",
    [
      "Several files can be dropped at once",
      "Files the board cannot show are offered as downloads",
    ],
  ],
];

const tideline = [
  [
    "harbour-tides",
    "Tide times for any harbour from the national tables",
    "feature",
    "high",
    "done",
    "tide-data",
    [],
    300,
    "Pick a harbour and see today's high and low waters, from the published tables.",
    ["Every standard port is listed"],
  ],
  [
    "offline-week",
    "Tide tables work offline for a week ahead",
    "feature",
    "high",
    "in-progress",
    "tide-data",
    ["harbour-tides"],
    2,
    "Out at sea there is no signal: keep the next seven days for your saved harbours on the phone.",
    [
      "Saved harbours download overnight on Wi-Fi",
      "The app says how old its tables are",
    ],
    { ticked: 1, run: RUNS.tides },
  ],
  [
    "clock-change",
    "Times shift by an hour after the clocks change",
    "bug",
    "high",
    "review",
    "tide-data",
    ["harbour-tides"],
    20,
    "Tides for the week after the clocks change were shown an hour late.",
    ["Times are right on both sides of the change"],
  ],
  [
    "low-water-alert",
    "Warn me before low water at my mooring",
    "feature",
    "high",
    "up-next",
    "alerts",
    ["offline-week"],
    30,
    "A notification an hour before the water is too shallow to leave the mooring.",
    ["The warning time is set per mooring"],
  ],
  [
    "home-widget",
    "Today's tides on the home screen",
    "feature",
    "medium",
    "backlog",
    "alerts",
    ["harbour-tides"],
    90,
    "A small widget with the next high and low water for your home harbour.",
    ["The widget updates without opening the app"],
  ],
  [
    "metres-feet",
    "Heights in metres or feet",
    "feature",
    "low",
    "done",
    "",
    [],
    260,
    "Choose metres or feet once and every height follows.",
    ["The choice applies everywhere"],
  ],
  [
    "store-listing",
    "App store listing and screenshots",
    "docs",
    "low",
    "backlog",
    "",
    [],
    120,
    "Write the listing and take screenshots for both stores.",
    ["Screenshots for every required size"],
  ],
];

const dotfiles = [
  [
    "new-mac",
    "One command to set up a new Mac",
    "feature",
    "medium",
    "up-next",
    "",
    [],
    48,
    "Clone, run one script, and the shell, editor and tools are as they were on the old machine.",
    ["The script can run twice without harm"],
  ],
  [
    "fast-shell",
    "Shell starts in under 100 ms",
    "refactor",
    "low",
    "backlog",
    "",
    [],
    140,
    "Load the slow plugins lazily so a new terminal is ready at once.",
    ["Startup time measured before and after"],
  ],
  [
    "editor-backup",
    "Back up editor settings and snippets",
    "infra",
    "low",
    "done",
    "",
    [],
    330,
    "Keep the editor's settings and snippets in the repository.",
    ["Restoring them is one link per file"],
  ],
];

const workstreams = {
  flashheart: [
    {
      slug: "board-files",
      title: "Board files",
      created: "2026-09-01",
      priority: "high",
      goal: "A board you can trust: plain files, ids that last and saves that never lose work.",
    },
    {
      slug: "agent-sessions",
      title: "Agent sessions",
      created: "2026-09-12",
      priority: "high",
      deps: ["board-files"],
      goal: "See every Claude Code and Codex session working on your projects, what it is doing, and when it needs you, without opening a terminal.",
      scope: [
        "Hooks for Claude Code and Codex that record sessions, tools and questions, quickly and quietly",
        "Linking sessions to the tickets they claim or work on",
        "Questions and answers on the board, delivered to the session",
        "Resuming a ticket in a new session, in either agent",
      ],
      success: [
        "A lost session can be resumed from its ticket in under a minute",
        "Every session that needs you shows on the Overview",
        "Hooks add less than 50 ms to a tool call",
      ],
      notes:
        "Hooks must never block or break a session: when Flashheart fails, the agent carries on and the failure goes to the hook error log.",
    },
    {
      slug: "editing",
      title: "Editing",
      created: "2026-09-15",
      priority: "high",
      deps: ["board-files"],
      goal: "Do everyday ticket work in the browser without breaking the files agents read and write.",
      scope: [
        "Moving cards, and editing a ticket's fields and sections in place",
        "A formatted editor that keeps the file plain markdown",
        "Screenshots and other files pasted or dropped into a ticket",
        "Comments and change requests the working agent can see",
      ],
      success: [
        "An edit changes only the lines it touched",
        "No save ever silently overwrites an agent's checkpoint",
        "Everything works by keyboard as well as by mouse",
      ],
      notes:
        "The files stay the source of truth: the board never keeps ticket state of its own, so an agent and a person can edit the same ticket safely.",
    },
    {
      slug: "railway-lines",
      title: "Railway lines",
      created: "2026-09-20",
      priority: "medium",
      deps: ["board-files"],
      goal: "Show each workstream as a line on a railway map, so what is done, what is next and what is held back is clear at a glance.",
    },
    {
      slug: "downloads",
      title: "Downloads",
      created: "2026-10-02",
      priority: "high",
      deps: ["agent-sessions", "editing"],
      goal: "Install Flashheart in one command on macOS and Linux, from signed builds with published checksums.",
    },
    {
      slug: "search",
      title: "Search",
      created: "2026-10-06",
      priority: "medium",
      deps: ["railway-lines"],
      goal: "Find any ticket in any project by its words or its id.",
    },
  ],
  tideline: [
    {
      slug: "tide-data",
      title: "Tide data",
      created: "2026-09-08",
      priority: "high",
      goal: "Reliable tide times for every harbour, online or off.",
    },
    {
      slug: "alerts",
      title: "Alerts",
      created: "2026-09-22",
      priority: "medium",
      deps: ["tide-data"],
      goal: "Tell sailors in good time when the water is too low to leave.",
    },
  ],
};

const projects = [
  {
    dir: "flashheart",
    key: "FH",
    name: "Flashheart",
    rows: flashheart,
    start: "2026-09-01",
  },
  {
    dir: "tideline",
    key: "TL",
    name: "Tideline",
    rows: tideline,
    start: "2026-09-08",
  },
  {
    dir: "dotfiles",
    key: "DOT",
    name: "Dotfiles",
    rows: dotfiles,
    start: "2026-09-18",
  },
];

// The richer text of the tickets the README's shots look at closely.
const details = {
  "FH-10": {
    plan: "Ask a decision through the real MCP server, answer it on the card and on the Overview, and check the next prompt hook hands the answer over exactly once.",
    context:
      "- The question is an event in the project's log (`question.asked`); the answer is written to the session's inbox and drained by the next prompt hook.\n- Card and Overview share the question card component.",
    notes: [
      `- 2026-10-19 · claude:${short(RUNS.review)}: Questions without choices take free text only; choices are optional.`,
      `- 2026-10-20 · Question from claude:${short(RUNS.review)} — "Should an unanswered question move the ticket back to In progress after a day?" Answer (${PERSON}): "No, leave it in Needs you and let the Overview show how long it has waited."`,
      `- 2026-10-21 · claude:${short(RUNS.review)}: Screenshots of the card and the Overview attached to the review.`,
    ],
    handoff: {
      at: "2026-10-21 09:42",
      run: RUNS.review,
      branch: "feature/agent-questions",
      done: [
        "Questions and answers through the MCP server and the prompt hook",
        "Question card on the ticket panel and the Overview",
        "Review written with screenshots",
      ],
      next: ["Human verifies the review and merges"],
      files: [
        "internal/mcpserver/ask.go",
        "internal/hooks/inbox.go",
        "frontend/src/features/card/QuestionCard.tsx",
      ],
    },
  },
  "FH-11": {
    plan: "Seed two sessions through the real hooks, one with three subagents, and check the Overview's In progress and At risk rows in both themes.",
    handoff: {
      at: "2026-10-21 10:05",
      run: RUNS.lanes,
      branch: "feature/overview-lanes",
      done: ["Session rows grouped under their tickets"],
      next: [
        "Count subagents as done and running",
        "Move quiet sessions to At risk",
        "Screenshots at 1440 and 390 px",
      ],
      files: [
        "frontend/src/model/overview.ts",
        "frontend/src/features/overview/InProgress.tsx",
      ],
    },
  },
  "FH-19": {
    plan: "Paste a canvas PNG, drop several files and an oversized one, and check what lands in the ticket's files.",
    handoff: {
      at: "2026-10-21 10:31",
      run: RUNS.paste,
      branch: "feature/paste-screenshots",
      done: ["Pasted images are stored and linked at the cursor"],
      next: [
        "Judge dropped files by their bytes",
        "Refuse files over the limit in words",
      ],
      files: [
        "frontend/src/features/editor/paste.ts",
        "internal/store/upload.go",
      ],
      questions: ["Convert pasted JPEGs to PNG, or keep them as they came?"],
    },
  },
  "FH-25": {
    plan: "Lay out lines with a fork, a join and a detour in unit tests, then look at the Workstreams view in both themes.",
    handoff: {
      at: "2026-10-21 09:58",
      run: RUNS.railway,
      agent: "codex",
      branch: "feature/branch-and-join",
      done: [
        "A chain keeps to the top track",
        "Independent work starts its own track",
      ],
      next: ["Route tracks round stations in the way"],
      files: ["frontend/src/features/workstreams/graph.ts"],
    },
  },
  "FH-38": {
    plan: "Narrow the window to 390 px with a card titled by a long file path and check it wraps.",
    handoff: {
      at: "2026-10-21 06:12",
      run: "c0ffee12-3456-4789-abcd-ef0123456789",
      branch: "fix/long-titles",
      done: ["Reproduced with a file path as the title"],
      next: ["Break long words in card titles", "Clamp titles to three lines"],
      files: ["frontend/src/features/board/TicketCard.css"],
    },
  },
  "TL-2": {
    handoff: {
      at: "2026-10-21 08:20",
      run: RUNS.tides,
      branch: "feature/offline-week",
      done: ["Overnight download of saved harbours"],
      next: ["Show how old the tables are"],
      files: ["app/src/tides/cache.ts"],
    },
  },
};

const isoMinutes = (date) => date.toISOString().replace(/\.\d+Z$/, "Z");

function createdDate(start, index) {
  const date = new Date(`${start}T09:00:00Z`);
  date.setUTCDate(date.getUTCDate() + Math.floor(index * 1.1));
  return date.toISOString().slice(0, 10);
}

function handoffSection({ at, run, agent = "claude", branch, ...lists }) {
  const bullets = (items) =>
    items?.length ? items.map((item) => `- ${item}`) : ["- None"];
  const lines = [
    "## Handoff",
    "",
    `_Updated ${at} UTC by ${agent}:${short(run)} (run) on ${branch}._`,
    "",
  ];
  if (lists.done?.length) lines.push("**Done**", ...bullets(lists.done));
  if (lists.next?.length) lines.push("**Next**", ...bullets(lists.next));
  lines.push("**Files**", ...bullets(lists.files));
  lines.push("**Open questions**", ...bullets(lists.questions));
  lines.push("");
  return lines;
}

function ticketFile(project, ids, row, index, now) {
  const [
    slug,
    title,
    type,
    priority,
    status,
    workstream,
    deps,
    hours,
    description,
    criteria,
    extra = {},
  ] = row;
  const id = ids.get(slug);
  const detail = details[id] ?? {};
  const ticked =
    extra.ticked ??
    (status === "done" || status === "review" ? criteria.length : 0);
  const branch =
    detail.handoff?.branch ??
    (["in-progress", "review"].includes(status) ? `feature/${slug}` : "");
  const changed = new Date(now.getTime() - hours * HOUR);
  const lines = [
    "---",
    `id: ${id}`,
    `status: ${status}`,
    `type: ${type}`,
    `priority: ${priority}`,
    `created: ${createdDate(project.start, index)}`,
    `branch: ${branch}`,
    "pr:",
    `workstream: ${workstream}`,
    `depends-on: [${deps.map((dep) => ids.get(dep)).join(", ")}]`,
    "depends-on-workstreams: []",
    `tags: [${(extra.tags ?? []).join(", ")}]`,
    `updated: ${isoMinutes(changed)}`,
    "---",
    "",
    `# ${title}`,
    "",
    "## Description",
    "",
    description,
    "",
    "## Acceptance Criteria",
    "",
    ...criteria.map(
      (criterion, n) => `- [${n < ticked ? "x" : " "}] ${criterion}`,
    ),
    "",
    type === "bug" ? "## Reproduction" : "## Test Plan",
    "",
    detail.plan ??
      (type === "bug"
        ? "Reproduce it first in a failing test, then fix it."
        : "Unit tests for the rules, then a browser check in both themes."),
    "",
  ];
  if (detail.context) lines.push("## Context", "", detail.context, "");
  lines.push(
    "## Notes",
    "",
    ...(detail.notes?.length ? detail.notes : ["None."]),
    "",
  );
  if (detail.handoff) lines.push(...handoffSection(detail.handoff));
  return { id, slug, workstream, changed, content: lines.join("\n") };
}

// writeShowcaseBoard writes the showcase root at root. repos maps a project
// directory to the checkouts its sessions work in (the real paths the hooks
// will report); now anchors file times.
export async function writeShowcaseBoard(
  root,
  { repos = {}, now = new Date() } = {},
) {
  await mkdir(join(root, ".flashheart"), { recursive: true });
  await writeFile(
    join(root, ".flashheart", "config.yaml"),
    [
      "version: 2",
      "auto_create_projects: true",
      "quiet_minutes: 10",
      `user_name: ${PERSON}`,
      "event_retention_days: 90",
      "done_column_limit: 20",
      "ui:",
      "  theme: system",
      "  density: normal",
      "",
    ].join("\n"),
  );
  const times = [];
  const written = {};
  for (const project of projects) {
    const ids = new Map(
      project.rows.map(([slug], index) => [
        slug,
        `${project.key}-${index + 1}`,
      ]),
    );
    const tickets = project.rows.map((row, index) =>
      ticketFile(project, ids, row, index, now),
    );
    for (const item of tickets) {
      const folder = `${item.id}-${item.slug}`;
      const dir = join(root, project.dir, "tickets", folder);
      await mkdir(dir, { recursive: true });
      await writeFile(join(dir, `${folder}.md`), item.content);
      times.push([dir, item.changed]);
    }
    const checkouts = repos[project.dir] ?? [`/Users/sam/src/${project.dir}`];
    await writeFile(
      join(root, project.dir, "project.yaml"),
      [
        `key: ${project.key}`,
        `next_id: ${tickets.length + 1}`,
        `name: ${project.name}`,
        "repos:",
        ...checkouts.map((path) => `  - ${path}`),
        "",
      ].join("\n"),
    );
    for (const line of workstreams[project.dir] ?? []) {
      const members = tickets.filter((item) => item.workstream === line.slug);
      const section = (heading, items) =>
        items?.length
          ? [`## ${heading}`, "", ...items.map((item) => `- ${item}`), ""]
          : [];
      const body = [
        "---",
        `slug: ${line.slug}`,
        "status: active",
        `priority: ${line.priority}`,
        `created: ${line.created}`,
        "tickets:",
        ...members.map((member) => `  - ${member.id}`),
        `depends-on-workstreams: [${(line.deps ?? []).join(", ")}]`,
        "tags: []",
        "---",
        "",
        `# ${line.title}`,
        "",
        "## Goal",
        "",
        line.goal,
        "",
        ...section("Scope", line.scope),
        ...section("Success Criteria", line.success),
        ...(line.notes ? ["## Notes", "", line.notes, ""] : []),
      ].join("\n");
      await mkdir(join(root, project.dir, "workstreams"), { recursive: true });
      await writeFile(
        join(root, project.dir, "workstreams", `${line.slug}.md`),
        body,
      );
    }
    written[project.dir] = new Map(
      tickets.map((item) => [item.id, `${item.id}-${item.slug}`]),
    );
  }
  await writeActivity(root, written.flashheart.get("FH-10"), now);
  await settle(root, times, now);
  return written;
}

// writeActivity gives the ticket in review a history and a comment, so its
// page's Activity reads like a real one.
async function writeActivity(root, folder, now) {
  const at = (hours) => isoMinutes(new Date(now.getTime() - hours * HOUR));
  const run = `claude:${RUNS.review}`;
  await writeFile(
    join(root, "flashheart", "tickets", folder, "activity.md"),
    [
      "# Activity: FH-10",
      "",
      `### ${at(140)} · history · ${PERSON} · 1f4a7c2e`,
      "",
      "- created: backlog",
      "",
      `### ${at(96)} · history · ${PERSON} · 3b6d9e1a`,
      "",
      "- moved: backlog → up-next",
      "",
      `### ${at(50)} · history · ${run} · 5c8e0a3b`,
      "",
      "- moved: up-next → in-progress",
      "- edited: branch",
      "",
      `### ${at(30)} · comment · ${PERSON} · 7e0a2c4d`,
      "",
      "state: resolved",
      "",
      "Keep the choices as buttons: most questions I get are yes or no.",
      "",
      `### ${at(20)} · history · ${run} · 9a2c4e6f`,
      "",
      "- ticked: criterion 1, criterion 2, criterion 3, criterion 4",
      "- moved: in-progress → review",
      "",
    ].join("\n"),
  );
}

// settle sets each ticket folder's files to the ticket's last change, and
// everything else an hour before now.
async function settle(root, times, now) {
  const earlier = new Date(now.getTime() - HOUR);
  const walk = async (dir) => {
    for (const entry of await readdir(dir, { withFileTypes: true })) {
      const path = join(dir, entry.name);
      if (entry.isDirectory()) await walk(path);
      await utimes(path, earlier, earlier);
    }
  };
  await walk(root);
  for (const [dir, changed] of times) {
    for (const entry of await readdir(dir)) {
      await utimes(join(dir, entry), changed, changed);
    }
    await utimes(dir, changed, changed);
  }
}

// reviewMarkdown is FH-10's review, whose evidence is a screenshot the spec
// takes of the board itself (evidence names its file).
export function reviewMarkdown(folder, evidence) {
  return [
    "# Review: Answer agents' questions from the board",
    "",
    `**Work Item:** [FH-10](${folder}.md)`,
    "",
    "## Summary",
    "",
    "An agent's question now waits on its ticket's card and on the Overview instead of in a terminal. Choices answer in one click, and the answer reaches the session with its next prompt, once.",
    "",
    "## Evidence",
    "",
    `![A question waiting on the Overview](files/${evidence})`,
    "",
    "## How to Verify",
    "",
    "1. Ask a question from a session with `ask_human`.",
    "2. Open the board: the ticket's card says it needs you.",
    "3. Answer it with one of its choices.",
    "4. Send the session a prompt: it is told the answer, and only once.",
    "",
    "## Risks / Things to Watch",
    "",
    "- A session that never prompts again never hears the answer; the Overview keeps it under Needs you.",
    "",
  ].join("\n");
}

// checkShowcaseBoard asserts what the README's shots rely on: tickets in all
// five columns, and a workstream that waits for two others.
export function checkShowcaseBoard() {
  const columns = new Set(flashheart.map((row) => row[4]));
  for (const status of ["backlog", "up-next", "in-progress", "review", "done"])
    if (!columns.has(status))
      throw new Error(`showcase board has no ticket in ${status}`);
  if (!workstreams.flashheart.some((line) => (line.deps ?? []).length >= 2))
    throw new Error("no showcase workstream waits for two others");
  const slugs = new Set(flashheart.map((row) => row[0]));
  for (const row of flashheart)
    for (const dep of row[6])
      if (!slugs.has(dep)) throw new Error(`${row[0]} needs unknown ${dep}`);
  return { tickets: flashheart.length, workstreams: workstreams.flashheart };
}

if (import.meta.url === `file://${process.argv[1]}`) {
  const target = process.argv[2];
  if (!target) {
    console.error("usage: node showcase-board.mjs <empty directory>");
    process.exit(2);
  }
  checkShowcaseBoard();
  await writeShowcaseBoard(resolve(target));
}
