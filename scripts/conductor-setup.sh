#!/bin/bash
# conductor-setup.sh — Conductor worktree setup
# Symlinks shared files from the main worktree, installs Go dev tools,
# and downloads module dependencies.
set -eo pipefail

# ── Resolve main worktree ───────────────────────────────────────
MAIN_WORKTREE="${CONDUCTOR_ROOT_PATH:-$(git worktree list --porcelain | head -1 | sed 's/^worktree //')}"

# ── Files to symlink from the main worktree ─────────────────────
# Paths are relative to the repo root. Add entries here as needed.
WORKTREE_LINKS=(
  ".claude/settings.local.json"
)

echo "[conductor-setup] Linking files from $MAIN_WORKTREE..."
for filepath in "${WORKTREE_LINKS[@]}"; do
  src="$MAIN_WORKTREE/$filepath"
  [[ -e "$src" ]] || continue

  dest_dir=$(dirname "$filepath")
  [[ "$dest_dir" != "." ]] && mkdir -p "$dest_dir"

  # Skip if symlink already points at the right target
  if [[ -L "$filepath" ]] && [[ "$(readlink "$filepath")" == "$src" ]]; then
    continue
  fi

  rm -rf "$filepath"
  ln -sfn "$src" "$filepath"
  echo "  [link] $filepath -> $src"
done

# ── Install Go dev tools ────────────────────────────────────────
echo "[conductor-setup] Installing Go dev tools (gofumpt, goimports, golangci-lint)..."
make tools

# ── Download module dependencies ────────────────────────────────
echo "[conductor-setup] Downloading Go modules..."
go mod download

echo "[conductor-setup] Setup complete."
