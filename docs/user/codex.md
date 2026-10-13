# Codex

Flashheart works with Codex as it does with Claude Code (see
[Claude Code](claude-code.md) for tickets, questions and what gets
recorded): **hooks** record what every session does, and **the `flashheart`
MCP server** lets the model claim tickets, record handoffs, ask you
questions and write reviews. Instead of a skill, Codex gets the same
protocol text in its global `AGENTS.md`.

Works in the Codex CLI, the IDE extension and the ChatGPT app, which share
`~/.codex`. Checked against Codex 0.160.0 (2026-10-12).

## Set it up

```sh
flashheart setup codex            # show what would change
flashheart setup codex --write    # apply it, with backups
```

`setup codex` changes nothing until you add `--write`. It works in
`$CODEX_HOME` when you set it, as Codex does, else in `~/.codex`, and shows,
as a diff:

- the MCP server it adds to `config.toml` as `[mcp_servers.flashheart]`,
  running `<path to flashheart> mcp`. Only that table's `command` and
  `args` are set; your comments, other servers, project trust and anything
  else you put in the table stay as they are;
- the hooks it merges into `hooks.json`: every event Flashheart handles,
  each running `<path to flashheart> hook codex <Event>`, and `PreToolUse`
  limited to Flashheart's own tools. Your other hooks are kept;
- the Flashheart protocol text it adds to your global `AGENTS.md`, between
  `<!-- BEGIN FLASHHEART … -->` and `<!-- END FLASHHEART -->`. Text outside
  the markers is yours; text inside is replaced by the next setup.

`--write` first copies everything it changes into
`~/.codex/flashheart-backup/<time>/`. Commands use the absolute path of the
`flashheart` you ran, and pass `--root` only when your board is not at
`~/reports/Kanban`. If Codex changed `config.toml` between the diff and the
write (it saves trust there), setup stops and asks you to run it again.

### Trust the hooks

Codex runs hooks only once you trust them. After `--write`:

1. run `codex` in a terminal;
2. type `/hooks` and trust each Flashheart hook;
3. quit, and start a new Codex thread.

The ChatGPT app never asks: it silently skips hooks you have not trusted in
the CLI, and uses the trust you give there. Trust belongs to a hook's exact
command, so trust them again after running setup with a moved binary or
another `--root`. Without trusted hooks Codex still uses the MCP server:
your board actions are recorded, but not the session's own activity, and
answers to questions arrive with its next Flashheart tool call.

To undo it:

```sh
flashheart setup codex --uninstall           # show what would change
flashheart setup codex --uninstall --write   # undo it
```

Uninstalling removes only Flashheart's table, hooks and marked text, and
deletes a file setup created that would be left empty.

## When something goes wrong

`flashheart doctor` has a Codex section: the MCP server, the hooks and the
protocol text against what `setup codex` writes. A missing entry or a
binary that no longer exists is a problem; another binary, out-of-date
protocol text (after you upgrade Flashheart, run `setup codex --write`
again), hooks turned off with `[features] hooks = false`, or hooks never
trusted with `/hooks`, are warnings. Doctor cannot tell whether trust is
still current after a hook changed, so trust them again when setup changed
them. Without a Codex home, the section only says Codex is not installed.
