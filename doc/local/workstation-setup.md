# Workstation setup

Driving the server from a laptop, and the loop you live in day to day.

The workstation is macOS on `arm64`. The server is Linux on `x86_64`. That mismatch
shapes everything below.

## The rule

**No source code on the server. No file on the server edited by hand.**

Every shortcut around this — `scp` the fix, `vim` it over SSH, `kubectl cp` the patched
file — makes the workflow one step less like deploying to a cluster you do not own,
which is the entire reason the project is built this way.

The one sanctioned exception is provisioning: scripts in `deploy/`, piped over SSH,
never copied. That is the local stand-in for Terraform.

## SSH

`~/.ssh/config`:

```
Host pc
    HostName 192.168.0.5
    User rkperes
    ServerAliveInterval 30
    ServerAliveCountMax 6
    LocalForward 3000 127.0.0.1:3000
    LocalForward 9090 127.0.0.1:9090
    LocalForward 8233 127.0.0.1:8233

# Tailnet path — works away from the LAN. Requires either an ssh: rule in the
# tailnet ACL policy, or `sudo tailscale set --ssh=false` on the server.
Host pc-ts
    HostName 100.120.164.25
    User rkperes
    ServerAliveInterval 30
    ServerAliveCountMax 6
```

The forwards cover a future app, Prometheus, and Temporal's UI.

`HostName` is the LAN address because neither mDNS nor MagicDNS resolved reliably from
this laptop. The Tailscale **IP** is the address that works from anywhere and is what
`kind.yaml` uses for the API server; it is stable per node.

### Key auth

```bash
ssh-keygen -t ed25519 -C "mac"
ssh-copy-id -i ~/.ssh/id_ed25519.pub rkperes@192.168.0.5
```

Then keep the passphrase in the keychain:

```
Host *
  AddKeysToAgent yes
  UseKeychain yes
  IdentityFile ~/.ssh/id_ed25519
```

> If Tailscale SSH is enabled on the server it bypasses `authorized_keys`, so testing
> through the tailnet proves nothing about your key. Test the real `sshd` explicitly:
> `ssh -o PasswordAuthentication=no rkperes@192.168.0.5`

### Host keys

Connecting by IP after having trusted a hostname triggers `Host key verification
failed`. Verify rather than blindly accept — compare the live key against the entry you
already trust:

```bash
ssh-keygen -F rkperes-linux0.local | grep -v '^#' | awk '{print $2, $3}' \
  | while read t k; do echo "$t $k" | ssh-keygen -lf -; done
ssh-keyscan -T 5 192.168.0.5 | ssh-keygen -lf -
```
Fingerprints match → `ssh-keyscan -T 5 192.168.0.5 >> ~/.ssh/known_hosts`.

## Cross-compilation

`arm64` workstation, `amd64` server. Every image must be `linux/amd64`.

A builder that can push to a registry — the default `docker` driver cannot:

```bash
docker buildx create --name nydus --driver docker-container --use
docker buildx inspect --bootstrap
```

**The cost is entirely in `RUN`.** Each `RUN` in a cross-built image executes under QEMU
at roughly 3–10× the native cost, and breaks outright on some native toolchains
(`node-gyp`, `bcrypt`, `sharp`).

The way around it is to do the work natively and copy the result in:

```dockerfile
# compiled on the workstation, natively, then placed in a foreign-arch image.
# No RUN, so no emulation.
FROM --platform=linux/amd64 gcr.io/distroless/static:nonroot
COPY bin/app-linux-amd64 /app
ENTRYPOINT ["/app"]
```

```bash
GOOS=linux GOARCH=amd64 go build -o bin/app-linux-amd64 ./cmd/app
```

This is free for Go. For Node with native modules it is not, and that is a real input to
any language decision this project makes later.

## Cluster access

`kubectl` talks to the API server over the tailnet — no tunnel, because `kind.yaml` binds
`apiServerAddress` to the server's Tailscale IP and kind places that address in the
serving certificate's SANs.

```bash
make kubeconfig
export KUBECONFIG=$HOME/.kube/nydus-lab.yaml
kubectl get nodes
```

Keep that export in your shell profile, or prefix commands with
`--kubeconfig`. Do **not** merge it into `~/.kube/config` alongside work clusters; a
mis-targeted `kubectl delete` is not worth the convenience.

The kubeconfig contains client certificates. It is written `chmod 600` and is
`.gitignore`d. Never commit it.

## Registry access

The registry lives on the server's **loopback only** — never exposed to the LAN or the
tailnet. Images reach it through an SSH tunnel:

```bash
make tunnel      # ssh -fN -L 5000:127.0.0.1:5000 pc
```

The reason for this shape: Docker treats `localhost` as an insecure registry by
default. Pushing to `localhost:5000` therefore needs no TLS certificate, no
`insecure-registries` daemon edit, and leaves nothing listening on a routable address.

Inside the cluster, containerd rewrites `localhost:5000` to the registry container on
the kind network, so the same image reference works in a manifest.

```bash
curl -fsS http://localhost:5000/v2/_catalog     # is the tunnel up?
make tunnel-stop
```

## The daily loop

```bash
# once per session
make tunnel
export KUBECONFIG=$HOME/.kube/nydus-lab.yaml

# per change
GOOS=linux GOARCH=amd64 go build -o bin/app-linux-amd64 ./cmd/app
docker buildx build --platform linux/amd64 -t localhost:5000/app:$(git rev-parse --short HEAD) --push .
kubectl set image deployment/app app=localhost:5000/app:$(git rev-parse --short HEAD)
kubectl rollout status deployment/app
```

Tagging by commit SHA rather than a moving tag is deliberate: it makes rollbacks
trivial and keeps `imagePullPolicy` honest. No `:latest`.

## After a server reboot

The machine returns on its own; the cluster does not.

```bash
ssh pc 'docker ps --format "{{.Names}}"'   # expect kind-registry (restart=always)
make cluster-up
make kubeconfig
make tunnel
```

Automating this is deferred. It is a known gap, recorded in
[`server-setup.md`](server-setup.md).

## Editors

Cursor and VS Code both offer Remote-SSH, and both are the **wrong tool here** —
editing files on the server is exactly what this model forbids. Edit locally, build
locally, deploy through the API.

Remote-SSH is still useful for one thing: reading logs and poking at the server while
diagnosing an infrastructure problem. Reading, not writing.

## Troubleshooting

| Symptom | Cause |
|---|---|
| `ssh: No route to host` on a `.local` name | mDNS not crossing the wireless/wired boundary. Use the IP. |
| `kex_exchange_identification: Connection reset` on the tailnet | Tailscale SSH enabled without an `ssh:` ACL rule. |
| `failed to do request` from `buildx --push` | Registry tunnel down. `make tunnel`. |
| `unknown driver` / `--push` unsupported | The `docker` driver cannot push. Create the `docker-container` builder. |
| TLS error naming a certificate SAN | The server's tailnet IP changed. Update `kind.yaml`, recreate the cluster. |
| `ErrImagePull` on `localhost:5000/...` | See Troubleshooting in the cluster-foundation plan. |
| Build inexplicably slow | A `RUN` instruction is executing under QEMU. |
