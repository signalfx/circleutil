#!/bin/bash
set -x
(which shellcheck &> /dev/null)
if [ $? -ne 0 ]; then
	set -e
	SHELLCHECK_VERSION="0.3.7-4"
	wget http://ftp.debian.org/debian/pool/main/s/shellcheck/shellcheck_${SHELLCHECK_VERSION}_amd64.deb
	mkdir "$HOME/shellcheck"
	dpkg -x shellcheck_${SHELLCHECK_VERSION}_amd64.deb "$HOME/shellcheck"
	cp "$HOME/shellcheck/usr/bin/shellcheck" "$CACHED_LINT_TOOLS_DIR"
fi
