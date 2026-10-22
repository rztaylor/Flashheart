# Writing tickets

Read this before you create a ticket, edit one with `update_ticket`, or
`checkpoint`. The board is how the user sees the work, so write each ticket
for someone who has not read the code.

## Titles and descriptions

- The title says what a person would notice or gain, in plain words and
  under 80 characters, with no file, function, test or type names.
  Instead of: "Derive the run's needs-you reason once on the server";
  write: "Card and Runs tab can disagree about why a session needs you".
- The description, in plain words: the problem as someone experiences it,
  why it matters (who it affects, what it costs), and the proposed change.
- Implementation detail (files, functions, design notes, exact rules)
  goes in `context`, which the next session reads.

## Handoffs

`checkpoint` rewrites the ticket's handoff: what is done, what is next, the
files that matter, and open questions. The next session resumes from it.

- The handoff is what the person reviewing the ticket reads first: each
  done and next item is a sentence or two saying what and why. Instead
  of: "Call Scope.Validate in contribute()"; write: "Refuse a malformed
  scope when a fact is added".
- An open question is one a person can answer.
- Exact rules and names from the code go in Context, not the handoff.
- `note` appends a paragraph to the ticket's notes; `files` lists the files
  that matter, repository-relative.

## Criteria and conventions

- Types: `feature`, `bug`, `test`, `refactor`, `infra`, `docs`, `spike`.
  Priorities: `high`, `medium`, `low`.
- One ticket per unit of work: a single feature, fix or task, not an epic.
  Work spanning several tickets belongs in a workstream (workstreams page).
- Acceptance criteria are testable statements. Tick each with
  `update_ticket`'s `check` as it passes.
- Feature tickets carry a Test Plan (which tests will prove it) and follow
  red, green, refactor: failing tests first, then the implementation.
- Bug tickets carry a Reproduction: write a failing test that shows the
  defect before fixing it, and name that test in the review.
- Test tickets add coverage; bugs they find become bug tickets.
- Spikes produce a written recommendation and follow-up tickets, not
  production code.
