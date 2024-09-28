#!/bin/bash

[[ -z "$1" ]] && { echo "Directory input is empty" ; exit 1; }

cd "$1"
for file in ./*; do
  ext=$(echo ${file} | sed 's,^.*\(\.[^\.]*$\),\1,')
  mv "$file" $((RANDOM * 32768 + RANDOM))${ext}
done