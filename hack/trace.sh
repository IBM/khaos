#!/bin/bash

PODS=$(kubectl get pods -n khaos-webhook) \
POD_NAME=$(echo "$PODS" | grep Running | awk '{print $1}') 

echo $POD_NAME

kubectl logs -f $POD_NAME -n khaos-webhook