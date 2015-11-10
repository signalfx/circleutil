#!/bin/bash
set -ex
verify_in_circle

if [ -z "$TMP_GOPATH" ]; then
  export TMP_GOPATH="$HOME/gopath_temp"
fi

if [ -z "$GOPATH_INTO" ]; then
  export GOPATH_INTO="$HOME/bin"
fi

export GOPATH="$HOME/install_gobuild_lints"
for GOGET_URL in "$@"; do
  echo "GOGET_URL is $GOGET_URL"
  IFS=':' read -ra NAMES <<< "$GOGET_URL"
  clone_repo "https://${NAMES[0]}.git" "$GOPATH/src/${NAMES[0]}" "${NAMES[1]}"
  (
    cd "$GOPATH/src/${NAMES[0]}"
    go install .
  )
done
mkdir -p "$GOPATH_INTO"
cp "$GOPATH/bin/"* "$GOPATH_INTO/"
