
# Common bash functions unsed in circle scripts

# Exit if these env are empty
function verify_env() {
  for e in $1;
  do
    if [ -z "${!e}" ]; then
      echo "Cannot find variable $e defined"
      exit 1
    fi
  done
}

# Exit if not in circle ci
function verify_in_circle() {
  verify_env "CIRCLECI"
}
  
function print_time() {
  echo "==PRINT_TIME $(date +%s) $(date) == $1"
}

# Clone $1 into $2 and rebase a git repository
# Then checkout to revision $3
function clone_repo() {
  print_time "clone_repo $1 $2 $3"
  if [ ! -d "$2" ]; then
    mkdir -p "$2"
    git clone "$1" "$2"
  fi
  (
    cd "$2"
    git fetch --all -a --tags
    git fetch origin
    if [ -z "$3" ]; then
      git reset --hard origin/master
    else
      git reset --hard "$3"
    fi
  )
}

# Copies everything in the current directory to another.  Useful for
# gopath setup
function copy_local_to_path() {
  mkdir -p "$1"
  rsync -azC --delete ./ "$1"
}

# Load all docker images inside DOCKER_STORAGE
function load_docker_images() {
  verify_env "DOCKER_STORAGE"
  if [  -d "$DOCKER_STORAGE" ]; then
    find "$DOCKER_STORAGE" -name "*.tar" -exec docker load -i {} \;
  fi
}

# Cache the image $1 into the name $2 inside DOCKER_STORAGE
function cache_docker_image() {
  verify_env "DOCKER_STORAGE"
  mkdir -p "$DOCKER_STORAGE"
  docker save -o "$DOCKER_STORAGE/$2.tar" "$1"
}

# Gets the tag we should push a docker image for, assuming we only
# push 'latest' for branch 'release'.  Intended for shell capture
function docker_release_tag() {
  DOCKER_TAG=$(echo "$1" | sed -e 's#.*/##')
  if [ "$DOCKER_TAG" = "latest" ]; then
    echo -n "latest-branch"
    return
  fi
  if [ "$DOCKER_TAG" = "release" ]; then
    echo -n "latest"
    return
  fi
  echo -n "$DOCKER_TAG"
}

# Splits stdin and executes each line.
# Intended to be used as a subshell with stdin piped in.
# By doing it as a subshell with stdin piped in rather than xargs I avoid
# some of the xargs # of values limitations.
function speed_split() {
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
  done < <(cat | junitappend split)
  return $EXIT_CODE
}
