#!/bin/bash
set -x
(which shellcheck &> /dev/null)
if [ $? -ne 0 ]; then
	set -e
	wget http://ftp.debian.org/debian/pool/main/s/shellcheck/shellcheck_0.3.7-4_amd64.deb
	mkdir "$HOME/shellcheck"
	dpkg -x "shellcheck_0.3.7-1_amd64.deb $HOME/shellcheck"
	cp "$HOME/shellcheck/bin/shellcheck $CACHED_LINT_TOOLS_DIR"
fi
