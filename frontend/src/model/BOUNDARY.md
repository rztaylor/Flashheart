# Model boundary

Owns pure view-model logic shared across features: workstream line
assignment (`lines`), filters and search (VIEW-7), running times, markdown
link resolution and section trimming (CARD-2), and keyboard grid movement.
Everything here is a plain function or type, unit-tested with Vitest.

Does not own React components, data loading, requests or styling. It may
import API types; nothing here imports components, features or the app.
