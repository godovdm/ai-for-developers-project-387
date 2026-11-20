#!/usr/bin/env bash
# History is owned by GitHub Actions; PR comments do not control this counter.
set -euo pipefail

: "${GH_TOKEN:?}" "${GITHUB_REPOSITORY:?}" "${GITHUB_RUN_ID:?}" "${GITHUB_RUN_ATTEMPT:?}" "${PR_NUMBER:?}"
[[ "$PR_NUMBER" =~ ^[1-9][0-9]*$ && "$GITHUB_RUN_ID" =~ ^[1-9][0-9]*$ && "$GITHUB_RUN_ATTEMPT" =~ ^[1-9][0-9]*$ ]]

# Pagination and exact display_title include cancelled runs and re-runs. The current
# attempt must be visible, otherwise an incomplete history fails closed.
runs=$(gh api --paginate --slurp "repos/$GITHUB_REPOSITORY/actions/workflows/opencode.yml/runs?per_page=100")
created=$(gh api "repos/$GITHUB_REPOSITORY/pulls/$PR_NUMBER" --jq .created_at)
[[ "$created" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$ ]]
count=$(printf '%s' "$runs" | jq -er --arg title "oc-pr-$PR_NUMBER" --arg created "$created" --argjson id "$GITHUB_RUN_ID" --argjson attempt "$GITHUB_RUN_ATTEMPT" '
  type == "array" and length > 0 and
  all(.[]; (.workflow_runs | type) == "array") and
  # Older versions did not identify a PR in the run name. If one could have
  # belonged to this PR, the historical count is unknowable: fail closed.
  all(.[].workflow_runs[]; (.display_title | type) == "string" and
      (.created_at | type) == "string" and
      (.created_at < $created or .display_title == "opencode-other" or
       (.display_title | test("^oc-pr-[1-9][0-9]*$")))) and
  ([.[].workflow_runs[] | select(.display_title == $title)] as $runs |
    all($runs[]; (.id | type) == "number" and (.run_attempt | type) == "number" and .run_attempt >= 1) and
    ([$runs[] | select(.id == $id and .run_attempt == $attempt)] | length) == 1 and
    true)
' >/dev/null && printf '%s' "$runs" | jq -er --arg title "oc-pr-$PR_NUMBER" '[.[].workflow_runs[] | select(.display_title == $title) | .run_attempt] | add // 0')
printf 'Attempts: %s / 3\n' "$count"
if (( count > 3 )); then
  exit 2
fi
