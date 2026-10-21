#!/bin/bash

ACTUAL_USER=$(id -u)
ACTUAL_GROUP=$(id -g)

echo running subscription-manager identity
sudo subscription-manager identity | tee subscription-manager-identity >/dev/null
echo running subscription-manager syspurpose --show
sudo subscription-manager syspurpose --show | tee "subscription-manager-syspurpose --show" >/dev/null
echo running subscription-manager facts
sudo subscription-manager facts | tee subscription-manager-facts >/dev/null

echo preparing /etc/pki/consumer
sudo cp -r /etc/pki/consumer ./
sudo chown $ACTUAL_USER:$ACTUAL_GROUP -R ./consumer
