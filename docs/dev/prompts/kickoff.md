# Kickoff prompt

Paste the block below into a new Claude Code or Codex session opened at
`~/src/Flashheart`. It covers the first two roadmap items with a stop for
review after each. Later items get their own prompts, written from their
briefs when they start.

---

```text
You are implementing Flashheart, a local single-binary Go + React app that is
both a multi-project kanban board and an MCP server/hook handler for AI coding
agents. The repository currently contains only the specification and
foundation documents.

## Read first (in this order)

1. AGENTS.md
2. docs/SPEC.md (authoritative; cite requirement ids such as LIFE-1, STO-4)
3. docs/dev/specs/board-format.md and docs/dev/specs/agent-protocol.md
4. .agents/facts/*.md (architecture, frontend-ui, testing, go, node, cli, git)
5. docs/dev/decisions.md
6. docs/dev/roadmap.md, then docs/dev/roadmap-items/foundation.md and
   docs/dev/roadmap-items/board-core.md
7. testdata/boards/sample/README.md
8. The Singleserve consumer guide:
   ~/src/singleserve/docs/dev/guides/building-consumer-apps.md and its
   example ~/src/singleserve/examples/minimal. ~/src/DiskOrbit is a working
   Singleserve + React + Vite app by the same author: reuse its patterns
   (internal/app, internal/webui, frontend lifecycle, scripts/check.sh,
   Playwright lifecycle tests) rather than inventing new ones.

Before writing code, restate in a few bullets: the goal of `foundation`, its
acceptance criteria, the files and packages you will create, the library
versions you will pin, and anything you think the brief gets wrong. Then
proceed without waiting unless something blocks you.

## Ground rules

- Work on a feature branch: `feature/foundation`, later `feature/board-core`.
  Commit in focused steps. Do not push, add a remote or open a PR unless I ask.
- Test first for every feature: write the failing test, then the code.
- Every hand-written Go package gets a concise doc.go boundary contract;
  every frontend feature folder and shared layer gets a BOUNDARY.md
  (≤150 words). Use the document-code-boundaries skill if available.
- Verify current versions before pinning: github.com/rztaylor/singleserve
  (v0.2.1 or later in the v0.2 line), github.com/modelcontextprotocol/go-sdk
  (needed later, not in foundation), the maintained YAML v3 module
  (go.yaml.in/yaml/v3 vs gopkg.in/yaml.v3; pick the maintained one and record
  it in docs/dev/decisions.md D10), Tailwind CSS v4 with @tailwindcss/vite,
  React, Vite, Biome, Vitest, Playwright. Prefer the toolchain versions
  DiskOrbit already uses where they are current.
- Never read or write the real board root (~/reports/Kanban) or real agent
  configuration (~/.claude, ~/.codex) in tests or during development. Use temp
  directories and copies of testdata/boards/sample.
- No network access in the app beyond the loopback listener. No telemetry.
- Keep facts, specs, roadmap and CHANGELOG.md accurate in the same commits as
  the behaviour they describe. Make scripts/check.sh strict (remove the
  pre-foundation skips) as part of foundation.

## Singleserve integration (required)

- Use singleserve.New with the app http.Handler and BrowserBoundLifetime.
- Start, open the browser, print the manual URL to stderr once if opening
  fails, then wait for shutdown.
- In the browser, import connect from /_singleserve/client.js, call it once
  in src/lifecycle/, and provide onHeartbeat and onServerUnavailable.
- Use session.fetch for every API call. Provide quiet backend status, a health
  check, Quit wired to session.requestShutdown with denial handling, and a
  terminal state that stops the session, attempts window.close and shows
  manual close instructions.
- Do not parse Launch.URL, expose tokens, use Web Storage, send
  X-Singleserve-Token from JavaScript, or add routes under /_singleserve/.
- Verify in a real browser (Playwright): initial launch, reload, new tab,
  last-tab close, guarded shutdown denial and acceptance, backend loss.

## Stage 1: foundation

Implement docs/dev/roadmap-items/foundation.md in full. When its acceptance
criteria pass and scripts/check.sh is green:

- update docs/dev/roadmap.md (remove or mark the item), CHANGELOG.md and any
  facts that changed;
- write a short review note: what was built, how to run it, how you verified
  it (commands and results), deviations from the brief, and open questions;
- STOP and wait for my review before starting board-core.

## Stage 2: board-core (after I approve stage 1)

1. Implement the non-UI parts test-first: internal/mdfile, internal/board
   (blocking with reasons), internal/store read side with path confinement,
   internal/index, and the read API. Test against copies of
   testdata/boards/sample, including the broken-frontmatter ticket, the
   missing dependency and the malformed event line.
2. Before any board UI code, run the ui-concept-design skill (if unavailable,
   produce three distinct written concepts — Editorial, Calm, Precision — with
   static HTML mockups) for the shell, Board, card, card panel and
   Workstreams view, then STOP for my approval. Record the approved direction
   in .agents/facts/frontend-ui.md and DESIGN.md.
3. Implement the approved UI per the board-core brief using the frontend-ui
   skill if available: tokens in src/styles/tokens.css via Tailwind @theme,
   shared components before features, light and dark themes, keyboard
   navigation, axe-clean.
4. Verify with scripts/check.sh and Playwright screenshots at 1280 and 1920 in
   both themes; inspect the screenshots yourself.
5. Update roadmap, changelog and facts; write the review note; STOP.

## Handoff format at each stop

- Summary (3–5 bullets)
- How to run and what I should look at
- Verification evidence (commands, test counts, screenshots paths)
- Deviations from the spec or brief, and why
- Open questions and proposed next step
```
