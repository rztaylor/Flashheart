# Editor feature boundary

Owns the formatted markdown editor (FH-16, D61): CodeMirror editing the
markdown text in a shadow root (the CSP), the live preview, the slash
menu, the selection toolbar, formatting and task commands, the Formatted
and Markdown views (`mode`), paste including a web page's HTML as
markdown (`paste`, `htmlToMarkdown`, D68), files pasted, dropped or
attached at the cursor (`uploads`, FH-18), and `MarkdownField`, the lazy
entry the rest of the app uses.

Does not own sending a file (the caller's `upload`), file rules
(`model/uploads`), the open item, saving, conflicts and their shortcuts
(`editing/`), which sections edit (`card/`), link rules
(`model/markdown`), section text (`model/sectionEdit`), the saved
preference (`state/`, `app/`) or the read view (`components/Markdown`).
