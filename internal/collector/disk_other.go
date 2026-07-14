//go:build !windows

package collector

// keepPartition keeps every partition on non-Windows platforms: disk.Partitions
// with all=false already returns only physical mounts there, so there is nothing
// extra to filter out.
func keepPartition(mountpoint string) bool { return true }
