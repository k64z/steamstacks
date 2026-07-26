# steamstacks

[![CI](https://github.com/k64z/steamstacks/actions/workflows/ci.yml/badge.svg)](https://github.com/k64z/steamstacks/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/k64z/steamstacks.svg)](https://pkg.go.dev/github.com/k64z/steamstacks)
[![Go Report Card](https://goreportcard.com/badge/github.com/k64z/steamstacks)](https://goreportcard.com/report/github.com/k64z/steamstacks)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A Go library for the whole Steam surface: logging in (with Steam Guard),
trading, the Community Market, inventories, the store, and the real-time
Connection Manager protocol the desktop client speaks — plus a Team
Fortress 2 Game Coordinator client built on top of it.

I built this because the existing Go Steam libraries are effectively
unmaintained, and I run trading and market tooling against Steam every
day. Steam's unofficial surface shifts constantly — endpoints move,
pages get rewritten, rate limits change — and this library moves with
it, because my own systems break first when it doesn't.

## Install

```sh
go get github.com/k64z/steamstacks
```

Requires Go 1.25+. Three direct dependencies (`protobuf`,
`coder/websocket`, `x/net`).

## What's inside

| Package | What it does |
|---|---|
| [`steamsession`](https://pkg.go.dev/github.com/k64z/steamstacks/steamsession) | Full login flow (RSA + Steam Guard), token refresh, session persistence, authenticated `http.Client` |
| [`steamtotp`](https://pkg.go.dev/github.com/k64z/steamstacks/steamtotp) | Steam Guard TOTP codes, confirmation keys, device IDs |
| [`steamid`](https://pkg.go.dev/github.com/k64z/steamstacks/steamid) | SteamID64 / Steam2 / Steam3 / account-ID conversions |
| [`steamcommunity`](https://pkg.go.dev/github.com/k64z/steamstacks/steamcommunity) | Inventories, trade offers, mobile confirmations, friends, profiles, Community Market (buy/sell/order book/wallet) |
| [`steamstore`](https://pkg.go.dev/github.com/k64z/steamstacks/steamstore) | Wallet, wallet codes, in-game purchases, licenses, gifting, phone management |
| [`steamapi`](https://pkg.go.dev/github.com/k64z/steamstacks/steamapi) | Typed Web API client: trade offers, asset class info, player summaries, auth/2FA service methods |
| [`steamclient`](https://pkg.go.dev/github.com/k64z/steamstacks/steamclient) | The CM protocol over WebSocket/TCP: presence, friend messages, server pushes, Game Coordinator routing |
| [`tf2`](https://pkg.go.dev/github.com/k64z/steamstacks/tf2) | TF2 Game Coordinator: backpack via SO cache sync, crafting, item operations |

## Quickstart

Log in with a Steam Guard code and fetch your TF2 backpack:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/k64z/steamstacks/steamcommunity"
	"github.com/k64z/steamstacks/steamsession"
	"github.com/k64z/steamstacks/steamtotp"
)

func main() {
	ctx := context.Background()

	session, _ := steamsession.New()
	code, _ := steamtotp.GenerateAuthCode(os.Getenv("STEAM_SHARED_SECRET"), 0)
	if err := session.LoginWithDeviceCode(ctx,
		os.Getenv("STEAM_USERNAME"), os.Getenv("STEAM_PASSWORD"), code); err != nil {
		log.Fatal(err)
	}
	if err := session.GetWebCookies(ctx); err != nil {
		log.Fatal(err)
	}

	community, _ := steamcommunity.New(
		steamcommunity.WithHTTPClient(session.HTTPClient()),
	)
	items, err := community.GetOwnInventory(ctx, 440, "2")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%d items\n", len(items))
}
```

Sessions persist to disk (`SaveToFile`/`LoadFromFile`), so long-running
programs log in once and reuse tokens across restarts. See
[`examples/`](examples/) for complete programs: login, inventory, trade
offers with mobile confirmation, a live CM client, and a TF2 GC session.

## Design notes

- **Context everywhere.** Every network call takes a `context.Context`.
- **Functional options.** Every client is constructed with
  `New(WithHTTPClient(...), ...)`.
- **Typed failures.** Steam's undocumented error modes are mapped to
  sentinel errors (`ErrRateLimited`, `ErrMarketPendingConfirmation`, …)
  you can match with `errors.Is`.
- **Cheap endpoints preferred.** Where Steam offers several routes to the
  same data, the library uses the least rate-limited one, and the doc
  comments record why — e.g. wallet balance comes from a ~200-byte store
  JSON endpoint instead of the heavily rate-limited `/market/` page.
- **Hermetic tests.** ~7k lines of table-driven tests against recorded
  fixtures; `go test -race ./...` in CI. No test touches the network.
- **Few dependencies.** Three direct, all mainstream.

The commit history doubles as documentation of Steam's undocumented
behavior: when Steam moves an endpoint or changes a payload, the commit
that adapts to it explains what changed and how it was verified.

## Stability

The module is pre-1.0: the API is settling but not frozen, and minor
versions may still contain breaking changes (they're called out in
release notes). It is used in production against real accounts and real
money daily — correctness issues get fixed fast.

## A note on development

This library is developed with heavy use of [Claude
Code](https://claude.com/claude-code) — you'll see co-author trailers in
the history. Every change is reviewed by me, tested against real Steam,
and usually deployed to my own production systems the same day. The AI
writes code; the understanding of how Steam actually behaves — what
rate-limits, what lies in its responses, what breaks at 3am — comes from
running this stack for real.

## Legal

MIT licensed. This project is not affiliated with or endorsed by Valve.
It talks to Steam's unofficial web and client interfaces; use it in
accordance with the [Steam Subscriber
Agreement](https://store.steampowered.com/subscriber_agreement/), and be
sensible with rate limits.
