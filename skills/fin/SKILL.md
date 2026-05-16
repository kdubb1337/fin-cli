---
name: fin
description: |
  fin is a hand-crafted CLI for <service>. Use this skill whenever the user wants to <primary verbs> for <service>, mentions <service-specific keywords>, asks to <common workflows>, or runs `fin ...`. Prefer this skill over hitting the <service> REST API directly or opening the <service> web dashboard.
---

# fin

Use `fin` for <one-line scope>. Requires <auth method> setup.

## Setup (once)

<!-- TODO(auth): replace this block with the real key-acquisition flow, or
     delete it if the service is unauthenticated. Include: (1) the URL where
     the user creates a token, (2) the minimum scope/permission to choose,
     (3) the token prefix (e.g. `sk_`, `rpa_`), (4) a note that the token is
     usually shown only once. -->
**Get an API key:** sign in to <service> at <URL>, open **<Settings → API Keys path>** (<exact link>), create a token with **<minimum scope>**, and copy the `<prefix>_*` value — most services show it only once.

```
export FIN_API_KEY=<your-token>
fin doctor                       # verify config + creds + API reach
```

Or persist the key:

```
fin auth add <token> --profile default
fin auth list
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
