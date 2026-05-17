# Changelog

## v0.3.0 — 2026-05-16

### Fixed

- `fin auth add` now actually works. The previous implementation opened `https://link.plaid.com/?token=…`, which is not a real Plaid endpoint — Plaid Link is JS-SDK only and Hosted Link is gated behind a Plaid sales contact. The local callback listener now serves an HTML shell that loads `link-initialize.js` and drives `Plaid.create({…}).open()` directly, forwarding `public_token` back to `/callback` on the same listener. Works for sandbox + non-OAuth institutions on a Plaid free trial.
- Redirect URI sent to `link_token/create` now uses `localhost` rather than `127.0.0.1`. Plaid's allowed-redirect-URIs allowlist is byte-exact and only accepts `localhost` as the loopback host.
- `fin tx list --csv` was emitting the wrapper object (`count,next_cursor,transactions` header with the transaction list stringified into one cell) instead of one CSV row per transaction. Switched to `output.EmitPage`, which is the existing helper designed for this.

### Added

- `--human` is now a real plain-text table renderer (was a stub returning pretty JSON). Stable column order via a priority list; missing optional fields render as blank cells. No new dependencies.
- `--csv` column order is now stable and uses the same priority list as `--human`. Previously non-deterministic and could drop columns when the first row lacked optional fields.
- `fin auth add` success page auto-closes the browser tab after a 5-second countdown, with a "Close now" link as fallback for browsers that block scripted `window.close()`.

## v0.2.0 — 2026-05-16

- Repository made public.
- License: replaced placeholder with real MIT text.
- README: rewrote Quick start around the actual `fin auth setup` → `fin auth add` flow (the old draft referenced a non-existent `FIN_API_KEY` / `fin auth add <token>` path).
- Removed leftover `fin hello` sample command from the initial scaffold.

## v0.1.0 — 2026-05-16

Initial release.

- `fin auth setup` / `add` / `list` / `remove` — Plaid credential + Link flow with local OAuth callback listener.
- `fin profile save` / `use` / `get` / `list` / `delete` — named handles over Plaid `item_id`s, with an auto-created `default` profile on first link.
- `fin accounts list` / `get` — list and fetch accounts on the resolved item.
- `fin tx list` (alias `transactions list`) — date- and account-filtered transaction queries with cursor pagination.
- `fin doctor` — config + credential + per-item Plaid reachability health checks.
- `fin agent-context` — versioned structured introspection (verbs, exit codes, providers, envs).
- `fin skill install` / `list` / `path` / `uninstall` — manage the bundled SKILL.md across Claude / Codex / Gemini / OpenHands / `agents`.
- Typed exit codes (`0`/`2`/`3`/`4`/`5`/`6`/`7`/`8`/`9`/`124`) with `4` reserved for `ITEM_LOGIN_REQUIRED` (re-link signal).
- OS keychain storage for Plaid secret and per-item access tokens; `FIN_KEYRING_BACKEND=file` fallback for headless environments.
