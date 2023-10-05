#!/bin/bash

ACTUAL_USER=$(id -u)
ACTUAL_GROUP=$(id -g)

echo running subscription-manager identity
sudo subscription-manager identity > subscription-manager-identity
echo running subscription-manager usage
sudo subscription-manager usage > subscription-manager-usage
echo running subscription-manager service-level
sudo subscription-manager service-level > subscription-manager-service-level
echo running subscription-manager facts
sudo subscription-manager facts > subscription-manager-facts

echo preparing /etc/pki/consumer
sudo cp -r /etc/pki/consumer ./
sudo chown $ACTUAL_USER:$ACTUAL_GROUP -R ./consumer
