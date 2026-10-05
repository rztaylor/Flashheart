# Security policy

Flashheart runs only on your machine: it serves a loopback-only, authenticated
browser page and reads and writes files under one board root. It makes no
network calls and has no accounts or telemetry.

## Supported versions

Flashheart is pre-release. Only the latest `main` receives fixes.

## Reporting a vulnerability

Please report vulnerabilities privately through GitHub's
[private vulnerability reporting](https://github.com/rztaylor/Flashheart/security/advisories/new),
not in public issues. If that form is unavailable, open an issue asking for a
private contact, without details of the problem. Include the version or
commit, your platform, and steps to reproduce. You should hear back within a
week.

Of particular interest: anything that lets a web page, another local user or
agent-written ticket text read or write outside the board root, bypass the
local server's authentication, or run commands.
