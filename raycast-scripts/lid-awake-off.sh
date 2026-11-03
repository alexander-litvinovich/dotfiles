#!/bin/bash
# @raycast.schemaVersion 1
# @raycast.title Lid Awake Off
# @raycast.mode silent
# @raycast.packageName Power
# @raycast.description Restore normal sleep when the lid closes

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec "$SCRIPT_DIR/../bin/lid-awake" off
