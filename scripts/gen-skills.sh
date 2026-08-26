#!/bin/sh
# Generate skills/<agent>/bitbucket/SKILL.md for each agent from
#   skills/<agent>/bitbucket/frontmatter.md   agent-specific front matter
#   skills/bitbucket.description.txt          the shared one-line description
#   skills/bitbucket.body.md                  the shared body
# AGENT_NAME and SKILL_DESCRIPTION are replaced literally in the front matter
# and the body. Deterministic: no dates or versions are embedded, so the
# output is checked in and compared by CI.
#
# usage: scripts/gen-skills.sh            # (re)generate the three files
#        scripts/gen-skills.sh --check    # generate into a temporary directory
#                                         # and fail if a checked-in file differs
set -eu

cd "$(dirname "$0")/.."
BODY=skills/bitbucket.body.md
DESCFILE=skills/bitbucket.description.txt

check=0
outroot=skills
case "${1:-}" in
  "") ;;
  --check)
    check=1
    outroot=$(mktemp -d)
    trap 'rm -rf "$outroot"' EXIT
    ;;
  *)
    echo "usage: $0 [--check]" >&2
    exit 2
    ;;
esac

# The description is inserted inside the double quotes of the YAML
# `description:` line, so it must be a single non-empty line without
# characters that would end or escape that string.
[ -s "$DESCFILE" ] || { echo "$DESCFILE: missing or empty" >&2; exit 1; }
[ "$(wc -l < "$DESCFILE" | tr -d ' ')" -le 1 ] || { echo "$DESCFILE: must be a single line" >&2; exit 1; }
DESCRIPTION=$(head -n 1 "$DESCFILE")
case "$DESCRIPTION" in
  *'"'*|*'\'*) echo "$DESCFILE: must not contain double quotes or backslashes" >&2; exit 1 ;;
esac
stale=0

# render <file> <agent name>: prints <file> with SKILL_DESCRIPTION and then
# AGENT_NAME replaced. index()/substr() substitute literally, unlike sed or
# gsub(), where `&` and backslashes in the replacement are special.
render() {
  AGENT_NAME="$2" SKILL_DESCRIPTION="$DESCRIPTION" awk '
    function subst(line, key, val,    out, i) {
      out = ""
      while ((i = index(line, key)) > 0) {
        out = out substr(line, 1, i - 1) val
        line = substr(line, i + length(key))
      }
      return out line
    }
    {
      line = subst($0, "SKILL_DESCRIPTION", ENVIRON["SKILL_DESCRIPTION"])
      print subst(line, "AGENT_NAME", ENVIRON["AGENT_NAME"])
    }' "$1"
}

gen() {
  agent_dir=$1
  agent_name=$2
  fm="skills/$agent_dir/bitbucket/frontmatter.md"
  target="skills/$agent_dir/bitbucket/SKILL.md"
  out="$outroot/$agent_dir/bitbucket/SKILL.md"
  [ "$(head -n 1 "$fm" 2>/dev/null)" = "---" ] || {
    echo "$fm: missing, or its first line is not --- (front matter)" >&2
    exit 1
  }
  mkdir -p "$(dirname "$out")"
  { render "$fm" "$agent_name"; printf '\n'; render "$BODY" "$agent_name"; } > "$out"
  if [ "$check" = 1 ] && ! cmp -s "$out" "$target"; then
    echo "$target is stale or missing; run 'make skills' and commit the result" >&2
    stale=1
  fi
}

gen claude_code "Claude Code"
gen codex "Codex"
gen copilot "GitHub Copilot"

exit "$stale"
