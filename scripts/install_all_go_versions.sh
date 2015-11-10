#!/bin/bash

# Install versions of go into GO_BIN_INTO
set -ex
verify_in_circle

if [ -z "$GO_BIN_INTO" ]; then
  GO_BIN_INTO="$HOME/gover"
fi

if [ -z "$GOLANG_VERSION" ]; then
  GOLANG_VERSION="1.5.1"
fi

install_go_ver() {
  if [ ! -d "$GO_BIN_INTO/go$1" ]; then
    mkdir "$GO_BIN_INTO/go$1"
    wget -O - https://storage.googleapis.com/golang/go"$1".linux-amd64.tar.gz | tar -v -C "$GO_BIN_INTO/go$1" -xzf -
  fi
}

[ -d /usr/local/go ] && sudo mv /usr/local/go /usr/local/go_backup
mkdir -p "$GO_BIN_INTO"
install_go_ver 1.5.1
install_go_ver 1.4.3
install_go_ver 1.3.3
install_go_ver $GOLANG_VERSION

mv "$GOROOT" "${GOROOT}_backup" || true
ln -s "$GO_BIN_INTO/go$GOLANG_VERSION" "$GOROOT"
