#!/bin/bash
# @raycast.schemaVersion 1
# @raycast.title Toggle Lid Awake
# @raycast.mode silent
# @raycast.packageName Power
# @raycast.description Toggle closed-lid operation, including on battery power

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec "$SCRIPT_DIR/../bin/lid-awake" toggle
