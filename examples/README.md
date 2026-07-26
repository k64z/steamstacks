# Examples

Each directory is a self-contained program. All of them authenticate with
environment variables — nothing is ever hardcoded:

| Variable | Used by | What it is |
|---|---|---|
| `STEAM_USERNAME` | all | account name |
| `STEAM_PASSWORD` | all | account password |
| `STEAM_SHARED_SECRET` | all | base64 `shared_secret` from your mobile authenticator |
| `STEAM_IDENTITY_SECRET` | tradeoffer | base64 `identity_secret`, for mobile confirmations |
| `TRADE_PARTNER`, `TRADE_TOKEN`, `TRADE_ASSET_ID` | tradeoffer | who and what to trade |

Run them from their directory, e.g.:

```sh
cd examples/login
STEAM_USERNAME=... STEAM_PASSWORD=... STEAM_SHARED_SECRET=... go run .
```

| Example | Shows |
|---|---|
| [login](login/) | credentials + TOTP login, session persistence |
| [inventory](inventory/) | fetching an inventory over the web API |
| [tradeoffer](tradeoffer/) | sending a trade offer + mobile confirmation |
| [cmclient](cmclient/) | the CM (desktop-client) protocol: presence, friend messages, wallet pushes |
| [tf2](tf2/) | TF2 Game Coordinator: backpack via the shared-object cache |

Sessions are cached in `steam_session_web.json` / `steam_session_client.json`
(gitignored). The web and client sessions are separate because Steam issues
platform-specific tokens.

A note on secrets: `shared_secret` and `identity_secret` come from your Steam
Guard mobile authenticator (e.g. an exported maFile). Treat them like
passwords — anyone holding them can generate your 2FA codes and approve
trades.
