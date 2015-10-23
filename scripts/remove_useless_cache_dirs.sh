#!/bin/bash
set -x
set -e
if [ -z "$CIRCLECI" ]; then
	echo "Very likely you only want to run this setup step on circleCI"
	exit 1
fi
rm -rf "$HOME/.go_workspace" "$HOME/.gradle" "$HOME/.ivy2" "$HOME/.m2"
