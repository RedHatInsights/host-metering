#!/bin/bash

if [ -z "${HTTP_PROXY}" ] || [ -z "${HTTPS_PROXY}" ]; then
  HTTP_PROXY=""
  HTTPS_PROXY=""

  if [ -f /etc/rhsm/rhsm.conf ]; then
      PROXY_HOSTNAME=$(grep -E '^proxy_hostname\s*=' /etc/rhsm/rhsm.conf | awk -F= '{print $2}' | xargs)
      PROXY_PORT=$(grep -E '^proxy_port\s*=' /etc/rhsm/rhsm.conf | awk -F= '{print $2}' | xargs)

      if [ -n "$PROXY_HOSTNAME" ] && [ -n "$PROXY_PORT" ]; then
          HTTP_PROXY="http://$PROXY_HOSTNAME:$PROXY_PORT"
          HTTPS_PROXY="http://$PROXY_HOSTNAME:$PROXY_PORT"
      fi
  fi
fi

export HTTP_PROXY
export HTTPS_PROXY

exec /usr/bin/host-metering daemon