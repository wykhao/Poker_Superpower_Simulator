# Contributing

This is a single-human, local poker experiment. Changes should preserve these invariants:

1. Seat 0 is always human-controlled. Production code must not choose its actions.
2. A receives opponent hole cards, including folded cards, but no live future board or seed.
3. B receives the scheduled board but no live opponent hole cards, including through APIs, logs or exports.
4. The five bots use the original `complex` policy. New opponent variants must be explicit and have separate statistics/rules versions.
5. Every completed hand is zero-sum. All wins and losses count; interrupted hands resume instead of disappearing.
6. No real-money functionality, remote listening default, telemetry, secrets or player data belongs in the repository.

Before a change is ready:

```sh
gofmt -w <changed-go-files>
go test ./...
go vet ./...
```

Use `go test -race ./internal/app ./internal/table ./internal/web` where a supported C toolchain is installed. UI changes should be checked at desktop and phone widths with `tests/browser.cjs` or a browser. Update documentation and `CHANGELOG.md` when behavior changes.

The `vendor/` folder is intentional. After changing a dependency, run `go mod tidy` and `go mod vendor`, preserving license notices. Release archives are built from explicit allowed files, not from the entire working directory.

Keep upstream licensing and attribution. Describe whether a change touches poker rules, bot strategy, visibility boundaries, persistence, statistics or only presentation; include relevant validation.
