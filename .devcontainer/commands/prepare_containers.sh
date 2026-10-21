#!/bin/bash

pushd "$(dirname "$0")/.." || exit 1
DEVCONTAINER_PATH=$(pwd)
popd || exit 1

LOCAL_COMPOSE_FILE=$DEVCONTAINER_PATH/docker-compose.local.yml

if [ ! -f "$LOCAL_COMPOSE_FILE" ]; then
	cat >"$LOCAL_COMPOSE_FILE" <<EOF
version: '3'

services:
  host-metering:
    build:
      context: ${DEVCONTAINER_PATH}
    command: /bin/sh -c "while sleep 1000; do :; done"
EOF
fi
