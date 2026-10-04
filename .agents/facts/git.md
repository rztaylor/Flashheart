# Git facts

- Default branch: `main`. Local repository at `~/src/Flashheart`; no remote
  yet. Decision needed: GitHub remote (`rztaylor/Flashheart` assumed); add
  only when the user asks.
- The bootstrap commit (spec and foundation docs) went to `main`; all later
  changes use `feature/<short-description>` branches and pull requests.
- Commits are focused; facts, specs, roadmap and changelog change in the same
  commit as the behaviour they describe.
- Never commit board data, recorded hook payloads that are not scrubbed, or
  agent configuration from the developer's machine.
