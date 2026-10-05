# Git facts

- Remote: `origin` is `https://github.com/rztaylor/Flashheart` (public);
  default branch `main`.
- Every change lands through a pull request from a
  `feature/<short-description>` branch; `main` is never committed to or
  merged into locally. Run `pre-pr-review` before opening a pull request;
  CI (`.github/workflows/ci.yml`) must pass before merging.
- Commits are focused; facts, specs, roadmap and changelog change in the same
  commit as the behaviour they describe.
- Never commit board data, recorded hook payloads that are not scrubbed, or
  agent configuration from the developer's machine.
