#!/bin/bash

pushd $(dirname "$0")
#prepend path with the mocks folder
MOCKS_PATH=$(pwd)
PATH=$MOCKS_PATH:$PATH
popd

pushd $(dirname "$0")/..
HOST_METERING_HOST_CERT_PATH=$MOCKS_PATH/consumer/cert.pem HOST_METERING_HOST_CERT_KEY_PATH=$MOCKS_PATH/consumer/key.pem go run main.go $@
popd