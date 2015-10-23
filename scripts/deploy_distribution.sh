#!/bin/bash
set -x
set -e
if [ -z "$CIRCLECI" ]; then
	echo "Very likely you only want to run this setup step on circleCI"
	exit 1
fi

if [ -z "$GITHUB_TOKEN" ]; then
	echo "github token not set in env.  Set this in the UI to deploy your code"
	exit 1
fi

go get github.com/mitchellh/gox
cd "$SRC_PATH" && gox -output "$CIRCLE_ARTIFACTS/dist/gobuild_{{.OS}}_{{.Arch}}"
go get github.com/tcnksm/ghr
ghr -t "$GITHUB_TOKEN" -u "$CIRCLE_PROJECT_USERNAME" -r "$CIRCLE_PROJECT_REPONAME" --replace "$(git describe --tags)" "$CIRCLE_ARTIFACTS/dist/"
