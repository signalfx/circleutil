#!/bin/bash
set -x
set -e
if [ -z "$CIRCLECI" ]; then
	echo "Very likely you only want to run this setup step on circleCI"
	exit 1
fi

export GOINTO="/usr/local/gover"

#TODO: Move these checks into vendor?
install_go_ver() {
  if [ ! -d $GOINTO/go"$1" ]; then
    sudo mkdir $GOINTO/go"$1"
    wget -O - https://storage.googleapis.com/golang/go"$1".linux-amd64.tar.gz | sudo tar -v -C $GOINTO/go"$1" -xzf -
  fi
}


[ -d /usr/local/go ] && sudo mv /usr/local/go /usr/local/go_backup
sudo mkdir -p $GOINTO
install_go_ver 1.5.1
install_go_ver 1.4.2

sudo ln -s $GOINTO/go1.5.1/go /usr/local/go
