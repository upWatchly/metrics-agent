package collector

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	gnet "github.com/shirou/gopsutil/v4/net"

	"github.com/upwatchly/metrics-agent/internal/client"
)

// Collector gathers system metrics.
type Collector struct {
	prevNetIn  uint64
	prevNetOut uint64
	prevTime   time.Time
}

// New creates a new Collector and takes initial network baseline.
func New() *Collector {
	c := &Collector{}
	c.snapshotNetwork()
	return c
}

func (c *Collector) snapshotNetwork() {
	counters, err := gnet.IOCounters(false)
	if err == nil && len(counters) > 0 {
		c.prevNetIn = counters[0].BytesRecv
		c.prevNetOut = counters[0].BytesSent
	}
	c.prevTime = time.Now()
}

// Collect gathers all system metrics into a MetricsReport.
func (c *Collector) Collect(ctx context.Context) (*client.MetricsReport, error) {
	report := &client.MetricsReport{}

	// Hostname & OS info
	info, err := host.InfoWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("host info: %w", err)
	}
	report.Hostname = info.Hostname
	report.OS = fmt.Sprintf("%s %s", strings.Title(info.Platform), info.PlatformVersion)
	report.KernelVersion = info.KernelVersion
	report.Uptime = info.Uptime

	// CPU usage (1-second sample)
	cpuPercents, err := cpu.PercentWithContext(ctx, time.Second, false)
	if err != nil {
		return nil, fmt.Errorf("cpu percent: %w", err)
	}
	if len(cpuPercents) > 0 {
		report.CPUPercent = round2(cpuPercents[0])
	}

	// Memory
	vmem, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("memory: %w", err)
	}
	report.MemoryPercent = round2(vmem.UsedPercent)
	report.MemoryUsedBytes = vmem.Used
	report.MemoryTotalBytes = vmem.Total

	// Disks
	report.Disks, err = c.collectDisks(ctx)
	if err != nil {
		return nil, fmt.Errorf("disks: %w", err)
	}

	// Network throughput
	now := time.Now()
	counters, err := gnet.IOCountersWithContext(ctx, false)
	if err == nil && len(counters) > 0 {
		elapsed := now.Sub(c.prevTime).Seconds()
		if elapsed > 0 {
			report.NetworkInBytesPS = uint64(float64(counters[0].BytesRecv-c.prevNetIn) / elapsed)
			report.NetworkOutBytesPS = uint64(float64(counters[0].BytesSent-c.prevNetOut) / elapsed)
		}
		c.prevNetIn = counters[0].BytesRecv
		c.prevNetOut = counters[0].BytesSent
		c.prevTime = now
	}

	// Load average
	if runtime.GOOS != "windows" {
		avg, err := load.AvgWithContext(ctx)
		if err == nil {
			report.Load1m = round2(avg.Load1)
			report.Load5m = round2(avg.Load5)
			report.Load15m = round2(avg.Load15)
		}
	}

	return report, nil
}

func (c *Collector) collectDisks(ctx context.Context) ([]client.DiskReport, error) {
	partitions, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		return nil, err
	}

	var disks []client.DiskReport
	for _, p := range partitions {
		usage, err := disk.UsageWithContext(ctx, p.Mountpoint)
		if err != nil || usage.Total == 0 {
			continue
		}
		disks = append(disks, client.DiskReport{
			MountPoint:   p.Mountpoint,
			UsagePercent: round2(usage.UsedPercent),
			TotalBytes:   usage.Total,
			UsedBytes:    usage.Used,
			FreeBytes:    usage.Free,
			Filesystem:   p.Fstype,
		})
	}
	return disks, nil
}

func round2(v float64) float64 {
	return float64(int(v*100)) / 100
}
