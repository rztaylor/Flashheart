# Styles boundary

Owns design tokens (`tokens.css`: colour, type, radius, spacing, focus,
motion) and global base styles (`index.css`). Tokens are CSS custom
properties switched by `<html data-theme>` and exposed to Tailwind through
`@theme`, so components use token-backed utilities and never raw values.

Also owns the shared CSS utilities (station sign, hatching, platform ground)
and keyframes (panel and toast entrances, the run beat), each switched off
under reduced motion. Values follow `DESIGN.md`.

Does not own components, layout or behaviour. Every other frontend layer may
depend on it; it depends on nothing.
