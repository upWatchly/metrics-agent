//go:build windows

package collector

import (
	"strings"

	"github.com/shirou/gopsutil/v4/disk"
	"golang.org/x/sys/windows"
)

// keepPartition reports whether a Windows volume should be measured. gopsutil's
// disk.Partitions returns *every* logical drive — including the CD/DVD drive,
// removable media, RAM disks, and mapped network drives — and disk.Usage on
// those reports the wrong capacity (e.g. a DVD's size instead of the system
// disk) or blocks on a disconnected share. We keep only DRIVE_FIXED: local hard
// disks and SSDs.
func keepPartition(p disk.PartitionStat) bool {
	root := p.Mountpoint
	if !strings.HasSuffix(root, `\`) {
		root += `\` // GetDriveType wants a root path like "C:\"
	}
	ptr, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return false
	}
	return windows.GetDriveType(ptr) == windows.DRIVE_FIXED
}
