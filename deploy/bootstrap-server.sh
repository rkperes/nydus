#!/usr/bin/env bash
# Runs ON THE SERVER, piped over SSH. Idempotent.
set -euo pipefail

REG_NAME="kind-registry"
REG_PORT="5000"
REG_IMAGE="registry:2.8.3"

running="$(docker inspect -f '{{.State.Running}}' "${REG_NAME}" 2>/dev/null || echo false)"
if [ "${running}" != "true" ]; then
  docker rm -f "${REG_NAME}" >/dev/null 2>&1 || true
  docker run -d \
    --restart=always \
    --name "${REG_NAME}" \
    -p "127.0.0.1:${REG_PORT}:5000" \
    "${REG_IMAGE}"
  echo "registry started"
else
  echo "registry already running"
fi

docker inspect -f '{{.State.Status}} {{.HostConfig.RestartPolicy.Name}}' "${REG_NAME}"
