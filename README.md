# fin

Agent-native CLI for Plaid / personal banking data.

## Install

```
brew install kdubb1337/tap/fin
# or
go install github.com/kdubb1337/fin-cli/cmd/fin@latest
```

## Getting Plaid API keys

1. Sign up at https://dashboard.plaid.com/signup. Pick "Personal use" — auto-approved in ~60 seconds.
2. Visit https://dashboard.plaid.com/team/keys.
3. Copy your `client_id` and your `sandbox` (and later `production`) secret.
4. Run `fin auth setup --client-id <id> --secret <secret> --env sandbox`.

The `client_id` is a public identifier and is stored in `~/.fin/config.json`.
The secret is a credential and is stored in your OS keychain.

## Quick start

```
export FIN_API_KEY=<your-token>
fin doctor
fin <resource> list --json
```

Or persist the key as a profile:

```
fin auth add <token> --profile default
fin profile list
```

## Output rules

- stdout = data, stderr = humans
- Auto-JSON when piped; `--human` forces tables in a pipe
- `--compact` for high-gravity fields only; `--select` for explicit projection
- Exit codes: `0` ok, `2` usage, `3` not-found, `4` auth, `5` api, `6` conflict, `7` rate-limit, `8` network, `9` validation, `124` timeout

See `fin agent-context` for the full schema.

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
