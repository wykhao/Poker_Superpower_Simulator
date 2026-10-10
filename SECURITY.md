# Security model

This is a single-user application bound to loopback. It is not an authenticated multi-user web service and should not be exposed through a public proxy or tunnel.

The server rejects non-local Host headers, requires same-origin JSON mutation requests with a custom header, limits request size, serves only embedded assets, and never exposes a pending hand through history/export APIs. Human actions carry hand ID and turn revision to reject stale or duplicate submissions. An OS file lock prevents multiple writers to the same history directory.

The local computer owner can read the program, inspect storage and derive cards from saved seeds. A/B isolation protects ordinary UI/API play; it is not designed as adversarial anti-cheat.

If reporting a bug, use the repository's private vulnerability reporting feature if its maintainer enables it. Do not publish personal `history.json` files or operating-system paths in public issues. A small synthetic reproduction is preferable.
