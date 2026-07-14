//go:build linux

package collector

import (
	"slices"
	"strings"

	"github.com/shirou/gopsutil/v4/disk"
)

// excludedFstypes are virtual / pseudo / read-only-image filesystems that never
// represent real, fillable storage. Mirrors node_exporter's default fstype
// exclusion set. gopsutil's Partitions(false) already drops most of these, but
// filtering explicitly keeps the behaviour defined here rather than relying on
// gopsutil heuristics — squashfs and overlay in particular can slip through.
var excludedFstypes = map[string]struct{}{
	"autofs": {}, "binfmt_misc": {}, "bpf": {}, "cgroup": {}, "cgroup2": {},
	"configfs": {}, "debugfs": {}, "devpts": {}, "devtmpfs": {}, "fusectl": {},
	"hugetlbfs": {}, "iso9660": {}, "mqueue": {}, "nsfs": {}, "overlay": {},
	"proc": {}, "procfs": {}, "pstore": {}, "rpc_pipefs": {}, "securityfs": {},
	"selinuxfs": {}, "squashfs": {}, "sysfs": {}, "tmpfs": {}, "tracefs": {},
}

// excludedMountPrefixes are mount trees that hold only virtual, container, or
// package-manager mounts — never a disk a user cares about filling up. A path is
// dropped if it equals a prefix or sits under it (prefix + "/").
var excludedMountPrefixes = []string{
	"/proc", "/sys", "/dev", "/run", "/snap",
	"/var/lib/docker", "/var/lib/containers/storage",
}

// keepPartition keeps only real, writable local filesystems on Linux. It drops
// virtual/pseudo filesystems (by fstype), container/OS mount trees (by path),
// and read-only mounts — a read-only filesystem can't fill up, so it has no
// alerting value and only adds noise (e.g. a 95%-full squashfs /snap image or
// a read-only /boot).
func keepPartition(p disk.PartitionStat) bool {
	if _, bad := excludedFstypes[p.Fstype]; bad {
		return false
	}
	for _, prefix := range excludedMountPrefixes {
		if p.Mountpoint == prefix || strings.HasPrefix(p.Mountpoint, prefix+"/") {
			return false
		}
	}
	// Read-only mount → can't fill up → nothing to alert on.
	if slices.Contains(p.Opts, "ro") {
		return false
	}
	return true
}
