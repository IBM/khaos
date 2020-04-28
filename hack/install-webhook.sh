#!/bin/bash

SCRIPTS_HOME=$(dirname ${BASH_SOURCE})

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: ktest-webhook
  labels:
    app.kubernetes.io/name: ktest-webhook
EOF


${SCRIPTS_HOME}/webhook-create-signed-cert.sh
cat ${SCRIPTS_HOME}/templates/mutatingwebhook.yaml | ${SCRIPTS_HOME}/webhook-patch-ca-bundle.sh > ${SCRIPTS_HOME}/../deploy/006-mutatingwebhookconfig.yaml

kubectl apply -f ${SCRIPTS_HOME}/../deploy
