
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

function install_go_version() {
  mkdir -p "$1"
  if [ ! -d "$1/go$2" ]; then
    mkdir "$1/go$2"
    wget -O - https://storage.googleapis.com/golang/go"$2".linux-amd64.tar.gz | tar -v -C "$1/go$2" -xzf -
  fi
  mv "$GOROOT" "${GOROOT}_backup" || rm "$GOROOT"
  ln -s "$1/go$2/go" "$GOROOT"
}

function install_all_go_versions() {
  install_go_ver "$1" 1.3.3
  install_go_ver "$1" 1.4.3
  install_go_ver "$1" 1.5.1
}

function install_shellcheck() {
  if [ ! -f "$1/shellcheck" ]; then
    mkdir -p "$1"
    SHELLCHECK_VERSION="0.3.7-4"
    wget http://ftp.debian.org/debian/pool/main/s/shellcheck/shellcheck_${SHELLCHECK_VERSION}_amd64.deb
    dpkg -x shellcheck_${SHELLCHECK_VERSION}_amd64.deb "/tmp/shellcheck"
    cp "/tmp/shellcheck/usr/bin/shellcheck" "$1/shellcheck"
  fi
  which shellcheck
}


# prints out the tag you should use for docker images, only doing "latest"
# on the release branch, but otherwise using the circle tag or branch as
# the tag on docker.  Suffix the tag with DOCKER_TAG_SUFFIX if set.
function docker_tag() {
  DOCKER_TAG=${DOCKER_TAG-${CIRCLE_TAG-${CIRCLE_BRANCH}}}$DOCKER_TAG_SUFFIX"
  DOCKER_TAG=$(echo "$DOCKER_TAG" | sed -e 's#.*/##')
  if [ -z $DOCKER_TAG ]; then
    echo -n "unknown"
    return 1 
  fi
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

function versioned_goget() {
  if [ -z "$TMP_GOPATH" ]; then
    TMP_GOPATH="/tmp/gopath_temp"
  fi

  if [ -z "$GOPATH_INTO" ]; then
    GOPATH_INTO="$HOME/bin"
  fi

  for GOGET_URL in "$@"; do
    print_time "GOGET_URL is $GOGET_URL"
    IFS=':' read -ra NAMES <<< "$GOGET_URL"
    clone_repo "https://${NAMES[0]}.git" "$TMP_GOPATH/src/${NAMES[0]}" "${NAMES[1]}"
    (
      cd "$TMP_GOPATH/src/${NAMES[0]}"
      GOPATH="$TMP_GOPATH" go install .
    )
  done
  mkdir -p "$GOPATH_INTO"
  cp "$TMP_GOPATH/bin/"* "$GOPATH_INTO/"
}
