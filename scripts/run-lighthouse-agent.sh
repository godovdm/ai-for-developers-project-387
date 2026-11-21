#!/bin/sh
set -eu

: "${LIGHTHOUSE_REPORT:?LIGHTHOUSE_REPORT is required}"
: "${APP_URL:?APP_URL is required}"
: "${TRAINING_MODE:?TRAINING_MODE is required}"
: "${GITHUB_RUN_URL:?GITHUB_RUN_URL is required}"
: "${GH_TOKEN:?GH_TOKEN is required}"

case "$TRAINING_MODE" in
  true|false) ;;
  *) echo 'TRAINING_MODE must be true or false' >&2; exit 1 ;;
esac

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
prompt=$(cat "$script_dir/../prompts/lighthouse-audit.md")
export LIGHTHOUSE_REPORT APP_URL TRAINING_MODE GITHUB_RUN_URL GH_TOKEN
opencode run --standalone --auto --model opencode-go/deepseek-v4.1-flash "$prompt"
