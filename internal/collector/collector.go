package collector

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	gnet "github.com/shirou/gopsutil/v4/net"
	log "github.com/sirupsen/logrus"

	"github.com/upwatchly/metrics-agent/internal/client"
)

// Collector gathers system metrics.
type Collector struct {
	prevNetIn  uint64
	prevNetOut uint64
	prevTime   time.Time
	ipv4       string
	ipv6       string
}

// New creates a new Collector, detects public IPs and takes initial network baseline.
func New() *Collector {
	c := &Collector{}
	c.detectPublicIPs()
	c.snapshotNetwork()
	return c
}

func (c *Collector) detectPublicIPs() {
	c.ipv4 = fetchIP("https://api4.ipify.org")
	c.ipv6 = fetchIP("https://api6.ipify.org")
	log.WithFields(log.Fields{"ipv4": c.ipv4, "ipv6": c.ipv6}).Info("detected public IPs")
}

func fetchIP(url string) string {
	cl := &http.Client{Timeout: 5 * time.Second}
	resp, err := cl.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(body))
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
	report.PublicIPv4 = c.ipv4
	report.PublicIPv6 = c.ipv6
	report.OS = fmt.Sprintf("%s %s", strings.Title(info.Platform), info.PlatformVersion)
	report.KernelVersion = info.KernelVersion
	report.Uptime = info.Uptime

	// CPU cores (logical)
	cores, err := cpu.CountsWithContext(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("cpu cores: %w", err)
	}
	report.CPUCores = cores

	// CPU usage (1-second sample)
	cpuPercents, err := cpu.PercentWithContext(ctx, time.Second, false)
	if err != nil {
		return nil, fmt.Errorf("cpu percent: %w", err)
	}
	if len(cpuPercents) > 0 {
		report.CPU = round2(cpuPercents[0])
	}

	// Memory
	vmem, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("memory: %w", err)
	}
	report.Memory = client.Memory{
		UsedBytes:  vmem.Used,
		TotalBytes: vmem.Total,
	}

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
			report.Network = client.Network{
				InBytesPerSec:  uint64(float64(counters[0].BytesRecv-c.prevNetIn) / elapsed),
				OutBytesPerSec: uint64(float64(counters[0].BytesSent-c.prevNetOut) / elapsed),
			}
		}
		c.prevNetIn = counters[0].BytesRecv
		c.prevNetOut = counters[0].BytesSent
		c.prevTime = now
	}

	// Load average
	if runtime.GOOS != "windows" {
		avg, err := load.AvgWithContext(ctx)
		if err == nil {
			report.Load = client.Load{
				Load1m:  round2(avg.Load1),
				Load5m:  round2(avg.Load5),
				Load15m: round2(avg.Load15),
			}
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
			MountPoint: p.Mountpoint,
			UsedBytes:  usage.Used,
			TotalBytes: usage.Total,
		})
	}
	return disks, nil
}

func round2(v float64) float64 {
	return float64(int(v*100)) / 100
}
