# prunesh/kubectl

Token-reduction plugin for [prunesh](https://github.com/prunesh/prunesh) that filters `kubectl` output.

`kubectl describe pod` can produce 80–200 lines of metadata, hashes, and volume details that rarely matter for an AI coding session. `kubectl get -o yaml` embeds `managedFields` blocks that can add 100+ lines of noise. This plugin strips the noise and keeps what's actionable.

## What it filters

| Subcommand | Action |
|---|---|
| `describe` | Drops Annotations, Volumes, Tolerations, QoS Class, Node-Selectors, Priority. Within Containers: drops Container ID, Image ID, Host Port, Mounts. Keeps Name, Namespace, Status, Node, Containers (Image/State/Ready/Restart Count), Conditions, Events. |
| `get -o yaml` | Strips `managedFields` block, `resourceVersion`, `uid`, `generation`, `creationTimestamp`, `selfLink`. |
| `get -o json` | Same fields as `-o yaml`. |
| `logs` | Rewrite adds `--tail=100` when no `--tail` or `-f` flag is present. |
| Everything else | Full passthrough (`get` table, `apply`, `delete`, `create`, `rollout`, …). |

### Example — describe pod

**Before** (excerpt from a real `kubectl describe pod`):
```
Annotations:  kubectl.kubernetes.io/last-applied-configuration: {"apiVersion":"v1","kind":"Pod",...}
...
Volumes:
  kube-api-access:
    Type:                    Projected
    TokenExpirationSeconds:  3607
    ConfigMapName:           kube-root-ca.crt
QoS Class:                   BestEffort
Node-Selectors:              <none>
Tolerations:                 node.kubernetes.io/not-ready:NoExecute op=Exists for 300s
                             node.kubernetes.io/unreachable:NoExecute op=Exists for 300s
```

**After**: those sections are gone. Events and Conditions stay — they are the first thing to check when debugging.

## Install

Requires [prunesh core](https://github.com/prunesh/prunesh) >= 0.12.0.

```bash
prunesh plugin install github.com/prunesh/kubectl@v0.1.0
```

To replace an existing `kubectl` plugin:

```bash
prunesh plugin install github.com/prunesh/kubectl@v0.1.0 --replace
```

## Uninstall

```bash
prunesh plugin uninstall prunesh/kubectl
```

## How it works

- **Rewrite**: adds `--tail=100` to `kubectl logs` when no tail limit or follow flag is present, limiting output at the source before any filtering.
- **FilterOutput**: section-aware heuristic filter for `describe`; block-skip filter for `-o yaml`/`-o json`.

## License

MIT
