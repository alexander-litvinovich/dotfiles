#!/bin/bash
# sessionStart hook: inject global agent rules into the conversation's
# initial system context. Fails open (empty context) if the file is missing.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RULES_FILE="${SCRIPT_DIR}/../AGENTS.md"

if [[ ! -r "${RULES_FILE}" ]]; then
    echo '{}'
    exit 0
fi

if [[ "$(head -n 1 "${RULES_FILE}")" == "@RTK.md" ]]; then
    tail -n +2 "${RULES_FILE}" | jq -Rs '{additional_context: .}'
else
    jq -Rs '{additional_context: .}' < "${RULES_FILE}"
fi
