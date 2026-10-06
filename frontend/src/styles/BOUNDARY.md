# Styles boundary

Owns design tokens (`tokens.css`: colour, type, radius, spacing, focus,
motion) for both palettes, Metro Pop light and Night Service dark, on the
semantic roles of `docs/dev/specs/ui-layout.md` §7, and global base styles
(`index.css`). Tokens are CSS custom properties switched by
`<html data-theme>` and exposed to Tailwind through `@theme`, so components
use token-backed utilities and never raw values; a theme changes values,
never layout. `tokens.test.ts` holds both palettes to WCAG AA and the dark
neutrals to no hue.

Also owns the shared CSS utilities (heading and display cuts, hatching) and
keyframes (panel and toast entrances, the run beat), each switched off
under reduced motion. Values follow `DESIGN.md`.

Does not own components, layout or behaviour. Every other frontend layer may
depend on it; it depends on nothing.
