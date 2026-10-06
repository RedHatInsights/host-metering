#!/bin/bash
# Go is already available in the default path in modern UBI images.

cd "${WORKDIR:-/workspace/host-metering}"
exec "$@"

