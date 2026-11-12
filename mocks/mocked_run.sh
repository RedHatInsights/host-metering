#!/bin/bash

pushd "$(dirname "$0")" >/dev/null || exit 1
#prepend path with the mocks folder
MOCKS_PATH=$(pwd)
PATH=$MOCKS_PATH:$PATH
popd >/dev/null || exit 1

pushd "$(dirname "$0")/.." >/dev/null || exit 1
# The mocked run is self-contained: it writes to the local Prometheus
# receiver, never to the compiled-in production default. Set
# HOST_METERING_WRITE_URL to override (e.g. http://prometheus:9090/...
# inside the devcontainer's compose network).
HOST_METERING_HOST_CERT_PATH=$MOCKS_PATH/consumer/cert.pem \
HOST_METERING_HOST_CERT_KEY_PATH=$MOCKS_PATH/consumer/key.pem \
HOST_METERING_WRITE_URL=${HOST_METERING_WRITE_URL:-http://localhost:9090/api/v1/write} \
go run main.go "$@"
popd >/dev/null || exit 1
