#!/bin/bash
set -ex
verify_in_circle

if [ -z "$TMP_GOPATH" ]; then
  TMP_GOPATH="$HOME/gopath_temp"
fi

if [ -z "$GOPATH_INTO" ]; then
  GOPATH_INTO="$HOME/bin"
fi

for GOGET_URL in "$@"; do
  echo "GOGET_URL is $GOGET_URL"
  IFS=':' read -ra NAMES <<< "$GOGET_URL"
  clone_repo "https://${NAMES[0]}.git" "$TMP_GOPATH/src/${NAMES[0]}" "${NAMES[1]}"
  (
    cd "$TMP_GOPATH/src/${NAMES[0]}"
    GOPATH="$TMP_GOPATH" go install .
  )
done
mkdir -p "$GOPATH_INTO"
cp "$TMP_GOPATH/bin/"* "$GOPATH_INTO/"
