#!/bin/bash
set -ex
which python
EXIT_CODE=0
LOOP_PARTS=$(cat | junitappend split)
for d in $LOOP_PARTS;
do
  print_time "Running test $d"
  START_TIME=$(python -c 'import time;print time.time()')
  DID_CMD_WORK="0"
  FAILURE_MSG=""
  $d || DID_CMD_WORK="1"
  if [ "$DID_CMD_WORK" -ne "0" ]; then
    FAILURE_MSG="Failed to run command $d"
    EXIT_CODE=1
  fi
  END_TIME=$(python -c 'import time;print time.time()')
  TOTAL_TIME=$(python -c 'import sys;print float(sys.argv[2]) - float(sys.argv[1])' "$START_TIME" "$END_TIME")
  junitappend -testname "$d" -testduration "${TOTAL_TIME}s" -failuremsg "$FAILURE_MSG" add
done
exit $EXIT_CODE
