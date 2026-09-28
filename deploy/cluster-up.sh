#!/usr/bin/env bash
# Runs ON THE SERVER, piped over SSH.
# Expects the kind config to already be at /tmp/nydus-kind.yaml (the Makefile puts it there).
set -euo pipefail

CLUSTER="lab"
REG_NAME="kind-registry"
REG_PORT="5000"
CONFIG="/tmp/nydus-kind.yaml"

[ -f "${CONFIG}" ] || { echo "missing ${CONFIG}" >&2; exit 1; }

kind create cluster --name "${CLUSTER}" --config "${CONFIG}" --wait 120s

# Teach every node that localhost:5000 means the registry container.
REGISTRY_DIR="/etc/containerd/certs.d/localhost:${REG_PORT}"
for node in $(kind get nodes --name "${CLUSTER}"); do
  docker exec "${node}" mkdir -p "${REGISTRY_DIR}"
  echo "[host.\"http://${REG_NAME}:5000\"]" \
    | docker exec -i "${node}" cp /dev/stdin "${REGISTRY_DIR}/hosts.toml"
done

# Put the registry on the cluster's network so the nodes can actually reach it.
if [ "$(docker inspect -f '{{json .NetworkSettings.Networks.kind}}' "${REG_NAME}")" = "null" ]; then
  docker network connect kind "${REG_NAME}"
  echo "registry joined kind network"
fi

kind get nodes --name "${CLUSTER}"
