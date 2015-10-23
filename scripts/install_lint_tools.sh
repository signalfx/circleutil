#!/bin/bash
set -x
set -e
if [ -z "$CIRCLECI" ]; then
	echo "Very likely you only want to run this setup step on circleCI"
	exit 1
fi

if [ -z "$CACHED_LINT_TOOLS_DIR" ]; then
	echo "Set cached lint tools to the directory you want to install this cache into"
	exit 1
fi

export GOPATH="$HOME/tmp_gopath"

mkdir -p "$CACHED_LINT_TOOLS_DIR"
go get -u github.com/cep21/gobuild
"$GOPATH/bin/gobuild" -verbose install
cp "$GOPATH/bin/*" "$CACHED_LINT_TOOLS_DIR/"
rm -rf "$GOPATH"
