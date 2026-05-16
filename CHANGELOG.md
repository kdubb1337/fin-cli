# Changelog

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
