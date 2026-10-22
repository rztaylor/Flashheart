# Codex

Flashheart works with Codex as it does with Claude Code (see
[Claude Code](claude-code.md) for tickets, questions and what gets
recorded): **hooks** record what every session does, and **the `flashheart`
MCP server** lets the model claim tickets, record handoffs, ask you
questions and write reviews. Codex gets the same Flashheart skill, in
`~/.agents/skills`, and a short pointer to it in its global `AGENTS.md`.

Works in the Codex CLI, the IDE extension and the ChatGPT app, which share
`~/.codex`. Checked against Codex 0.160.0 (2026-10-12); the skill folder
against Codex 0.162 (2026-10-22).

One difference: a Codex session that changes files without a ticket is
asked to claim one with your next prompt (or, with handoff enforcement
on, when it tries to stop), not with a tool result, since Codex is not
known to show the model what a tool result's hook says.

To move a project's existing to-do lists onto the board, see
[Moving an existing to-do list onto the board](migrating.md).

## Set it up

```sh
flashheart setup codex
```

`setup codex` shows what it would change, then asks
`Apply these changes? [y/N]` and writes only if you answer `y`. `--yes`
applies without asking (for scripts; `--write` is its older name), and
`--dry-run` only shows the changes; with no terminal to ask in it only
shows them unless you pass `--yes`. It works in
`$CODEX_HOME` when you set it, as Codex does, else in `~/.codex`, and shows,
as a diff:

- the MCP server it adds to `config.toml` as `[mcp_servers.flashheart]`,
  running `<path to flashheart> mcp`. Only that table's `command` and
  `args` are set; your comments, other servers, project trust and anything
  else you put in the table stay as they are;
- the hooks it merges into `hooks.json`: every event Flashheart handles,
  each running `<path to flashheart> hook codex <Event>`. Your other hooks
  are kept, and a Flashheart hook an earlier version registered that is no
  longer needed (`PreToolUse`) is removed;
- the Flashheart skill it installs in `~/.agents/skills/flashheart/`, where
  Codex finds your own skills: a short guide, `SKILL.md`, and the pages it
  points to in `references/`, which the model reads when it reaches that
  task. A Flashheart skill you copied there by hand is replaced. When
  `~/.agents/skills` is a link into your own repository, the skill is
  written there, so you may want to ignore `flashheart/` in it. If
  `~/.claude/skills` links to the same folder, Claude Code and Codex share
  the skill, and uninstalling one leaves it for the other;
- a short section it adds to your global `AGENTS.md`, which Codex reads in
  every session, saying that Flashheart tracks your work and to use the
  skill. It sits between `<!-- BEGIN FLASHHEART … -->` and
  `<!-- END FLASHHEART -->`; text outside the markers is yours, text inside
  is replaced by the next setup, and an `AGENTS.md` that links into your
  dotfiles is edited where it points.

After you upgrade Flashheart, the next Codex session in a git repository
brings the skill and that section up to date by itself, once you have
trusted the hooks (below): its session-start hook rewrites only the
skill's files and the text between the markers, and tells the session it
did so it reads them again. It never adds the section to an `AGENTS.md`
without it (so an uninstall stays uninstalled), and leaves a file whose
markers are broken alone for `flashheart doctor` to report. Setups from
before the skill had the whole protocol text in that section: the first
session start after the upgrade installs the skill, then shrinks the
section to its pointer.

Before writing, setup copies everything it changes into
`~/.codex/flashheart-backup/<time>/`. Commands use the absolute path of the
`flashheart` you ran, and pass `--root` only when your board is not at
`~/reports/Kanban`. If Codex changed `config.toml` between the diff and the
write (it saves trust there), setup stops and asks you to run it again.

### Trust the hooks

Codex runs hooks only once you trust them. After setup has written them:

1. run `codex` in a terminal;
2. type `/hooks` and trust each Flashheart hook;
3. quit, and start a new Codex thread.

The ChatGPT app never asks: it silently skips hooks you have not trusted in
the CLI, and uses the trust you give there. Trust belongs to a hook's exact
command, so trust them again after running setup with a moved binary or
another `--root`; `flashheart doctor` names the hooks that need trusting,
or trusting again. Without trusted hooks Codex still uses the MCP server:
your board actions are recorded, but not the session's own activity, and
answers to questions arrive with its next Flashheart tool call.

To undo it:

```sh
flashheart setup codex --uninstall   # shows what would change, then asks
```

Uninstalling removes only Flashheart's table, hooks, skill and marked
text, and deletes a file setup created that would be left empty.

## When something goes wrong

`flashheart doctor` has a Codex section: the MCP server, the hooks, each
file of the skill and the section of `AGENTS.md` against what
`setup codex` writes. A missing entry, a missing skill or a binary that no
longer exists is a problem; another binary, a hook an earlier version
registered that setup no longer writes, a missing section, an out-of-date
skill or section (after an upgrade, until the next Codex session
refreshes them or you run `setup codex` again),
hooks turned off with `[features] hooks = false`, or hooks never
trusted with `/hooks`, are warnings. Doctor cannot tell whether trust is
still current after a hook changed, so trust them again when setup changed
them. Without a Codex home, the section only says Codex is not installed.
