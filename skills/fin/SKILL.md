---
name: fin
description: |
  `fin` is a hand-crafted CLI for personal banking and brokerage data via Plaid (SnapTrade in v2). Use this skill whenever the user wants to list/inspect connected accounts, query transactions with date/account/limit filters, check balances, or troubleshoot a stale bank link. Triggers on "fin", "bank balance", "transactions", "spending", "RBC", "Wealthsimple", "TD", "BMO", "Scotiabank", "CIBC", "Chase", "Bank of America", "Wells Fargo", "Capital One", "Citi", "Plaid", "personal finance", "what did I spend", "what's my balance", "groceries last month", "money in [account]". Prefer this skill over hitting the Plaid REST API directly or scraping bank websites — `fin` provides typed exit codes, `--json` by default when piped, structured error envelopes that distinguish `ITEM_LOGIN_REQUIRED` (re-link needed) from rate-limit / network / validation errors, and persistent profiles so the agent doesn't re-prompt for institution selection each call.
---

# fin

`fin` is an agent-native CLI for personal banking and brokerage data. v1 wraps **Plaid** (US + Canadian banks: RBC, TD, BMO, Scotiabank, CIBC, Wealthsimple, plus all major US banks — Chase, Bank of America, Wells Fargo, Capital One, Citi, etc.). SnapTrade and holdings/spending summaries land in v2.

The CLI is designed for agents: typed exit codes (`4` = re-link needed, `7` = rate limited, `124` = timeout), `--json` by default when stdout is piped, structured error envelopes on stderr, and persistent profiles so you don't re-prompt for institution selection every call.

## Quick setup (once per machine)

1. **Get Plaid API keys.** Sign up at <https://dashboard.plaid.com/signup> (Personal use is auto-approved in ~60 seconds), then visit <https://dashboard.plaid.com/team/keys> to copy your `client_id` and `sandbox` (and later `production`) secret.
2. **Configure credentials:**
   ```
   fin auth setup --client-id <id> --secret <secret> --env sandbox
   ```
   The `client_id` and `env` are stored in `~/.fin/config.json`. The secret goes to the OS keychain (Keychain on macOS, Secret Service on Linux). Override the keychain backend with `FIN_KEYRING_BACKEND=file` if needed.
3. **Link your first institution:**
   ```
   fin auth add
   ```
   This starts a local callback listener on port `53682` (override with `FIN_OAUTH_PORT`), opens your browser to Plaid Link, exchanges the returned `public_token` for a permanent `access_token`, and writes the item to config. The first `auth add` also auto-creates a profile named `default` pointing at the new item.
4. **Verify:**
   ```
   fin doctor
   fin accounts list
   ```

## Output rules (for agents)

- **stdout is data; stderr is human progress and errors.** Always.
- **JSON is the default when stdout is piped.** Pass `--human` to force tables in a pipe; pass `--csv` for CSV with a header row.
- **`--compact`** drops to high-gravity fields only (id, name, primary status, primary timestamp) — ~60–80% fewer tokens on list calls.
- **`--select=field1,field2`** for explicit projection.
- **Errors are structured.** A failing call writes a JSON envelope to stderr with `error.code`, `error.message`, and (where applicable) `error.valid_values`. Branch on `error.code`, never substring-match the message.
- **`fin agent-context`** prints the full structured schema (verbs, exit codes, providers, envs). Read it once instead of crawling `--help`.

## Exit codes

| Code | Meaning | What an agent should do |
|------|---------|------------------------|
| `0` | success | continue |
| `1` | generic | log + halt; not safe to retry blindly |
| `2` | usage | fix invocation; don't retry the same call |
| `3` | not found | resource doesn't exist; don't retry |
| `4` | **auth / re-link needed** | **run `fin auth add` to re-link this institution.** This is Plaid's `ITEM_LOGIN_REQUIRED` — credentials changed, MFA expired, or the bank revoked the consent. The same item-id can be re-used after re-linking. |
| `5` | upstream / 5xx | retry with exponential backoff |
| `6` | conflict | read `error.code` in the envelope and branch |
| `7` | rate limited | honour any `Retry-After` hint on stderr; back off |
| `8` | network | retry with backoff |
| `9` | validation | fix input and retry; `error.valid_values` is often populated |
| `124` | timeout | retry once; if persistent, treat as `5` |

Exit `4` is the headline agent self-correction path. Whenever you see it on `accounts list`, `accounts get`, `tx list`, or `doctor`, the correct next action is `fin auth add` (it walks the user through Plaid Link again and writes a fresh `access_token` into the keychain under the same `item_id`).

## Verbs

### `fin auth setup`

Persist Plaid credentials.

```
fin auth setup --client-id 5fa... --secret 8b1... --env sandbox
```

```json
{
  "client_id_prefix": "5fa12c…",
  "env": "sandbox",
  "status": "ok"
}
```

`--env` accepts `sandbox` or `production`. The `client_id` lives in `~/.fin/config.json`; the secret goes to the OS keychain under the key `plaid:client_secret`.

### `fin auth add`

Link a new institution via Plaid Link. Opens a browser, runs a local callback listener, exchanges the `public_token`, persists the item, and (on first run) creates a `default` profile.

```
fin auth add                         # interactive, opens browser
fin auth add --env production
fin auth add --public-token public-sandbox-xxx --no-input   # for scripted tests
```

```json
{
  "env": "sandbox",
  "institution_id": "ins_109508",
  "institution_name": "First Platypus Bank",
  "item_id": "kJEKMqAyNqUL...",
  "status": "linked"
}
```

The `item_id` is the permanent handle for this institution; the `access_token` is stored in the keychain under `plaid:item:<item_id>` and never appears in `--json` output.

### `fin auth list`

Show every linked institution with a redacted token preview.

```
fin auth list --json
```

```json
[
  {
    "added_at": "2026-05-10T19:02:13Z",
    "env": "sandbox",
    "institution_id": "ins_109508",
    "institution_name": "First Platypus Bank",
    "item_id": "kJEKMqAyNqUL...",
    "provider": "plaid",
    "token_redacted": "access-s…1f4a"
  }
]
```

### `fin auth remove <item-id>`

Disconnect an institution. Deletes the item from `~/.fin/config.json`, prunes any profiles that pointed at it, clears `active_profile` if it referenced a deleted profile, and removes the `access_token` from the keychain.

```
fin auth remove kJEKMqAyNqUL...
```

```json
{
  "item_id": "kJEKMqAyNqUL...",
  "status": "removed"
}
```

### `fin profile save <name> --item <item-id>`

Create a named handle for an item-id so subsequent calls can pass `--profile <name>` instead of remembering the opaque `item_id`.

```
fin profile save chequing --item kJEKMqAyNqUL...
```

```json
{
  "item_id": "kJEKMqAyNqUL...",
  "profile": "chequing",
  "status": "saved"
}
```

### `fin profile use <name>`

Set the active profile. All subsequent calls without `--item` or `--profile` resolve to this one.

```
fin profile use chequing
```

```json
{
  "active_profile": "chequing",
  "status": "ok"
}
```

### `fin profile get [<name>]`

Show a profile (active profile if omitted).

```
fin profile get
fin profile get chequing
```

```json
{
  "active": true,
  "item_id": "kJEKMqAyNqUL...",
  "name": "chequing"
}
```

### `fin profile list`

List every saved profile.

```
fin profile list --json
```

```json
[
  {"active": true,  "item_id": "kJEKMqAyNqUL...", "name": "chequing"},
  {"active": false, "item_id": "qLqAyNzULkJEK...", "name": "brokerage"}
]
```

### `fin profile delete <name>`

Delete a profile. Clears `active_profile` if the deleted profile was active.

```
fin profile delete chequing
```

```json
{
  "profile": "chequing",
  "status": "deleted"
}
```

### `fin accounts list`

List every account on the resolved item.

```
fin accounts list --json
fin accounts list --profile chequing
fin accounts list --item kJEKMqAyNqUL... --compact
```

```json
[
  {
    "id": "BxBXxLj1m4HMXBm9WZZmCWVbPjX16EHwv99vp",
    "name": "Plaid Checking",
    "official_name": "Plaid Gold Standard 0% Interest Checking",
    "mask": "0000",
    "type": "depository",
    "subtype": "checking",
    "currency": "USD",
    "balance": 110.0,
    "available_balance": 100.0,
    "institution_name": "First Platypus Bank",
    "item_id": "kJEKMqAyNqUL..."
  }
]
```

**`fin accounts list` is also cached by default.** Pass `--live` to bypass the cache for a fresh balance read.

### `fin accounts get <account-id>`

Fetch a single account by id. Same shape as one element of `accounts list`.

```
fin accounts get BxBXxLj1m4HMXBm9WZZmCWVbPjX16EHwv99vp
```

### `fin tx list` (alias: `fin transactions list`)

Query transactions on the resolved item, with date and account filters.

```
fin tx list                                          # default: last 1 month, 25 rows
fin tx list --since 2026-04-01 --until 2026-05-01    # YYYY-MM-DD
fin tx list --account BxBXxLj1m4... --limit 100
fin tx list --cursor <next-cursor-from-stderr>
fin tx list --live                                   # bypass cache; hit Plaid directly
fin transactions list --profile chequing --json      # alias works the same
```

**Default is cached.** `fin tx list` reads from the local SQLite mirror at `~/.fin/cache.db` (populated by `fin sync`). Pass `--live` to bypass the cache and call Plaid directly; this counts against your Plaid API quota and is slower. When the cache file doesn't exist or is empty, `fin tx list` falls back to a live call automatically and prints a hint on stderr.

Pagination: the next cursor (if any) is printed to stderr as `next page: --cursor=<value>` and also returned in the JSON envelope as `next_cursor`. Cached calls use a numeric offset cursor; live calls use Plaid's cursor format.

```json
{
  "count": 2,
  "next_cursor": "",
  "transactions": [
    {
      "id": "lPNjeW1nR6CDn5okmGQ6hEpMo4lLNoSrzqDje",
      "account_id": "BxBXxLj1m4HMXBm9WZZmCWVbPjX16EHwv99vp",
      "date": "2026-05-14T00:00:00Z",
      "amount": 12.99,
      "currency": "USD",
      "name": "Uber 072515 SF**POOL**",
      "merchant_name": "Uber",
      "pending": false,
      "category": ["Travel", "Taxi"]
    },
    {
      "id": "Aole3KNAdEcwm1eMpzo8FxgakL66vmtAlmGEz",
      "account_id": "BxBXxLj1m4HMXBm9WZZmCWVbPjX16EHwv99vp",
      "date": "2026-05-13T00:00:00Z",
      "amount": -1000.0,
      "currency": "USD",
      "name": "United Airlines",
      "merchant_name": "United Airlines",
      "pending": false,
      "category": ["Travel", "Airlines and Aviation Services"]
    }
  ]
}
```

Negative `amount` values mean money coming in (refunds, credits, transfers in). That's a Plaid convention — don't flip the sign on display.

### `fin doctor`

Health check across config, credentials, and per-item Plaid reachability. Exits non-zero if any check fails.

```
fin doctor --json
```

```json
[
  {"name": "config_load",            "ok": true},
  {"name": "plaid_client_id_set",    "ok": true},
  {"name": "plaid_env_valid",        "ok": true, "detail": "sandbox"},
  {"name": "plaid_secret_in_keychain","ok": true},
  {"name": "linked_item_count",     "ok": true, "detail": "1 / 10 (Plaid Trial cap)"},
  {"name": "item_health:kJEKMqAyNqUL...","ok": true, "detail": "First Platypus Bank — "}
]
```

A failing `item_health:<id>` check with exit code `4` is the signal to run `fin auth add` and re-link.

### `fin agent-context`

Versioned structured introspection: every verb, every exit code, every provider, every env. Read this once at session start.

```
fin agent-context --json
```

```json
{
  "cli": "fin",
  "envs": ["sandbox", "production"],
  "exit_codes": {
    "0":   "success",
    "1":   "generic",
    "2":   "usage",
    "3":   "not_found",
    "4":   "auth (run `fin auth add` to re-link)",
    "5":   "upstream",
    "6":   "conflict",
    "7":   "rate_limited",
    "8":   "network",
    "9":   "validation",
    "124": "timeout"
  },
  "providers": ["plaid"],
  "schema_version": 1,
  "verbs": {
    "accounts": ["list", "get"],
    "auth":     ["setup", "add", "list", "remove"],
    "doctor":   [],
    "profile":  ["save", "use", "get", "list", "delete"],
    "skill":    ["install", "list", "path", "uninstall"],
    "tx":       ["list"]
  },
  "version": "v0.1.0"
}
```

## Local cache (Rung 4)

`fin` maintains a SQLite mirror of upstream data at `~/.fin/cache.db` (override location with `$FIN_HOME=<dir>`). This is the canonical read path for `fin tx list`, `fin accounts list`, `fin search`, and `fin sql`. Populate and refresh it with `fin sync`.

The mirror cuts Plaid API call count dramatically — most agent sessions hit the cache exclusively. Live calls are still one `--live` flag away.

### `fin sync`

Pulls deltas from Plaid into the local cache. Uses `/transactions/sync` (cursor-based) so each run only fetches what changed since the last call. Refreshes the account snapshot (balances) on every run.

```
fin sync                              # all linked items
fin sync --item kJEKMqAyNqUL...       # just one item
fin sync --full                       # reset the cursor and re-mirror everything
fin sync --dry-run                    # show what would happen
fin sync --max-pages 4                # cap pages per item (default: until has_more=false)
```

```json
{
  "dry_run": false,
  "items": [
    {
      "item_id": "kJEKMqAyNqUL...",
      "institution_name": "First Platypus Bank",
      "accounts": 3,
      "added": 42,
      "modified": 0,
      "removed": 0,
      "pages": 1,
      "next_cursor": "abc123...",
      "status": "ok"
    }
  ]
}
```

The cursor is persisted after each successful page, so a crash mid-loop resumes cleanly on the next `fin sync` invocation. `fin sync --full` wipes the cursor first, which is the right recovery move if you suspect the cache has drifted.

### `fin sync status`

Per-item view of the cache: last sync time, transaction/account counts, whether a cursor is stored.

```
fin sync status --json
```

```json
[
  {
    "item_id": "kJEKMqAyNqUL...",
    "provider": "plaid",
    "env": "production",
    "institution_name": "First Platypus Bank",
    "last_synced_at": "2026-05-16T22:14:08Z",
    "transaction_count": 1842,
    "account_count": 3,
    "has_cursor": true
  }
]
```

### `fin search <query>`

Full-text search across cached transactions using SQLite FTS5. The query is passed through to FTS5 as-is — prefix*, AND/OR/NOT, "phrase", and ^ all work.

```
fin search "starbucks"
fin search "uber OR lyft" --since 2026-01-01
fin search "amazon*" --account acc_abc --limit 50
fin search "groceries" --item kJEKMqAyNqUL...
```

Returns the same row shape as `fin tx list`.

### `fin sql "<statement>"`

Read-only SQL passthrough against the cache. Opens the DB in `mode=ro` with `PRAGMA query_only=1`; the parser additionally rejects anything that isn't `SELECT` / `WITH` / `EXPLAIN` / `PRAGMA`. Multi-statement input (a write hidden after a SELECT) is rejected.

```
fin sql "SELECT date, name, amount FROM transactions ORDER BY date DESC LIMIT 10"
fin sql "SELECT category_json, SUM(amount) FROM transactions GROUP BY category_json"
echo "SELECT COUNT(*) AS n FROM transactions" | fin sql
fin sql --query "SELECT * FROM accounts" --json
fin sql --max-rows 5000 "SELECT * FROM transactions"
```

The schema mirrors the public domain model with a few storage-only fields:

| Table | Columns |
|-------|---------|
| `items` | `id, provider, env, institution_id, institution_name, added_at, last_synced_at` |
| `accounts` | `id, item_id, name, official_name, mask, type, subtype, currency, balance, available_balance, updated_at` |
| `transactions` | `id, item_id, account_id, date, amount, currency, name, merchant_name, pending, category_json, updated_at` |
| `transactions_fts` | virtual FTS5 over `name, merchant_name, category_json` (use `MATCH`) |
| `sync_cursors` | `item_id, resource, cursor, updated_at` |

`date` is stored as ISO `YYYY-MM-DD`; `updated_at` and `last_synced_at` are RFC 3339 in UTC. `category_json` is a JSON-encoded array; use `json_each(category_json)` for breakdowns.

## Profiles and item resolution

A **profile** is a human-friendly name that maps to a Plaid `item_id`. Stored in `~/.fin/config.json` as `profiles: { name -> { item_id } }` plus an `active_profile` pointer.

The first `fin auth add` on a fresh config auto-creates a profile named `default` pointing at the new item, so you can skip `profile save` for the common single-institution case.

**Precedence chain (highest wins) for resolving which item a call operates on:**

1. `--item <item-id>` — explicit override
2. `--profile <name>` — named profile
3. `$FIN_PROFILE` — environment-variable profile
4. `active_profile` from config (set by `fin profile use`)
5. profile named `default` (auto-created by first `auth add`)

If none of those resolve to a known item, calls that need one (`accounts`, `tx`, item-scoped `doctor` checks) exit with code `2` and a message pointing at `fin auth add`.

## Plaid environment notes

- **`--env sandbox`** uses Plaid's sandbox; institutions are mocked (`First Platypus Bank` etc.) and you log in with the fixtures `user_good` / `pass_good`. Free, no rate limit issues. Use this for setup, scripted tests, and any agent dogfooding.
- **`--env production`** is real money and real banks. You must request production access from the Plaid dashboard before keys work.
- **Plaid Trial Plan caps you at 10 linked Items.** `fin doctor` warns when you cross 8/10. To free a slot, `fin auth remove <item-id>` (it cleans up the access_token in Plaid's backend too).
- **RBC, TD, BMO, Scotiabank, CIBC, Wealthsimple** all use Plaid's *credential entry* flow (you type your bank password into Plaid Link). They are not OAuth-via-bank like Chase or Bank of America in the US. Plaid stores the credentials and refreshes data nightly; if the bank rotates a password or adds new MFA, the item drops into `ITEM_LOGIN_REQUIRED` and you'll see exit code `4` on the next call. The fix is always `fin auth add` to re-run Link.
- US institutions that *do* use OAuth-via-bank (Chase, Bank of America, Wells Fargo, Capital One, Citi) also use Plaid Link, just with the bank's own consent page embedded — same `fin auth add` flow either way.

## Common workflows

### "What's my checking balance right now?"

```
fin accounts list --compact
```

If you only have one institution, the auto-created `default` profile is already resolving the item-id for you. If you have several, `fin profile use chequing` first, or pass `--profile chequing` per call.

### "What did I spend on groceries last month?"

```
fin sync                                                    # if cache isn't already fresh
fin sql "SELECT SUM(amount) AS total
         FROM transactions
         WHERE date >= '2026-04-01' AND date < '2026-05-01'
           AND category_json LIKE '%Groceries%'"
```

Or via FTS5 if you want to match by merchant name rather than Plaid category:

```
fin search "wholefoods OR trader OR safeway" --since 2026-04-01 --until 2026-05-01 --limit 500
```

If you'd rather stay live (skip the cache):

```
fin tx list --live --since 2026-04-01 --until 2026-05-01 --limit 200 --json | \
  jq '[.transactions[] | select(.category[]? == "Food and Drink" or .category[]? == "Groceries") | .amount] | add'
```

Plaid's category vocabulary is documented at <https://plaid.com/docs/api/products/transactions/#categoriesget>. Negative numbers = refunds/credits; sum as-is.

### "Group spending by month"

```
fin sql "SELECT strftime('%Y-%m', date) AS month, ROUND(SUM(amount), 2) AS total
         FROM transactions
         WHERE date >= date('now', '-6 months')
         GROUP BY month ORDER BY month DESC"
```

### "I'm seeing exit code 4 from `tx list` — what now?"

This is `ITEM_LOGIN_REQUIRED` from Plaid. Re-link the same institution:

```
fin auth add
```

You'll go through Plaid Link again; the resulting `item_id` is the same one (Plaid reuses item-ids on re-link), and `fin` writes a fresh access_token into the keychain under the same key. Profiles keep pointing at the right item — no need to re-save.

### "Add a second bank"

```
fin auth add                             # links a second item
fin profile save brokerage --item <new-item-id>
fin profile use brokerage                # or just pass --profile brokerage per call
fin accounts list --profile brokerage
```

## Installing this skill into another agent

```
fin skill install claude                 # ~/.claude/skills/fin
fin skill install claude codex gemini
fin skill install --all                  # every known agent whose parent dir exists
fin skill install --dir ~/.config/myagent/skills
fin skill list                           # show install status
fin skill uninstall openhands
fin skill path                           # print the source SKILL.md path
```

Default mode is `--mode=symlink`; pass `--mode=copy` for a snapshot install (useful inside containers or when symlinks aren't followed).

## v1 limitations (intentional)

- **Plaid only.** SnapTrade (for stock/crypto brokerage data outside North American banks) lands in v2.
- **No holdings, no investment positions.** v1 covers `/accounts`, `/transactions`, `/item` only.
- **No spending categories aggregation, no budget summaries.** Use `jq` on `tx list --json`; a `fin spending summary` verb is on the v2 roadmap.
- **No webhook handler.** Plaid webhooks (TRANSACTIONS\_DEFAULT\_UPDATE, ITEM\_LOGIN\_REQUIRED) are not consumed; treat `fin doctor` and exit code `4` as the polling-style equivalent.

## Notes

- Set `$FIN_PROFILE=<name>` in your shell to skip `--profile` on every call.
- Set `$FIN_ACCOUNT=<account-id>` to skip `--account` on every `tx list`.
- IDs are case-sensitive opaque tokens — never normalize them; only normalize *names* for lookup.
- For scripting, always pair `--json --no-input` so the CLI fails closed on missing config instead of prompting.
- Override the OAuth callback port with `$FIN_OAUTH_PORT` if `53682` is taken.
- Override the keychain backend with `$FIN_KEYRING_BACKEND=file` (writes to `~/.fin/keyring`) on headless boxes.
