#!/bin/bash

# Required parameters:
# @raycast.schemaVersion 1
# @raycast.title Paste Next Line
# @raycast.mode silent
# @raycast.packageName Queue Tools
# @raycast.icon 📥

# Путь к файлу-очереди
# FILE="$HOME/Documents/raycast-queue.txt"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FILE="$SCRIPT_DIR/raycast-queue.txt"
EMPTY_QUEUE_MESSAGE="Queue file is empty"

set -euo pipefail

if [ ! -f "$FILE" ]; then
  echo "File not found: $FILE"
  exit 1
fi

if [ ! -s "$FILE" ]; then
  echo "$EMPTY_QUEUE_MESSAGE"
  exit 1
fi

# Берем первую строку
first_line="$(head -n 1 "$FILE")"

# Вставляем текст через clipboard, чтобы не зависеть от раскладки
osascript - "$first_line" <<'APPLESCRIPT'
on run argv
  set pasteText to item 1 of argv
  set originalClipboard to the clipboard
  set frontAppName to missing value
  try
    tell application "System Events"
      set frontAppName to name of first application process whose frontmost is true
    end tell

    set the clipboard to pasteText
    delay 0.1

    try
      tell application id "com.raycast.macos" to hide
    end try

    if frontAppName is not missing value then
      tell application frontAppName to activate
    end if

    delay 0.8

    tell application "System Events"
      key code 9 using command down
    end tell

    delay 0.2
  on error errMsg number errNum
    set the clipboard to originalClipboard
    error errMsg number errNum
  end try
  set the clipboard to originalClipboard
end run
APPLESCRIPT

# Удаляем первую строку из файла только после успешной вставки
tmp_file="$(mktemp)"
tail -n +2 "$FILE" > "$tmp_file"
mv "$tmp_file" "$FILE"
