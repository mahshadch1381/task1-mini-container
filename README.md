# Mini Container

A minimal container runtime written in Go that creates Linux namespace
isolation using chroot, UTS namespaces, and PID namespaces.

## Features

- Accepts a folder as the new root filesystem (e.g. ./new-root)
- Verifies /bin/sh exists inside the rootfs
- Uses chroot to lock the process inside the new root
- Sets a custom hostname via UTS namespace isolation
- Hides host processes via PID namespace isolation
- Mounts /proc inside the container automatically
- Runs any command inside the isolated environment
- Passes parent environment variables through
- Cleans up by unmounting /proc on exit
- Makefile for build/run/test/clean
- Automated test script (test.sh) for validation

## Requirements

- Go 1.20+
- Linux (kernel 4.x+ recommended)
- sudo / root access
- A minimal rootfs in ./new-root containing /bin/sh

## Quick Rootfs Setup

mkdir -p new-root
docker export $(docker create alpine) | tar -C new-root -xf -

Or use an existing Alpine/Busybox rootfs.

## Usage

Build:
  make build

Run a specific command:
  go mod init mahshad-container
  make build
  sudo ./mahshad-container --rootfs ./new-root /bin/ls -la /

With custom hostname:
  sudo ./mahshad-container --rootfs ./new-root --hostname mybox /bin/sh

## Flags

--rootfs     Path to the root filesystem (required)
--hostname   Hostname inside the container (default: container)

## Makefile Targets

make build    Compile the binary 

## Tests

Run:
  make test

What it checks:
  Test 1 - /bin/sh runs inside the isolated rootfs
  Test 2 - Hostname inside is different from the host
  Test 3 - /proc is mounted and readable
  Test 4 - PID namespace hides host process list

Expected output:

==> Building...
==> Test 1: /bin/sh runs inside the isolated rootfs
  PASS: /bin/sh ran
==> Test 2: hostname is different inside
  host=myhost  inside=mahshad
  PASS: hostname inside is 'mahshad'
==> Test 3: /proc is mounted inside
  PASS: /proc is mounted
==> Test 4: PID namespace hides host processes
  processes visible inside: 3
  PASS: only 3 processes visible (host list is hidden)

ALL TESTS PASSED

## Limitations

- No network namespace (shares host network)
- No cgroups (no resource limits)
- No user namespace (root in container = root on host)
- Rootfs must be provided separately
- Linux only

