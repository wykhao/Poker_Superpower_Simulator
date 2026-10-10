# Third-party notices and provenance

## PokerForBots

- Upstream: https://github.com/lox/pokerforbots
- Reference revision: `6d9099dbbf46758e33d209d0ef451f2cec6d7841`
- License: MIT, copyright (c) 2025 Lachlan Donald.
- The original license is retained verbatim at `licenses/PokerForBots-MIT.txt`.

The following packages are adapted from that project: `internal/game`, `internal/randutil`, `poker`, `protocol`, `sdk/analysis`, `sdk/classification`, `sdk/client`, `sdk/config`, and `sdk/bots/complex`.

Local adaptations retained from the verified experiment code:

1. Imports use this standalone module path, `poker-human-simulator`.
2. `complex.NewSeededHandler` supplies independent explicit randomness without changing the bot's strategy tables or decisions.
3. Pot settlement preserves odd chips, awarding them clockwise from the button separately for each pot.
4. Best-five evaluation handles five, six or seven cards; a six-high or higher straight takes precedence over an available A2345 wheel.
5. The betting-only engine method remains part of the engine adapter. This app uses the regular hand progression and has no automatic human policy.

No A/B equity policy, response-bluff policy, sizing-search policy, range inference, face-up forced-call rules, experiment results or prior player data is included.

## Go dependencies

Versions are pinned in `go.mod`/`go.sum`; source and license files are included in `vendor/`. The release script copies applicable `LICENSE`, `COPYING`, and `NOTICE` files into binary archives under `licenses/dependencies/` and includes the Go distribution's BSD license.

Direct/transitive dependencies include Gorilla WebSocket, zerolog, msgp, go-colorable, go-isatty, philhofer/fwd, and golang.org/x/sys. Their own license files govern those components; this project's MIT license does not replace them.

The optional Playwright test runner is a development tool, not part of the distributed application. System fonts are used and no external font or image service is requested by the UI.
