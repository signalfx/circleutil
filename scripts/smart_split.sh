#!/bin/bash
set -ex

function print_time() {
  echo "==PRINT_TIME $(date +%s) $(date) == $1"
}

which python
which junitappend
which cat

EXIT_CODE=0
while read line
do
  print_time "Running test $line"
  START_TIME=$(python -c 'import time;print time.time()')
  DID_CMD_WORK="0"
  FAILURE_MSG=""
  $line || DID_CMD_WORK="1"
  if [ "$DID_CMD_WORK" -ne "0" ]; then
    FAILURE_MSG="Failed to run command $line"
    EXIT_CODE=1
  fi
  END_TIME=$(python -c 'import time;print time.time()')
  TOTAL_TIME=$(python -c 'import sys;print float(sys.argv[2]) - float(sys.argv[1])' "$START_TIME" "$END_TIME")
  junitappend -testname "$line" -testduration "${TOTAL_TIME}s" -failuremsg "$FAILURE_MSG" add
done < "$(cat | junitappend split)"
exit $EXIT_CODE
