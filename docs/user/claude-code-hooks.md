# Claude Code hooks

Flashheart sees your Claude Code sessions through hooks: Claude Code runs
`flashheart hook claude <Event>` at each step of a session and Flashheart
records what happened in the board's event log. The board then shows every
session live in the **Agents** view, puts a badge on the ticket a session is
working on, and lifts sessions that need you (a permission prompt) into a
**Needs you** column and the top bar.

`flashheart setup claude` will write this configuration for you once the
`mcp-protocol` roadmap item lands. Until then, add it by hand.

Works in the Claude Code CLI, the IDE extensions and the Claude desktop
app's Code tab, which runs the same Claude Code and reads
`~/.claude/settings.json` (user-level hooks reach every session; project
settings are read from the main checkout, not a worktree). Ordinary chats in
the Claude app do not run Claude Code and have no hooks. Checked against
Claude Code 2.1.288, CLI and desktop app (2026-10-05).

## Add the hooks

1. Find the absolute path of the binary: `command -v flashheart`.
2. Open `~/.claude/settings.json` (all projects) or a repository's
   `.claude/settings.json` (that repository only), and merge in the `hooks`
   below, replacing `/usr/local/bin/flashheart` with your path. Keep any
   hooks you already have; each event takes a list.
3. If your board is not at `~/reports/Kanban`, add `--root /path/to/board`
   to every command.
4. Start a new Claude Code session. Hooks are read when a session starts.

```json
{
  "hooks": {
    "SessionStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/flashheart hook claude SessionStart",
            "timeout": 5
          }
        ]
      }
    ],
    "UserPromptSubmit": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/flashheart hook claude UserPromptSubmit",
            "timeout": 5
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/flashheart hook claude PostToolUse",
            "timeout": 5
          }
        ]
      }
    ],
    "PostToolUseFailure": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/flashheart hook claude PostToolUseFailure",
            "timeout": 5
          }
        ]
      }
    ],
    "PermissionRequest": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/flashheart hook claude PermissionRequest",
            "timeout": 5
          }
        ]
      }
    ],
    "PermissionDenied": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/flashheart hook claude PermissionDenied",
            "timeout": 5
          }
        ]
      }
    ],
    "Notification": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/flashheart hook claude Notification",
            "timeout": 5
          }
        ]
      }
    ],
    "TaskCreated": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/flashheart hook claude TaskCreated",
            "timeout": 5
          }
        ]
      }
    ],
    "TaskCompleted": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/flashheart hook claude TaskCompleted",
            "timeout": 5
          }
        ]
      }
    ],
    "SubagentStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/flashheart hook claude SubagentStart",
            "timeout": 5
          }
        ]
      }
    ],
    "SubagentStop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/flashheart hook claude SubagentStop",
            "timeout": 5
          }
        ]
      }
    ],
    "PreCompact": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/flashheart hook claude PreCompact",
            "timeout": 5
          }
        ]
      }
    ],
    "PostCompact": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/flashheart hook claude PostCompact",
            "timeout": 5
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/flashheart hook claude Stop",
            "timeout": 5
          }
        ]
      }
    ],
    "SessionEnd": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/flashheart hook claude SessionEnd",
            "timeout": 5
          }
        ]
      }
    ]
  }
}
```

Flashheart does not register `PreToolUse`: it runs before every tool and
would only add latency. It arrives later, limited to Flashheart's own MCP
tools.

## What gets recorded

Into `<root>/<project>/.flashheart/events/<date>.jsonl`, where the project is
named after the repository's main checkout (worktrees share it) and is
created on first contact with a `project.yaml` but no ticket key yet:

- session and subagent starts and ends, prompts submitted (never their
  text), turns finished and compaction;
- tool names, success or failure, and for edits the file path relative to
  the repository;
- the session's own task list (`TodoWrite` and the task tools), as its plan;
- permission requests and notifications, by type only.

Prompts, commands, tool inputs and outputs, and message text are never
stored. Plan text and paths are scrubbed of anything that looks like a
secret.

## Linking sessions to tickets

A session is linked to a ticket when exactly one **In progress** ticket in
its project has `branch:` set to the session's branch. Linked sessions show
on the ticket's card and in its **Runs** tab, and a new session in the same
worktree is told about the ticket and its handoff when it starts. Explicit
claims arrive with the MCP tools.

## When something goes wrong

The hook never interrupts Claude Code: it always exits 0 and prints nothing
except the recovery note at session start. Problems are logged to
`<root>/.flashheart/hook-errors.log`. To remove Flashheart, delete its
entries from `settings.json`.
