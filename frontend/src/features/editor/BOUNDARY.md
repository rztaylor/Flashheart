# Editor feature boundary

Owns the formatted markdown editor (FH-16, D61): CodeMirror editing the
markdown text itself in a shadow root (the CSP), the live preview
(syntax hidden off the edited line, task, image, bullet and rule
widgets, links and ticket ids as the view resolves them), the slash menu,
the selection toolbar, the formatting and task commands, the Formatted
and Markdown views and its context (`mode`), plain-text paste, files
pasted, dropped or chosen with Attach files going in at the cursor
behind a placeholder (`uploads`, FH-18), and `MarkdownField`, the lazy
entry the rest of the app uses, given an `upload` to attach files.

Does not own sending a file (the caller's `upload`), file names,
references and refusal words (`model/uploads`), which item is open,
saving, conflicts or Escape and
Ctrl+Enter (`editing/`), which sections edit and the criteria list
(`card/`), link rules (`model/markdown`), section text
(`model/sectionEdit`), the saved preference (`state/`, `app/`) or the
read view (`components/Markdown`).
