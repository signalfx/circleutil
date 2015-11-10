#!/bin/bash
set -ex
verify_in_circle

if [ -z "$GOBUILD_PATH" ]; then
  export GOBUILD_PATH="github.com/cep21/gobuild"
fi

if [ -z "$1" ]; then
  print "Please set the path to copy go binaries into"
  exit 1
fi

export GOPATH="$HOME/install_gobuild_lints"
mkdir -p "$GOPATH/src/$GOBUILD_PATH"
clone_repo "https://$GOBUILD_PATH.git" "$GOPATH/src/$GOBUILD_PATH" "$2"
(
  cd "$GOPATH/src/$GOBUILD_PATH"
  go install .
)
for path in "$@"; do
  go get -u "$path"
done
"$GOPATH/bin/gobuild" -verbose install
mkdir -p "$1"
cp "$GOPATH/bin/"* "$1/"
