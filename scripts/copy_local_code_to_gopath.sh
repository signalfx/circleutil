#!/bin/bash
set -x
set -e
if [ -z "$CIRCLECI" ]; then
	echo "Very likely you only want to run this setup step on circleCI"
	exit 1
fi

if [ -z "$SRC_PATH" ]; then
	echo "Did you forget to set SRC_PATH in your env?"
	exit 1
fi

mkdir -p "$SRC_PATH"
rsync -azC --delete ./ "$SRC_PATH"
