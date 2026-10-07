#!/bin/bash

pushd "$(dirname "$0")" >/dev/null || exit 1
#prepend path with the mocks folder
MOCKS_PATH=$(pwd)
PATH=$MOCKS_PATH:$PATH
popd >/dev/null || exit 1

pushd "$(dirname "$0")/.." >/dev/null || exit 1
HOST_METERING_HOST_CERT_PATH=$MOCKS_PATH/consumer/cert.pem HOST_METERING_HOST_CERT_KEY_PATH=$MOCKS_PATH/consumer/key.pem go run main.go "$@"
popd >/dev/null || exit 1
