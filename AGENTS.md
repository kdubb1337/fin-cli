# Repository Guidelines

## Project Structure

- `cmd/fin/`: CLI entrypoint.
- `internal/`: implementation (`cmd/`, API client, OAuth, config/secrets, output/UI).
- Tests: `*_test.go` next to code; opt-in integration suite in `internal/integration/` (build-tagged).
- `bin/`: build outputs. `docs/`: design + releasing. `scripts/`: release + lint helpers.
- `skills/fin/SKILL.md`: bundled skill for downstream agents using the CLI.

## Build, Test, and Development Commands

- `make` / `make build`: build `bin/fin`.
- `make tools`: install pinned dev tools into `.tools/`.
- `make fmt` / `make lint` / `make test` / `make ci`: format, lint, test, full local gate.
- Hooks: `git config core.hooksPath hooks` enables pre-commit/pre-push checks.

## Coding Style & Naming Conventions

- Formatting: `make fmt` (`goimports` local prefix `github.com/kdubb1337/fin-cli` + `gofumpt`).
- Output: keep stdout parseable (`--json` / auto-JSON when piped); send human hints/progress to stderr.
- Treat external IDs as case-sensitive opaque tokens; only case-fold names for name lookup.
- Verbs: `get` / `list` / `create` / `update` / `delete`. Flags: `--json` / `--force` / `--yes` / `--dry-run` / `--no-input`. `scripts/lint-naming.sh` enforces.

## Testing Guidelines

- Unit tests: stdlib `testing` (and `httptest` where needed).
- Integration tests (local only):
  - `FIN_IT_ACCOUNT=you@example.com go test -tags=integration ./internal/integration`
  - Requires real credentials in your keyring.

## Commit & Pull Request Guidelines

- Create commits with `scripts/committer "<msg>" <file...>`; avoid manual staging.
- Conventional Commits with action-oriented subjects (e.g. `feat(cli): add --wait to issue create`).
- Group related changes; avoid bundling unrelated refactors.
- PRs summarize scope, testing performed, and user-facing changes / new flags.
- PR review flow: when given a PR link, review via `gh pr view` / `gh pr diff` and do not change branches.

### PR Workflow (Review vs Land)

- **Review mode (PR link only):** read `gh pr view/diff`; do not switch branches; do not change code.
- **Landing mode:** temp branch from `main`; bring in PR (squash default); fix; update `CHANGELOG.md`; run `make ci`; merge to `main`; delete temp; end on `main`.
- Always add `Co-authored-by:` trailers for PR authors.

## Security & Configuration Tips

- Never commit OAuth client credential JSON files or tokens.
- Prefer OS keychain backends; use `FIN_KEYRING_BACKEND=file` + `FIN_KEYRING_PASSWORD` only for headless environments.
- `FIN_ACCOUNT=<id>` selects the active account without `--account` on every command.

## Commit Subject Scopes

`cli`, `api`, `auth`, `output`, `store`, `docs`, `ci`, `release`.
