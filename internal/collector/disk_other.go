//go:build !windows && !linux

package collector

import "github.com/shirou/gopsutil/v4/disk"

// keepPartition keeps every partition on platforms without a specific filter
// (e.g. macOS): disk.Partitions with all=false already returns only physical
// mounts there, so there is nothing extra to drop.
func keepPartition(_ disk.PartitionStat) bool { return true }
