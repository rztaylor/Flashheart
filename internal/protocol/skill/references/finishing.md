# Finishing a ticket

Read this before you finish a ticket.

1. Tick the criteria that pass (`update_ticket`).
2. Show the evidence. A change with a visible effect (UI, rendered output,
   an image, terminal output someone would otherwise reproduce) needs
   screenshots saved as files, one for every state the change touched
   (each theme, narrow widths, empty and error states), each with a
   caption, linked in the review's Evidence section by absolute path;
   Flashheart copies them into the ticket. `attach` does the same for a
   file on its own. Screenshots that only reached your context do not
   count. With nothing visible, say so instead: "No visible change:
   <why>". Moving to review warns when neither is there.
3. `write_review` using the template below.
4. `checkpoint`, then `move` the ticket to `review`. Never move a ticket to
   `done`; the human does that after verifying.

If you opened a pull request, its URL is the ticket's `pr`
(`update_ticket`).

## Review template

```markdown
# Review: <title>

## Summary
<2–4 sentences: what changed and why>

## Key Files
| File | What changed |
|------|--------------|

## Evidence
<captioned screenshots linked by absolute path, or "No visible change: why">

## How to Verify
### Prerequisites
### Steps
1. <concrete step>
### Expected Results

## Risks / Things to Watch

## Tests
<tests written and their result (feature and bug tickets)>
```
