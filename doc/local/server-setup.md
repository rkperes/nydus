# Server setup

Turning a desktop PC into a headless Kubernetes host you can reach from anywhere.

Written from the machine actually built for this project. Where something bit, it is
recorded — the traps cost more time than the steps.

## The machine

| | |
|---|---|
| CPU | AMD Ryzen 5 7600X — 6 cores / 12 threads |
| RAM | 32 GB DDR5 |
| Disk | 1 TB NVMe (953 GiB usable), already holding Windows |
| GPU | RTX 4060 Ti (unused by this project) |
| OS | Ubuntu Server 26.04.1 LTS, codename `resolute` |

A three-node kind cluster with a modest workload uses roughly 8–11 GB. On 32 GB that
leaves 2× headroom, enough to grow to five nodes later.

Chose the `.1` point release deliberately: it is the first build of a new LTS where the
hardware enablement stack has settled.

## Disk: living alongside Windows

Windows was kept. The layout after installation:

| Partition | Size | Purpose |
|---|---|---|
| p1 | 100 MB | EFI system partition — **shared, never reformat** |
| p2 | 16 MB | Microsoft reserved |
| p3 | 757 GB | Windows `C:` |
| p4 | 905 MB | Windows recovery — leave alone |
| new | 195 GiB | Linux `/`, ext4 |

Decisions worth keeping:

- **No swap.** OOM kills should happen when memory runs out, not after a long
  degradation into swap. Fidelity of the failure signal matters more than resilience
  here, and kubelet is happier without it.
- **No LVM, no LUKS.** Full-disk encryption on a headless machine means a passphrase
  prompt at every boot, with no keyboard attached. That is a self-inflicted outage.
- **Align partitions.** Use `parted -a optimal`, or
  `echo '- - L -' | sudo sfdisk --append /dev/nvme0n1`. Byte-tight partitions straddle
  NAND erase blocks and inject a read-modify-write penalty into exactly the IO
  measurements this project takes later.
- Do not use `100%` as the partition end. The free space sat *between* `C:` and the
  recovery partition, not after it.

> **Near miss:** the installer initially proposed reformatting the 905 MB *recovery*
> partition as ext4. Read the partition table in the installer summary before
> confirming — sizes are the giveaway.

## Installer choices

- **Skip third-party / NVIDIA drivers.** The proprietary driver is the single most
  common cause of a headless machine that never comes back, and Secure Boot MOK
  enrollment needs a keyboard at the console. Install it later over SSH with
  `ubuntu-drivers install`, after disabling Secure Boot in BIOS while physically present.
- **Install OpenSSH server.** Without it the first boot is a dead end.
- Skip Ubuntu Pro and featured snaps.
- DHCP is fine; addressing is solved properly below.

> **Watch out:** it is easy to accidentally create a network bond over a single NIC in
> the installer's network screen. Harmless — it takes a normal DHCP lease — but it will
> confuse you later when the interface is `bond0` instead of `enp5s0`.

## Surviving an unexpected restart

The machine must come back on its own after a power cut. Three separate things block
that, and all three are silent.

### 1. GRUB `recordfail` — the big one

When a boot does not end cleanly, Ubuntu's GRUB sets a `recordfail` flag and, on the
next boot, **waits at the menu for a human keypress, ignoring `GRUB_TIMEOUT`**. Power
blips, the machine comes back, and sits at a menu forever. No SSH.

`/etc/default/grub`:

```
GRUB_DEFAULT=0
GRUB_TIMEOUT=3
GRUB_TIMEOUT_STYLE=menu
GRUB_RECORDFAIL_TIMEOUT=5
GRUB_CMDLINE_LINUX_DEFAULT="fsck.repair=yes"
```

```bash
sudo update-grub
grep -E "^menuentry" /boot/grub/grub.cfg | head -3   # confirm Ubuntu is entry 0
```

`fsck.repair=yes` is the second trap: after an unclean shutdown a filesystem needing
repair otherwise **prompts at the console** and halts the boot.

To see Windows in the menu at all:

```bash
sudo sed -i 's/^#\?GRUB_DISABLE_OS_PROBER=.*/GRUB_DISABLE_OS_PROBER=false/' /etc/default/grub
sudo update-grub    # expect "Found Windows Boot Manager"
```

### 2. A failing mount halts boot

If anything in `/etc/fstab` fails to mount, systemd drops to an emergency shell — no
network, no SSH. Add `nofail` to the ESP line:

```
UUID=XXXX-XXXX  /boot/efi  vfat  defaults,nofail  0  1
```

Leave `/` alone; if root fails there are bigger problems.

### 3. BIOS power-loss behaviour

Requires physical presence. **Restore on AC Power Loss → Power On.** Not "Last State",
not "Power Off". Without it the machine simply stays off after a cut.

While in BIOS: disable Secure Boot (so the GPU driver is later a plain `apt` job),
enable Wake-on-LAN, and confirm the NVMe is first in the boot order.

### Optional: hardware watchdog

Covers the worst case — kernel hung, machine powered on, nothing reboots it.

```bash
ls /dev/watchdog*
```
`/etc/systemd/system.conf`:
```
RuntimeWatchdogSec=30s
RebootWatchdogSec=10min
```
```bash
sudo systemctl daemon-reexec && wdctl
```

### Persistent logs

So the previous boot can be read after an unexplained restart:

```bash
sudo mkdir -p /var/log/journal
sudo systemd-tmpfiles --create --prefix /var/log/journal
```
Then `journalctl -b -1`.

### Testing it honestly

A clean `sudo reboot` does **not** exercise the `recordfail` path. Force a dirty reset:

```bash
sudo sysctl -w kernel.sysrq=1
echo b | sudo tee /proc/sysrq-trigger
```

> **Expect a long black screen.** AM5 boards re-train DDR5 after an unclean reset and
> emit **no video signal at all** for 60–120 seconds. This looks exactly like a dead
> machine. Wait three minutes before touching anything, and read the motherboard
> diagnostic LEDs (CPU → DRAM → VGA → BOOT) rather than guessing.
>
> Budget ~3 minutes for "unattended recovery", not 60 seconds.

## Remote access

Three mechanisms were tried. Only one is dependable.

### mDNS — unreliable here

```bash
sudo apt install -y avahi-daemon
sudo hostnamectl set-hostname rkperes-linux0
```
Gives `rkperes-linux0.local`, which macOS resolves natively.

It depends on the access point forwarding multicast between wireless and wired
segments. On this network it works intermittently and failed outright when needed.
Treat it as a convenience, never as the path you rely on.

### Tailscale — the one that works

```bash
curl -fsSL https://tailscale.com/install.sh | sh
sudo tailscale up
tailscale ip -4
```

Two things in the admin console, both easy to miss:

1. **Disable key expiry** on this machine (Machines → host → ⋯). Node keys otherwise
   expire after ~180 days, at which point a headless box you reach *via* Tailscale drops
   off the tailnet and needs a monitor. This is the biggest footgun for a home server.
2. **MagicDNS** (DNS tab) for names instead of `100.x.y.z`. Note it must also be
   accepted on the client — on this setup the server joined fine but the laptop could
   not resolve the short name, so the tailnet **IP** is what the tooling uses.

> **`tailscale up --ssh` was a mistake here.** It makes Tailscale intercept port 22 on
> the tailnet address and authenticate with tailnet identity instead of
> `authorized_keys` — but only if an `ssh:` rule exists in the tailnet ACL policy.
> Without that rule the tailnet SSH path is simply closed, and the failure looks like
> `kex_exchange_identification: Connection reset by peer`. Either add the ACL rule, or
> run `sudo tailscale set --ssh=false` to hand port 22 back to real `sshd`.

### SSH

Ubuntu now ships OpenSSH **socket-activated**, so `systemctl is-enabled ssh` reads
`disabled` while `ssh.socket` does the listening. That is normal:

```bash
systemctl is-enabled ssh.socket
sudo ss -tlnp | grep :22
```

Consequence worth remembering: with socket activation, `Port` and `ListenAddress` in
`sshd_config` are **ignored**. The socket unit owns them (`sudo systemctl edit ssh.socket`).

Install the workstation's key, then harden — in that order, keeping the working session
open while testing a second one:

```bash
sudo sed -i 's/^#\?PasswordAuthentication .*/PasswordAuthentication no/' /etc/ssh/sshd_config
sudo systemctl restart ssh
```

> When Tailscale SSH is enabled it bypasses `authorized_keys` entirely, so testing key
> auth through the tailnet name proves nothing. Test against the LAN address:
> `ssh -o PasswordAuthentication=no user@192.168.0.5`

## Container tooling

```bash
# Docker may not publish for a brand-new Ubuntu codename; check before adding the repo
curl -sI https://download.docker.com/linux/ubuntu/dists/$(. /etc/os-release && echo "$VERSION_CODENAME")/Release | head -1
```
404 → substitute the previous LTS codename (`noble`) in the repo line. The packages are
not tied to the release.

```bash
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
  | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt update
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
sudo usermod -aG docker $USER
```

Reconnect the SSH session — group membership only applies to a new login. Then verify
the parts that actually matter:

```bash
docker info | grep -iE 'cgroup version|cgroup driver|storage driver'
```

| Field | Required |
|---|---|
| Cgroup Version | **2** |
| Cgroup Driver | systemd |
| Storage Driver | overlayfs / overlay2 |

**Cgroup version 2 is non-negotiable.** Everything this project claims about OOM kills
and CFS throttling depends on it.

### kind

```bash
curl -Lo ./kind https://kind.sigs.k8s.io/dl/latest/kind-linux-amd64
chmod +x ./kind && sudo mv ./kind /usr/local/bin/kind
kind version
```

> The `dl/latest` endpoint can hand back an **alpha** build — it did here (v0.34.0-alpha).
> For anything you intend to reproduce, install a named release tag instead.

### Sysctls for multi-node kind

Each kubelet consumes inotify instances; three nodes exhaust the default and leave
nodes stuck `NotReady`:

```bash
printf 'fs.inotify.max_user_instances=8192\nfs.inotify.max_user_watches=524288\n' \
  | sudo tee /etc/sysctl.d/99-kind.conf
sudo sysctl --system
```

## Deliberately not installed

`kubectl` is *not* used on the server, even though it is present. All cluster access
goes through the workstation. See [`workstation-setup.md`](workstation-setup.md) and the
rule in `AGENTS.md`.

## Known gaps

- **The kind cluster does not survive a reboot.** The machine comes back; the cluster
  does not. `make cluster-up` is currently part of recovery. Automating this is deferred
  to a later plan.
- **Passwordless sudo is not configured**, so any provisioning step needing root
  requires a human. This is a reasonable trade for a machine reachable from the internet
  via a tailnet.
- **Power:** roughly 60 W idle, about R$35–45/month. Suspend-to-RAM with Wake-on-LAN
  would cut it to ~5 W, at the cost of the machine not being instantly reachable.
