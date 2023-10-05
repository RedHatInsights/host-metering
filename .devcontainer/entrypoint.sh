#!/bin/bash
SCLS=go-toolset-1.19

if [ -x "$(command -v scl_source)" ]; then
  source scl_source enable $SCLS
fi

cd $WORKDIR
exec "$@"
