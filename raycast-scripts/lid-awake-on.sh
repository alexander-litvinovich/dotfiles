#!/bin/bash
# @raycast.schemaVersion 1
# @raycast.title Lid Awake On
# @raycast.mode silent
# @raycast.packageName Power
# @raycast.description Keep the Mac awake with the lid closed, including on battery power

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec "$SCRIPT_DIR/../bin/lid-awake" on
