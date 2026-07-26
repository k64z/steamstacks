# Security Policy

This library handles Steam credentials, refresh/access tokens, and
authenticator secrets (`shared_secret`, `identity_secret`). Bugs in that
handling can cost people accounts and wallet money, so reports are taken
seriously.

## Reporting a vulnerability

Use [GitHub private vulnerability
reporting](https://github.com/k64z/steamstacks/security/advisories/new)
— do not open a public issue for anything exploitable. You'll get an
initial response within a few days.

## Scope notes

- The library never sends credentials or secrets anywhere except
  Steam's own hosts (`steamcommunity.com`, `store.steampowered.com`,
  `api.steampowered.com`, Valve CM servers).
- Session files written by `steamsession.SaveToFile` contain live
  refresh tokens. They are created with mode 0600; treat them like
  passwords and keep them out of source control (`steam_session*.json`
  is gitignored here for that reason).
- Anything that would make the library log, persist, or transmit a
  secret more broadly than the above is a vulnerability — report it.
