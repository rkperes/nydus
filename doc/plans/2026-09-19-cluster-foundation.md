# Cluster Foundation — Implementation Plan

> **For agentic workers:** execute top to bottom. Every step states its exact command,
> the expected output, and what to do if the output differs.
> **If a step fails twice, stop and report.** Do not improvise around infrastructure
> failures — a wrong guess here is expensive to unwind.
>
> **Every code block is labelled `workstation` or `server`.** Run it where it says.
> The only sanctioned way to touch the server is by piping a script from `deploy/`
> over SSH. Never edit a file on the server by hand.

**Goal:** a kind cluster running on a remote server, provisioned entirely from this
repo, with images built on the workstation and delivered through a registry — and
cgroup-v2 fidelity proven by test rather than assumed.

**Why this shape:** the point of the exercise is a workflow that transfers to a cluster
you have no shell access to. Colocating source with the cluster would be faster and
would teach nothing.

## Machines

| | Workstation | Server |
|---|---|---|
| host | macOS, `arm64` | `rkperes-linux0`, Ubuntu 26.04.1 (`resolute`), `x86_64` |
| reached as | — | `ssh pc` → `192.168.0.5` |
| tailnet | `100.74.44.90` | `100.120.164.25` |
| hardware | — | Ryzen 5 7600X (6c/12t), 32 GB RAM, bare metal |

## Preconditions — verified 2026-09-19

**Server:**

| Item | State |
|---|---|
| docker | `/usr/bin/docker`, usable without sudo |
| Cgroup Version | **2** |
| Cgroup Driver | systemd |
| kind | `/usr/local/bin/kind`, v0.34.0-alpha |
| kubectl | present (unused — kubectl runs on the workstation) |
| passwordless sudo | **no** — any sudo step needs a human |
| existing kind clusters | none |

**Workstation:** Docker, `ssh pc` working with key auth, repo at `~/src/nydus`.

**Known risks**

1. The server's kind is an *alpha* build, installed via the `dl/latest` endpoint. If
   Task 5 fails in a way that looks like a kind bug, see Troubleshooting → *Downgrade kind*.
2. A kind cluster does not reliably survive a server reboot. Accepted for now; making
   it come back automatically is deferred to a later plan. After a reboot, expect to run
   `make cluster-up` again.

## Definition of done

- [ ] `make bootstrap` provisions the registry on the server, idempotently
- [ ] `make cluster-up` creates 3 `Ready` nodes, from the workstation, without an
      interactive shell on the server
- [ ] `kubectl` from the workstation reaches the cluster over the tailnet
- [ ] An image built on the workstation, pushed to the registry, runs in the cluster
- [ ] A pod exceeding its memory limit reports `OOMKilled`
- [ ] `container_cpu_cfs_throttled_seconds_total` is exposed by a worker
- [ ] No source file has been created or edited on the server
- [ ] All of it committed to `main`

---

## Task 0: Git identity

**This step needs a human. Do not guess and do not proceed without it.**

The workstation's global git identity is a **work** address
(`rkochp@ext.uber.com`). This is a public personal repository. Committing under the
work identity leaks an employer relationship into a public history and is annoying
to rewrite later.

- [ ] **Step 1 (workstation): check**

```bash
cd ~/src/nydus && git config --local user.email || echo UNSET
```
Expected: an email, or `UNSET`.

- [ ] **Step 2 (workstation): if UNSET, set repo-locally from values the human supplies**

```bash
git config --local user.name  "<full name>"
git config --local user.email "<personal email>"
```

Repo-local, never `--global`. Verify it did not inherit the work address:

```bash
git config --local user.email
```
Expected: the personal address. If it shows `@ext.uber.com`, stop.

---

## Task 1: Workstation prerequisites

- [ ] **Step 1 (workstation): confirm the toolchain**

```bash
docker version --format '{{.Server.Version}}'
kubectl version --client -o yaml | grep gitVersion
make --version | head -1
ssh -o BatchMode=yes pc 'echo ssh ok'
```
Expected: a version from each, and `ssh ok`.
Any `command not found` → install it before continuing. Any SSH failure → stop.

- [ ] **Step 2 (workstation): create a buildx builder that can push**

The default `docker` driver cannot push build output to a registry. A
`docker-container` builder can.

```bash
docker buildx create --name nydus --driver docker-container --use 2>/dev/null || docker buildx use nydus
docker buildx inspect --bootstrap | head -5
```
Expected: `Name: nydus`, `Status: running`.

---

## Task 2: Server provisioning script

The server gets a container registry. It is published on **loopback only** — never on
the LAN or the tailnet — and images reach it through an SSH tunnel.

- [ ] **Step 1 (workstation): create `deploy/bootstrap-server.sh`**

```bash
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
```

- [ ] **Step 2 (workstation): make it executable**

```bash
chmod +x deploy/bootstrap-server.sh
```

- [ ] **Step 3 (workstation): run it against the server**

```bash
ssh pc 'bash -s' < deploy/bootstrap-server.sh
```
Expected: `registry started` (or `already running`), then `running always`.

- [ ] **Step 4 (workstation): confirm it is not exposed beyond loopback**

```bash
ssh pc 'ss -tlnp | grep 5000'
```
Expected: a listener on `127.0.0.1:5000`.
If it shows `0.0.0.0:5000`, the port publish is wrong. Fix the script and re-run.

---

## Task 3: Cluster definition

- [ ] **Step 1 (workstation): create `kind.yaml` at the repo root**

```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: lab

networking:
  # Tailnet address of the server. Placed in the API server cert SANs by kind,
  # so kubectl from any tailnet device validates cleanly.
  apiServerAddress: "100.120.164.25"
  apiServerPort: 6443

# Point containerd at a per-registry config directory. The hosts.toml that makes
# localhost:5000 resolve to the registry container is written by deploy/cluster-up.sh,
# because it must be created inside each node after the nodes exist.
containerdConfigPatches:
  - |-
    [plugins."io.containerd.grpc.v1.cri".registry]
      config_path = "/etc/containerd/certs.d"

nodes:
  - role: control-plane
  - role: worker
  - role: worker
```

- [ ] **Step 2 (workstation): confirm the tailnet address is still correct**

```bash
ssh pc 'tailscale ip -4'
```
Expected: `100.120.164.25`. If it differs, update `kind.yaml` before continuing —
a mismatch produces a TLS error in Task 6 that looks unrelated.

---

## Task 4: Cluster provisioning script

- [ ] **Step 1 (workstation): create `deploy/cluster-up.sh`**

```bash
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
```

- [ ] **Step 2 (workstation)**

```bash
chmod +x deploy/cluster-up.sh
```

---

## Task 5: Makefile

- [ ] **Step 1 (workstation): create `Makefile` at the repo root**

Recipe lines must begin with a **tab**.

```make
SERVER          ?= pc
CLUSTER         := lab
REG_PORT        := 5000
KUBECONFIG_FILE := $(HOME)/.kube/nydus-$(CLUSTER).yaml

.PHONY: bootstrap cluster-up cluster-down kubeconfig tunnel tunnel-stop status nuke

bootstrap:
	ssh $(SERVER) 'bash -s' < deploy/bootstrap-server.sh

cluster-up:
	cat kind.yaml | ssh $(SERVER) 'cat > /tmp/nydus-kind.yaml'
	ssh $(SERVER) 'bash -s' < deploy/cluster-up.sh
	$(MAKE) kubeconfig

cluster-down:
	ssh $(SERVER) 'kind delete cluster --name $(CLUSTER)'

kubeconfig:
	@mkdir -p $(dir $(KUBECONFIG_FILE))
	ssh $(SERVER) 'kind get kubeconfig --name $(CLUSTER)' > $(KUBECONFIG_FILE)
	@chmod 600 $(KUBECONFIG_FILE)
	@echo "run: export KUBECONFIG=$(KUBECONFIG_FILE)"

tunnel:
	@pgrep -f "ssh.*-L $(REG_PORT):127.0.0.1:$(REG_PORT)" >/dev/null \
		|| ssh -fN -L $(REG_PORT):127.0.0.1:$(REG_PORT) $(SERVER)
	@echo "registry tunnel up on localhost:$(REG_PORT)"

tunnel-stop:
	@pkill -f "ssh.*-L $(REG_PORT):127.0.0.1:$(REG_PORT)" || true

status:
	kubectl --kubeconfig $(KUBECONFIG_FILE) get nodes -o wide
	kubectl --kubeconfig $(KUBECONFIG_FILE) get pods -A

nuke: cluster-down tunnel-stop
	ssh $(SERVER) 'docker system prune -f'
```

- [ ] **Step 2 (workstation): verify the tabs survived**

```bash
cd ~/src/nydus && grep -cP '^\t' Makefile
```
Expected: `18`.
If `0`, the recipe lines are space-indented:
```bash
sed -i '' 's/^    /\t/' Makefile
```

---

## Task 6: Bring up the cluster

- [ ] **Step 1 (workstation)**

```bash
cd ~/src/nydus && make bootstrap && make cluster-up
```
First run pulls the node image on the server — allow several minutes.
Expected: three node names, then the `export KUBECONFIG=...` hint.

- [ ] **Step 2 (workstation): point kubectl at it**

```bash
export KUBECONFIG=$HOME/.kube/nydus-lab.yaml
kubectl config view --minify -o jsonpath='{.clusters[0].cluster.server}{"\n"}'
```
Expected: `https://100.120.164.25:6443` — **not** `https://127.0.0.1:...`.
If it shows loopback, `apiServerAddress` did not take. Re-check Task 3.

- [ ] **Step 3 (workstation): reach the cluster over the tailnet**

```bash
kubectl get nodes
```
Expected:
```
lab-control-plane   Ready   control-plane
lab-worker          Ready   <none>
lab-worker2         Ready   <none>
```
A TLS error naming a certificate SAN → the tailnet IP changed; see Task 3 Step 2.
A node `NotReady` after 2 minutes → Troubleshooting → *Node stuck NotReady*.

- [ ] **Step 4 (workstation): control plane healthy**

```bash
kubectl get pods -n kube-system
```
Expected: all `Running` or `Completed`. Any `CrashLoopBackOff` → stop and report.

---

## Task 7: Prove the registry round-trip

The real test of this whole model: an image built on the workstation reaching the
cluster without anyone touching the server.

- [ ] **Step 1 (workstation): open the tunnel**

```bash
make tunnel
```
Expected: `registry tunnel up on localhost:5000`.

- [ ] **Step 2 (workstation): confirm the registry answers**

```bash
curl -fsS http://localhost:5000/v2/_catalog
```
Expected: `{"repositories":[]}` on first run.
Connection refused → the tunnel is not up, or Task 2 did not run.

- [ ] **Step 3 (workstation): create `deploy/roundtrip/Dockerfile`**

Deliberately no `RUN` instruction, so nothing executes under emulation — this builds
at native speed despite targeting a foreign architecture.

```dockerfile
FROM --platform=linux/amd64 alpine:3.20
CMD ["sh", "-c", "echo nydus round-trip ok && sleep 3600"]
```

- [ ] **Step 4 (workstation): build for amd64 and push**

```bash
docker buildx build \
  --platform linux/amd64 \
  -t localhost:5000/nydus-roundtrip:0.1.0 \
  --push \
  deploy/roundtrip
```
Expected: ends with `pushing manifest`.
`failed to do request` → tunnel down. `unknown driver` → Task 1 Step 2 was skipped.

- [ ] **Step 5 (workstation): confirm the registry holds it**

```bash
curl -fsS http://localhost:5000/v2/_catalog
```
Expected: `{"repositories":["nydus-roundtrip"]}`

- [ ] **Step 6 (workstation): deploy it**

```bash
kubectl run roundtrip \
  --image=localhost:5000/nydus-roundtrip:0.1.0 \
  --restart=Never
sleep 20
kubectl get pod roundtrip -o jsonpath='{.status.phase}{"\n"}'
```
Expected: `Running`.
`ErrImagePull` / `ImagePullBackOff` → Troubleshooting → *Nodes cannot pull from the registry*.

- [ ] **Step 7 (workstation): confirm it is really your image**

```bash
kubectl logs roundtrip
```
Expected: `nydus round-trip ok`

- [ ] **Step 8 (workstation): clean up**

```bash
kubectl delete pod roundtrip --ignore-not-found
```

---

## Task 8: Prove cgroup-v2 memory enforcement

The claim under test: a container exceeding its memory limit is killed by the kernel,
and Kubernetes reports it accurately. The later load and failure work rests on this.

- [ ] **Step 1 (workstation): apply a pod that will exceed its limit**

`tail /dev/zero` allocates without bound. The limit is 64Mi.

```bash
cat <<'EOF' | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: oomtest
spec:
  restartPolicy: Never
  containers:
    - name: hog
      image: alpine:3.20
      command: ["sh", "-c", "tail /dev/zero"]
      resources:
        limits:
          memory: 64Mi
EOF
```

- [ ] **Step 2 (workstation): read the termination reason**

```bash
sleep 20
kubectl get pod oomtest -o jsonpath='{.status.containerStatuses[0].lastState.terminated.reason}{"\n"}'
```
Expected: `OOMKilled`

Anything else — empty, `Error`, `Completed` — means memory limits are not enforced the
way later phases assume. **Stop and report.** Do not proceed.

- [ ] **Step 3 (workstation)**

```bash
kubectl delete pod oomtest --ignore-not-found
```

---

## Task 9: Prove the CFS throttling metric exists

The load phase measures CPU throttling directly. Confirm the counter is exposed now,
while the cluster is idle and the signal is unambiguous.

- [ ] **Step 1 (workstation)**

```bash
kubectl get --raw "/api/v1/nodes/lab-worker/proxy/metrics/cadvisor" \
  | grep -m1 container_cpu_cfs_throttled_seconds_total
```
Expected: one line beginning with `container_cpu_cfs_throttled_seconds_total`.

Absent → stop and report. Do not work around it; its absence means the CPU accounting
the load phase depends on is unavailable.

---

## Task 10: Tear down and commit

- [ ] **Step 1 (workstation): full teardown**

```bash
cd ~/src/nydus && make cluster-down && make tunnel-stop
```

- [ ] **Step 2 (workstation): confirm the server is clean, registry aside**

```bash
ssh pc 'docker ps --format "{{.Names}}"'
```
Expected: only `kind-registry`. The registry is meant to outlive the cluster.

- [ ] **Step 3 (workstation): confirm no source reached the server**

```bash
ssh pc 'ls ~/src 2>&1'
```
Expected: `No such file or directory`. Anything else violates the core rule — report it.

- [ ] **Step 4 (workstation): prove `make cluster-up` works from nothing, twice**

```bash
make cluster-up && kubectl get nodes && make cluster-down
```
Expected: three `Ready` nodes, then deletion. This is the actual deliverable.

- [ ] **Step 5 (workstation): commit**

```bash
cd ~/src/nydus
git add -A
git commit -m "feat: remote kind cluster provisioned from the workstation

Registry on the server's loopback, reached through an SSH tunnel. API server
bound to the tailnet address. Verified the build-push-deploy round trip,
cgroup-v2 OOM enforcement, and CFS throttling metric availability."
```

- [ ] **Step 6: mark this plan Done**

Edit `doc/plans/README.md`, change this plan's Status to `Done`, and commit that edit.

---

## Troubleshooting

### Nodes cannot pull from the registry

`ErrImagePull` on `localhost:5000/...`. Check, in order:

```bash
# 1. is the registry on the cluster's network?
ssh pc 'docker inspect -f "{{json .NetworkSettings.Networks.kind}}" kind-registry'
```
`null` → `ssh pc 'docker network connect kind kind-registry'`

```bash
# 2. did the hosts.toml land in the node?
ssh pc 'docker exec lab-worker cat /etc/containerd/certs.d/localhost:5000/hosts.toml'
```
Expected: `[host."http://kind-registry:5000"]`. Missing → re-run `deploy/cluster-up.sh`.

```bash
# 3. can a node reach the registry at all?
ssh pc 'docker exec lab-worker wget -qO- http://kind-registry:5000/v2/_catalog'
```

If all three are correct and it still fails, containerd may have moved the CRI plugin
config key. Check the containerd version inside a node
(`ssh pc 'docker exec lab-worker containerd --version'`) against the kind release notes
for the `config_path` patch.

### Node stuck NotReady

Almost always inotify exhaustion — each kubelet consumes instances.

```bash
ssh pc 'cat /proc/sys/fs/inotify/max_user_instances /proc/sys/fs/inotify/max_user_watches'
```
Want `>= 8192` and `>= 524288`. If lower, **a human** must run on the server:
```bash
printf 'fs.inotify.max_user_instances=8192\nfs.inotify.max_user_watches=524288\n' \
  | sudo tee /etc/sysctl.d/99-kind.conf
sudo sysctl --system
```
Then `make cluster-down && make cluster-up`.

### TLS error mentioning certificate SANs

The tailnet address in `kind.yaml` no longer matches the server. Re-read it with
`ssh pc 'tailscale ip -4'`, update `kind.yaml`, and recreate the cluster.

### buildx is slow

Check for a `RUN` instruction in the Dockerfile. Every `RUN` executes under QEMU when
cross-building. Compile on the workstation natively and `COPY` the artefact in instead.

### Downgrade kind

The server's build is an alpha. To move to a named stable release, **a human** runs on
the server:
```bash
curl -Lo /tmp/kind "https://kind.sigs.k8s.io/dl/v0.30.0/kind-linux-amd64"
chmod +x /tmp/kind && sudo mv /tmp/kind /usr/local/bin/kind
kind version
```
Check <https://github.com/kubernetes-sigs/kind/releases> for the current stable tag
rather than using that version verbatim.

### Out of disk on the server

```bash
ssh pc 'docker system df'
ssh pc 'docker system prune -a'
```
Node images are large. `/var/lib/docker` sits on the 195 GiB root partition.
