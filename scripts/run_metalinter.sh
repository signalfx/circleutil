#!/bin/bash
set -x


export CIRCLE_TEST_RUN_FILE="$CIRCLE_ARTIFACTS/testout"
# Run all tests in STDIN's directories.  Empty STDOUT means no test failures.  Any STDERR means
# there was a test failure.
while read p; do
        cd $p || exit 1
        echo "Running coverage on $p" 1>&2
        gocovercheck -required_coverage "$GO_COVERAGE_REQUIRED" -dirout "$1" "$p" 2>&1 > "$CIRCLE_TEST_RUN_FILE"
        cd - || exit 1
done
