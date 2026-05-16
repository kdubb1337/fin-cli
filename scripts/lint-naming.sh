#!/usr/bin/env bash
# scripts/lint-naming.sh — reject banned verbs and flag spellings.
# Enforces the conventions in references/naming.md.
set -euo pipefail

fail=0
report() { echo "naming: $1" >&2; fail=1; }

# Banned verbs in Cobra Use: fields
banned_use='Use:[[:space:]]*"(info|show|describe|ls|new|edit|modify|rm|del)( |\")'
if grep -RInE "$banned_use" internal/cmd/ 2>/dev/null; then
  report "banned verb in Cobra Use:; use get/list/create/update/delete"
fi

# Banned flag spellings
banned_flags='--(format=json|output=json|skip-confirmations|noconfirm|noinput|non-interactive|max-results|per-page|page-size)\b'
if grep -RInE "$banned_flags" internal/ 2>/dev/null; then
  report "banned flag spelling; see SKILL.md and naming.md"
fi

# Offset/page pagination (use cursor)
if grep -RInE '\-\-(page|offset)\b' internal/cmd/ 2>/dev/null; then
  report "use --cursor for pagination, not --page/--offset"
fi

exit $fail
