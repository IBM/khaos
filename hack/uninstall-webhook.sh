#!/bin/bash

SCRIPTS_HOME=$(dirname ${BASH_SOURCE})
kubectl delete -f ${SCRIPTS_HOME}/../deploy
kubectl delete namespace ktest-webhook