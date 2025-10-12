#!/usr/bin/env zsh

# OCR Screenshot Grabber
# This script captures a screenshot using interactive selection, performs OCR (Optical Character Recognition)
# on the captured image to extract text in English and Russian, and copies the extracted text to the clipboard.
# Dependencies: screencapture, tesseract, pbcopy, sips

set -euo pipefail
LANGS="eng+rus"

export PATH="/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"
if [ -d /opt/homebrew/share/tessdata ]; then
  export TESSDATA_PREFIX="/opt/homebrew/share/tessdata"
elif [ -d /usr/local/share/tessdata ]; then
  export TESSDATA_PREFIX="/usr/local/share/tessdata"
fi

for bin in screencapture tesseract pbcopy; do
  command -v "$bin" >/dev/null || { echo "Missing $bin" >&2; exit 2; }
done

tmp_png="$(mktemp -t ocrgrab).png"
cleanup() { rm -f "$tmp_png" 2>/dev/null || true; }
trap cleanup EXIT

if ! screencapture -i -t png "$tmp_png"; then
  exit 0
fi

sips -s dpiWidth 300 -s dpiHeight 300 "$tmp_png" >/dev/null 2>&1 || true

text="$(tesseract "$tmp_png" stdout -l "$LANGS" --psm 6 2>/dev/null | tr -d $'\f')"

printf %s "$text" | pbcopy
