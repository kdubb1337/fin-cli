# fin

Agent-native CLI for <service>. <!-- TODO(service): replace -->

## Install

```
brew install kdubb1337/tap/fin
# or
go install github.com/kdubb1337/fin-cli/cmd/fin@latest
```

## Getting an API key <!-- TODO(auth): replace with the real flow, or delete this section if the service is unauthenticated -->

1. Sign in to <service> at <URL>.
2. Open **<Settings → API Keys path>**: <link to the exact page>.
3. Create a new token; choose **<the minimum scope/permission this CLI needs>** (note what additional scopes unlock; e.g. read-only blocks mutations).
4. Copy the token (it usually starts with `<prefix>_`) — most services show it **only once**, so save it to your password manager.

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
