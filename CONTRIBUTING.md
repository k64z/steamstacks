# Contributing

Issues and PRs are welcome. This is a personal project maintained
actively; expect a response within a few days.

## Ground rules

- `go build ./... && go vet ./... && go test -race ./...` and
  `gofmt -l .` clean — CI enforces all four.
- Tests are hermetic: `httptest` servers and fixtures, never the live
  network. Fixtures must be synthetic — placeholder SteamIDs
  (`7656119800000000x`), invented asset IDs, no captured pages from real
  sessions.
- Never commit credentials, tokens, session files, or authenticator
  secrets — not even in test data. `steam_session*.json` is gitignored;
  keep it that way.
- Steam's unofficial surface changes without notice. When a change
  adapts to Steam behavior, say in the commit message what changed on
  Steam's side and how you verified the new behavior — the commit
  history is this project's changelog of Steam quirks.
- New endpoints should follow the existing patterns: `context.Context`
  first parameter, functional options, sentinel errors for conditions
  callers must distinguish.

## Reporting Steam breakage

If an endpoint stopped working, an issue with the failing method name,
the HTTP status/EResult you see, and a date is enough — that's usually a
Steam-side change and gets prioritized.
