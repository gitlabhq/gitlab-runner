#!/bin/sh
set -eu

# 0. Make the root mount tree private. pivot_root(2) returns EINVAL if the
# new root's parent mount has shared (MS_SHARED) propagation, which the
# container runtime (containerd/CRI-O) does not guarantee against. Recursively
# switching to private propagation prevents that failure. See pivot_root(2).
mount --make-rprivate /

# 1. Create overlay directory tree
#
# persist_dir is the mount path of the suspend PVC. It must match the Go-side
# suspendPVCMountPath constant (executors/kubernetes/suspend.go) — the runner
# passes it via CI_SUSPEND_PERSIST_DIR; the /gl_persist fallback only applies
# if the script is ever run without that env var set.
persist_dir="${CI_SUSPEND_PERSIST_DIR:-/gl_persist}"
mkdir -p "$persist_dir/upper" "$persist_dir/work" "$persist_dir/merged"

# 2. Check for image drift and persist current image tag
if [ -n "${CI_SUSPEND_IMAGE_TAG:-}" ]; then
        if [ -f "$persist_dir/.image-tag" ]; then
                stored=$(cat "$persist_dir/.image-tag")
                if [ "$stored" != "$CI_SUSPEND_IMAGE_TAG" ]; then
                        echo "warning: image tag changed from '$stored' to '$CI_SUSPEND_IMAGE_TAG' — drift may cause failures" >&2
                fi
        fi
        printf '%s' "$CI_SUSPEND_IMAGE_TAG" >"$persist_dir/.image-tag"
fi

# 3. Mount overlayfs
mount -t overlay overlay \
        -o "lowerdir=/,upperdir=$persist_dir/upper,workdir=$persist_dir/work" \
        "$persist_dir/merged"

# 4. Bind-mount kernel pseudo-filesystems
mount --rbind /proc "$persist_dir/merged/proc"
mount --rbind /sys "$persist_dir/merged/sys"
mount --rbind /dev "$persist_dir/merged/dev"

# 5. Bind-mount kubelet networking files
mount --bind /etc/resolv.conf "$persist_dir/merged/etc/resolv.conf"
mount --bind /etc/hosts "$persist_dir/merged/etc/hosts"
mount --bind /etc/hostname "$persist_dir/merged/etc/hostname"

# 6. These lines are needed to see the original persist directory after pivot_root is called.
# This is benefical for debugging the .image-tag file and other service container PVCs.
mkdir -p "$persist_dir/merged$persist_dir"
mount --bind "$persist_dir" "$persist_dir/merged$persist_dir"

# 7. Bind-mount paths to bypass the overlay. overlayfs (lowerdir=/) doesn't
# cross mount points, so any kubelet submount left un-rebound here — builds/
# scripts/logs or a configured volume — becomes an empty dir under merged/:
# reads see nothing, writes silently land in the overlay layer. No fallback
# if the var is unset: this script ships in lockstep with the runner binary
# that sets it.
if [ -n "${CI_SUSPEND_REBIND_PATHS:-}" ]; then
        printf '%s\n' "$CI_SUSPEND_REBIND_PATHS" | while IFS= read -r d; do
                [ -n "$d" ] && [ -d "$d" ] && mkdir -p "$persist_dir/merged$d" && mount --bind "$d" "$persist_dir/merged$d"
        done
fi

# 8. pivot_root into the overlay
mkdir -p "$persist_dir/merged/.old_root"
pivot_root "$persist_dir/merged" "$persist_dir/merged/.old_root"
cd /

# 9. Lazily unmount the old root. A lazy unmount detaches the mount point
# immediately but keeps the underlying filesystem accessible until every open
# file descriptor referencing it is closed, avoiding EBUSY from FDs still open
# in the old root at this point.
umount -l /.old_root 2>/dev/null || true

# 10. Execute the provided command
exec "$@"
