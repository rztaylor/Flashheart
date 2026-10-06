# Claude Code setup fixtures

`settings.json` is a `~/.claude/settings.json` as a user might have it before
running `flashheart setup claude`: their own hooks (a notification sound, a
formatter), permissions, non-ASCII text and other settings that setup must
leave exactly as they are. Tests copy it into a temporary home.
