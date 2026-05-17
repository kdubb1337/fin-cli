# fin

Agent-native CLI for personal banking and brokerage data via Plaid.

Designed for AI agents: typed exit codes, `--json` by default when piped,
structured error envelopes on stderr, and persistent profiles so the agent
doesn't re-prompt for institution selection every call.

## Install

```
brew install kdubb1337/tap/fin
# or
go install github.com/kdubb1337/fin-cli/cmd/fin@latest
```

## Get Plaid API keys

1. Sign up at <https://dashboard.plaid.com/signup>. Pick "Personal use" — auto-approved in ~60 seconds.
2. Visit <https://dashboard.plaid.com/team/keys>.
3. Copy your `client_id` and your `sandbox` (and later `production`) secret.

The `client_id` is a public identifier and is stored in `~/.fin/config.json`.
The secret is a credential and is stored in your OS keychain (Keychain on
macOS, Secret Service on Linux).

## Quick start

```
fin auth setup --client-id <id> --secret <secret> --env sandbox
fin auth add                          # opens Plaid Link in your browser
fin doctor
fin sync                              # mirror accounts + transactions into ~/.fin/cache.db
fin accounts list                     # reads cache
fin tx list --since 2026-04-01 --until 2026-05-01
fin sql "SELECT date, name, amount FROM transactions ORDER BY date DESC LIMIT 10"
fin search "starbucks"
```

The first `fin auth add` on a fresh config auto-creates a profile named
`default` pointing at the new item, so single-institution users can skip
`fin profile save`.

## Output rules

- stdout = data, stderr = humans
- Auto-JSON when piped; `--human` forces tables in a pipe
- `--compact` for high-gravity fields only; `--select` for explicit projection
- Exit codes: `0` ok, `2` usage, `3` not-found, `4` auth (re-link needed), `5` upstream, `6` conflict, `7` rate-limit, `8` network, `9` validation, `124` timeout

Exit code `4` is the agent self-correction signal: Plaid's `ITEM_LOGIN_REQUIRED`.
The fix is always `fin auth add` to walk the user through Plaid Link again.

See `fin agent-context` for the full structured schema (verbs, exit codes,
providers, envs).

## For agents

A bundled `SKILL.md` ships with the binary. Install it into your agent of choice:

```
fin skill install claude            # ~/.claude/skills/fin
fin skill install claude codex      # multiple
fin skill install --all             # every known agent
fin skill install --dir ~/custom    # custom path
fin skill install claude --mode=copy --force
```

Known targets: `claude` (`~/.claude/skills`), `codex` (`~/.codex/skills`), `gemini` (`~/.gemini/skills`), `openhands` (`~/.openhands/microagents`), `agents` (`~/.agents/skills`, the cross-agent universal path).

Default mode is `symlink` so edits to the source SKILL.md propagate instantly. Pass `--mode=copy` for a snapshot install. Check status with `fin skill list`; remove with `fin skill uninstall <agent>`.

To find the source path of the bundled skill:

```
fin skill path
```

Or read it directly at `skills/fin/SKILL.md` in this repo.

## Development

```
make tools     # install pinned dev tools
make           # build
make ci        # fmt + lint + test + build
```

See `AGENTS.md` for the full contributor guide.

## License

MIT — see [LICENSE](LICENSE).
