# Test support boundary

Owns setup shared by the interaction tests (`*.dom.test.ts(x)`, Vitest's
`dom` project in happy-dom, D42): `dom-setup.ts` unmounts what Testing
Library rendered after each test and makes happy-dom fire
`selectionchange` as a queued task, as browsers do.

Does not own tests themselves, which sit beside the code they test, or
fixtures for one feature. Nothing in the app imports from here, and it is
not part of the production build.
