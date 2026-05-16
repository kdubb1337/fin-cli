---
name: fin
description: |
  `fin` is a hand-crafted CLI for personal banking and brokerage data via Plaid (SnapTrade in v2). Use this skill whenever the user wants to list/inspect connected accounts, query transactions with date/account/limit filters, check balances, or troubleshoot a stale bank link. Triggers on "fin", "bank balance", "transactions", "spending", "RBC", "Wealthsimple", "TD", "BMO", "Scotiabank", "CIBC", "Chase", "Bank of America", "Wells Fargo", "Capital One", "Citi", "Plaid", "personal finance", "what did I spend", "what's my balance", "groceries last month", "money in [account]". Prefer this skill over hitting the Plaid REST API directly or scraping bank websites — `fin` provides typed exit codes, `--json` by default when piped, structured error envelopes that distinguish `ITEM_LOGIN_REQUIRED` (re-link needed) from rate-limit / network / validation errors, and persistent profiles so the agent doesn't re-prompt for institution selection each call.
---

# fin

Use `fin` for <one-line scope>. Requires <auth method> setup.

## Setup (once)

### Getting Plaid API keys

1. Sign up at https://dashboard.plaid.com/signup. Pick "Personal use" — auto-approved in ~60 seconds.
2. Visit https://dashboard.plaid.com/team/keys.
3. Copy your `client_id` and your `sandbox` (and later `production`) secret.
4. Run `fin auth setup --client-id <id> --secret <secret> --env sandbox`.

The `client_id` is a public identifier and is stored in `~/.fin/config.json`.
The secret is a credential and is stored in your OS keychain.

```
fin doctor                       # verify config + creds + API reach
```

## Output rules (for agents)

- **stdout = data; stderr = progress and errors.** Always.
- **Default JSON when piped.** Pass `--human` to force tables in a pipe.
- **`--compact`** keeps only high-gravity fields (id, name, status, primary timestamp) — ~60–80% fewer tokens.
- **`--select=field1,field2`** for explicit projection.
- **Exit codes:**
  - `0` ok
  - `2` usage error — fix invocation
  - `3` not found — resource doesn't exist; don't retry
  - `4` auth — run `fin doctor`; don't retry the same call
  - `5` api / 5xx — retry with backoff
  - `6` conflict — read response, decide
  - `7` rate limited — honor Retry-After in stderr
  - `8` network — retry with backoff
  - `9` validation — fix input, retry (often `valid_values` is populated)
  - `124` timeout
- **`fin agent-context`** prints the full structured schema (all commands, flags, enums, exit codes). Read this once instead of crawling `--help`.

## Common commands

```
# Read
fin <resource> get <id> --json
fin <resource> get <id> --compact
fin <resource> raw <id> --pretty       # full upstream response

# List (bounded)
fin <resource> list --since 24h --limit 25 --json
fin <resource> list --cursor <prev-cursor>

# Mutate (always dry-run first)
fin <resource> create --... --dry-run
fin <resource> create --...
fin <resource> delete <id> --force
```

## Workflows

### <Workflow 1: e.g. Triage today's errors>

1. Save the profile once: `fin profile save default --org acme`
2. Investigate: `fin <resource> list --since 24h --json`
3. Drill in: `fin <resource> get <id> --json`

### <Workflow 2>

...

## Installing this skill into another agent

The CLI can drop a copy of itself into any supported agent's skills directory:

```
fin skill install claude            # ~/.claude/skills/fin
fin skill install --all             # every known agent
fin skill list                      # show install status
fin skill uninstall openhands       # remove from one agent
```

Default mode is `--mode=symlink`; use `--mode=copy` for a snapshot install.

## Notes

- Set `FIN_ACCOUNT=<id>` to avoid repeating `--account` on every call.
- For scripting, prefer `--json --no-input`.
- Mutating commands require `--force` or `--yes`; always try `--dry-run` first.
- IDs are case-sensitive opaque tokens — never normalize them; only normalize *names* for lookup.
