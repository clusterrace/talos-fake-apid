# talos-fake-apid

A minimal stand-in for Talos `apid` so `talosctl upgrade-k8s` can run against a
cluster that mixes Talos nodes with non-Talos (Ubuntu) nodes.

## The problem

`talosctl -n <controlplane> upgrade-k8s` discovers every node in the cluster
(control plane + worker) and proxies API calls to each one through the
controlplane's `apid`. Proxying to a node that doesn't run `apid` fails with:

```
rpc error: code = Unavailable desc = connection error: desc = "transport:
  Error while dialing: dial tcp <ubuntu-node>:50000: connect: connection refused"
```

This service listens on `:50000` on the non-Talos node and answers the small
subset of RPCs `upgrade-k8s` needs from a worker, so the command can complete.

## What it mocks

`upgrade-k8s` (worker-node path) hits three things on each worker. The fake:

| Service / method                                            | Behavior                                                                                     |
|-------------------------------------------------------------|----------------------------------------------------------------------------------------------|
| `cosi.resource.State/Get` — `k8s/KubeletSpec/kubelet`       | Returns a `KubeletSpec` with `Image = ghcr.io/siderolabs/kubelet:v<current>`.                |
| `cosi.resource.State/Get` — `config/MachineConfig/v1alpha1` | Returns a real worker's `v1alpha1` config (loaded from `machine-config.yaml`).               |
| `cosi.resource.State/Watch` — `runtime/Service/kubelet`     | Sends one `Created` event with `Running=true, Healthy=true`, then holds the stream open.     |
| `machine.MachineService/Version`                            | Returns the Talos tag from `-talos-version` so the v1.11+ `upgrade-k8s` compat check passes. |
| `machine.MachineService/ImagePull`                          | Returns `Unimplemented`. `upgrade-k8s` already handles this with "not implemented, skipping". |
| Everything else                                             | `Unimplemented`.                                                                              |

The state is served by `cosi-project/runtime`'s in-memory state plus its
`protobuf/server` gRPC adapter, so the wire format matches real `apid` exactly.

## Scope

- Tested only for `talosctl upgrade-k8s --dry-run`. A real (non-dry-run)
  upgrade would call `MachineService.ApplyConfiguration`, which the fake does
  not implement.
- The fake only covers a *worker* node's responsibilities. It will not pretend
  to be a control-plane node.
- The fake never actually upgrades anything on the host. Whatever runs the
  kubelet on the Ubuntu node still needs to be upgraded out-of-band.

## TLS

The Talos control-plane `apid` proxies to workers using mTLS, validating both
peers against the cluster's OS root CA. The fake:

1. Reads the OS root CA cert + private key (`ca.crt` / `ca.key`).
2. At startup, generates an ED25519 server cert signed by that CA with the
   worker's IP/hostname in the SAN.
3. Requires and verifies client certs against the same CA.

The CA material is fetched via `talosctl get osrootsecret os -o yaml`.

## One-time setup

Run on a workstation that already has a working `talosctl` (admin access):

```bash
make fetch-ca       # writes ca.crt + ca.key (issuing CA + private key)
make fetch-config   # writes machine-config.yaml (worker config template)
```

`fetch-config` pulls the machine config from `192.168.0.20` by default; any
real worker in the cluster will do. The resulting YAML is only used to seed
the `MachineConfig` resource the fake returns, so it doesn't have to match
the Ubuntu host's actual hostname/IP — `upgrade-k8s --dry-run` just needs it
to parse as a valid `v1alpha1` config.

> `ca.key`, `machine-config.yaml`, and any generated `server.*` files are
> cluster secrets. The `.gitignore` excludes them; keep them off shared
> storage.

## Build and deploy

```bash
make build          # cross-compiles for linux/arm64 (Raspberry Pi worker)
make deploy         # scp binary + ca.* + machine-config.yaml to the target
make run-remote     # sudo nohup the binary on the target
make logs           # tail the remote server log
make stop-remote    # kill the remote process
```

Defaults target `ubuntu@192.168.0.23:/home/ubuntu/talos-fake-apid`. Override
via `make deploy TARGET=192.168.0.x TARGET_USER=foo TARGET_DIR=/path`.

## Validating

After the fake is running on the worker, the dry-run should reach every node:

```bash
talosctl -n 192.168.0.10 upgrade-k8s --to <version> --dry-run
```

Expected fragment for the Ubuntu node:

```
> "192.168.0.23": pre-pulling ghcr.io/siderolabs/kubelet:v<version>
< "192.168.0.23": not implemented, skipping
...
> "192.168.0.23": starting update
> update kubelet: <current> -> <version>
> skipped in dry-run
```

## Flags

```
-listen          0.0.0.0:50000                          listen address
-ca-cert         ca.crt                                 OS root CA cert PEM
-ca-key          ca.key                                 OS root CA private key PEM
-node-ip         192.168.0.23                           IP for server cert SAN
-hostname        ares-worker-4                          server cert CN + DNS SAN
-machine-config  machine-config.yaml                    seed v1alpha1 config YAML
-kubelet-image   ghcr.io/siderolabs/kubelet:v1.33.12    image to advertise in KubeletSpec
-talos-version   v1.12.7                                tag to advertise in MachineService.Version (compat check)
```

The `-kubelet-image` value is what `upgrade-k8s` reads as the worker's
"current" kubelet, so it should reflect the kubelet version actually running
on the Ubuntu host (otherwise the dry-run output is misleading; the upgrade
itself is still a no-op).

## Layout

```
main.go              entrypoint, TLS, state seeding, gRPC server
Makefile             build / fetch-ca / fetch-config / deploy / run-remote
ca.crt, ca.key       OS root CA (gitignored)
machine-config.yaml  seed v1alpha1 worker config (gitignored)
```
