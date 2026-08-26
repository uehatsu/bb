#!/bin/sh
# Generate skills/<agent>/bitbucket/SKILL.md from the shared body and the
# agent-specific front matter. Deterministic: no dates or versions are
# embedded, so `make check-skills` can diff the output against git.
#
# usage: scripts/gen-skills.sh            # generate all
#        scripts/gen-skills.sh --check    # generate, then fail if git sees changes
set -eu

cd "$(dirname "$0")/.."
BODY=skills/bitbucket.body.md

gen() {
  agent_dir=$1
  agent_name=$2
  out="skills/$agent_dir/bitbucket/SKILL.md"
  fm="skills/$agent_dir/bitbucket/frontmatter.md"
  [ -f "$fm" ] || { echo "missing $fm" >&2; exit 1; }
  # AGENT_NAME is substituted with awk to avoid sed delimiter/escape issues.
  { cat "$fm"; printf '\n'; awk -v name="$agent_name" '{ gsub(/AGENT_NAME/, name); print }' "$BODY"; } > "$out"
  [ "$(head -n 1 "$out")" = "---" ] || { echo "$out: first line must be --- (front matter)" >&2; exit 1; }
}

gen claude_code "Claude Code"
gen codex "Codex"
gen copilot "GitHub Copilot"

if [ "${1:-}" = "--check" ]; then
  if [ -n "$(git status --porcelain -- skills/)" ]; then
    echo "skills/ is out of date or has uncommitted edits; run 'make skills' and commit the result:" >&2
    git status --short -- skills/ >&2
    exit 1
  fi
fi
