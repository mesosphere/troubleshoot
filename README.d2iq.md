# D2iQ fork of Troubleshoot

This is a fork of upstream project https://github.com/replicatedhq/troubleshoot
that allows faster iteration and customizations for diagnostics data collection
project - `dkp-diagnostics`.

Any patch provided to D2iQ fork should be proposed to the upstream project which
would eventually allow using upstream in the `dkp-diagnostics` project.

The fork is based on the upstream tag `v0.13.10` (latest version at the time of
forking the project).

## Changes

The list of changes compared to the upstream project.

### `ExecCopyFromHost` collector

This is a new collector created specifically for gathering host level information
from cluster nodes. The collector allows to run a provided container image in a
privileged mode, as a root user, with additional linux capabilities and with the
host filesystem mounted into the container.

This allows to collect host level information other than copying host level
files that is already possible with `CopyFromHost` collector. Similarly to the
`CopyFromHost` collector the collector runs as a Kubernetes `DaemonSet` executed
on all nodes in the system. The data that are produced by the container are
copied from pre-defined directory into the diagnostics bundle under each
node name. The name of the parent directory in the diagnostics bundle is determined
by the name of the collector specified in its configuration.

The data written into diagnostics bundle look like:

```
<collector-name> / <node-name> / data / (file1|file2|...)
```

Example of a configuration:

```
spec:
  collectors:
    - execCopyFromHost:
        name: node-diagnostics
        image: mesosphere/dkp-diagnostics-node-collector:latest
        timeout: 30s
        command:
          - "/bin/bash"
          - "-c"
          - "/diagnostics/container.sh --hostroot /host --hostpath ${PATH} --outputroot /output"
        workingDir: "/diagnostics"
        includeControlPlane: true
        privileged: true
        capabilities:
          - AUDIT_CONTROL
          - AUDIT_READ
          - BLOCK_SUSPEND
          - BPF
          - CHECKPOINT_RESTORE
          - DAC_READ_SEARCH
          - IPC_LOCK
          - IPC_OWNER
          - LEASE
          - LINUX_IMMUTABLE
          - MAC_ADMIN
          - MAC_OVERRIDE
          - NET_ADMIN
          - NET_BROADCAST
          - PERFMON
          - SYS_ADMIN
          - SYS_BOOT
          - SYS_MODULE
          - SYS_NICE
          - SYS_PACCT
          - SYS_PTRACE
          - SYS_RAWIO
          - SYS_RESOURCE
          - SYS_TIME
          - SYS_TTY_CONFIG
          - SYSLOG
          - WAKE_ALARM
        extractArchive: true
```

Example of the data produced by running this collector:

```
├── node-diagnostics
│   ├── troubleshoot-control-plane
│   │   └── data
│   │       ├── certs_expiration_kubeadm
│   │       ├── containerd_config.toml
                ...
│   │       └── whoami_validate
│   └── troubleshoot-worker
│       └── data
│           ├── containerd_config.toml
│           ├── containers_crictl
                ...
│           └── whoami_validate
```

For more information about the configuration options see the
`ExecCopyFromHost` in the `pkg/apis/troubleshoot/v1beta2/exec_copy_from_host.go`
file.
