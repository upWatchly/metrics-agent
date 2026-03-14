package collector

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	gnet "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
	log "github.com/sirupsen/logrus"

	"github.com/upwatchly/metrics-agent/internal/client"
)

const (
	maxProcesses = 50
	diskTimeout  = 2 * time.Second
)

// Collector gathers system metrics.
type Collector struct {
	// Network state protected by netMu
	netMu      sync.Mutex
	prevNetIn  uint64
	prevNetOut uint64
	prevTime   time.Time

	ipv4 string
	ipv6 string

	// Cached static info
	hostname      string
	os            string
	kernelVersion string
	cpuCores      int

	// Cached slow data (updated in background)
	cachedDisks      []client.DiskReport
	cachedContainers []client.DockerContainer
	cacheMu          sync.RWMutex

	// Slow collector guard
	slowMu      sync.Mutex
	slowRunning bool
}

// New creates a new Collector, detects public IPs and takes initial network baseline.
func New() *Collector {
	c := &Collector{}
	c.detectPublicIPs()
	c.cacheStaticInfo()
	c.snapshotNetwork()
	return c
}

func (c *Collector) cacheStaticInfo() {
	info, err := host.Info()
	if err != nil {
		log.WithError(err).Warn("failed to get host info")
		return
	}
	c.hostname = info.Hostname
	platform := info.Platform
	if len(platform) > 0 {
		platform = strings.ToUpper(platform[:1]) + platform[1:]
	}
	c.os = fmt.Sprintf("%s %s", platform, info.PlatformVersion)
	c.kernelVersion = info.KernelVersion

	cores, err := cpu.Counts(true)
	if err == nil {
		c.cpuCores = cores
	}
}

func (c *Collector) detectPublicIPs() {
	var wg sync.WaitGroup
	var ipv4, ipv6 string

	wg.Add(2)
	go func() {
		defer wg.Done()
		ipv4 = fetchIP("https://api4.ipify.org")
	}()
	go func() {
		defer wg.Done()
		ipv6 = fetchIP("https://api6.ipify.org")
	}()
	wg.Wait()

	c.ipv4 = ipv4
	c.ipv6 = ipv6
	log.WithFields(log.Fields{"ipv4": c.ipv4, "ipv6": c.ipv6}).Info("detected public IPs")
}

func fetchIP(url string) string {
	cl := &http.Client{Timeout: 5 * time.Second}
	resp, err := cl.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256))
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
// If liveMode is true, also collects processes and docker containers.
func (c *Collector) Collect(ctx context.Context, liveMode bool) (*client.MetricsReport, error) {
	report := &client.MetricsReport{
		Hostname:      c.hostname,
		PublicIPv4:    c.ipv4,
		PublicIPv6:    c.ipv6,
		OS:            c.os,
		KernelVersion: c.kernelVersion,
		CPUCores:      c.cpuCores,
	}

	// Uptime (cheap — reads /proc/uptime)
	info, err := host.InfoWithContext(ctx)
	if err == nil {
		report.Uptime = info.Uptime
	}

	if liveMode {
		return c.collectLive(ctx, report)
	}
	return c.collectNormal(ctx, report)
}

func (c *Collector) collectNormal(ctx context.Context, report *client.MetricsReport) (*client.MetricsReport, error) {
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

	// Network
	c.collectNetwork(report)

	// Load average
	c.collectLoad(ctx, report)

	return report, nil
}

// StartSlowCollector runs a background loop that refreshes disks and docker data.
// Call this when entering live mode. Safe to call multiple times — only one instance runs.
func (c *Collector) StartSlowCollector(ctx context.Context) {
	c.slowMu.Lock()
	if c.slowRunning {
		c.slowMu.Unlock()
		return
	}
	c.slowRunning = true
	c.slowMu.Unlock()

	go func() {
		defer func() {
			c.slowMu.Lock()
			c.slowRunning = false
			c.slowMu.Unlock()
		}()

		// Initial collection inside goroutine to avoid blocking the caller
		c.refreshSlowData(ctx)

		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
				c.refreshSlowData(ctx)
			}
		}
	}()
}

func (c *Collector) refreshSlowData(ctx context.Context) {
	var wg sync.WaitGroup
	var disks []client.DiskReport
	var containers []client.DockerContainer

	wg.Add(1)
	go func() {
		defer wg.Done()
		d, err := c.collectDisks(ctx)
		if err != nil {
			log.WithError(err).Debug("slow collector: failed to collect disks")
		} else {
			disks = d
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		containers = c.collectDockerContainers(ctx)
	}()

	wg.Wait()

	// Only update cache with non-nil data to preserve previous values on transient errors
	c.cacheMu.Lock()
	if disks != nil {
		c.cachedDisks = disks
	}
	if containers != nil {
		c.cachedContainers = containers
	}
	c.cacheMu.Unlock()
}

func (c *Collector) collectLive(ctx context.Context, report *client.MetricsReport) (*client.MetricsReport, error) {
	var wg sync.WaitGroup
	var cpuPct float64
	var vmem *mem.VirtualMemoryStat
	var procs []client.Process
	var cpuErr, memErr error

	// CPU — 500ms sample
	wg.Add(1)
	go func() {
		defer wg.Done()
		pcts, err := cpu.PercentWithContext(ctx, 500*time.Millisecond, false)
		if err != nil {
			cpuErr = err
			return
		}
		if len(pcts) > 0 {
			cpuPct = pcts[0]
		}
	}()

	// Memory
	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		vmem, err = mem.VirtualMemoryWithContext(ctx)
		if err != nil {
			memErr = err
		}
	}()

	// Processes
	wg.Add(1)
	go func() {
		defer wg.Done()
		procs = c.collectProcesses(ctx)
	}()

	wg.Wait()

	if cpuErr != nil {
		return nil, fmt.Errorf("cpu percent: %w", cpuErr)
	}
	if memErr != nil {
		return nil, fmt.Errorf("memory: %w", memErr)
	}

	report.CPU = round2(cpuPct)
	report.Memory = client.Memory{
		UsedBytes:  vmem.Used,
		TotalBytes: vmem.Total,
	}

	// Use cached slow data
	c.cacheMu.RLock()
	report.Disks = c.cachedDisks
	report.DockerContainers = c.cachedContainers
	c.cacheMu.RUnlock()

	report.Processes = procs

	// Network
	c.collectNetwork(report)

	// Load
	c.collectLoad(ctx, report)

	return report, nil
}

func (c *Collector) collectNetwork(report *client.MetricsReport) {
	now := time.Now()
	counters, err := gnet.IOCounters(false)
	if err != nil || len(counters) == 0 {
		return
	}

	c.netMu.Lock()
	defer c.netMu.Unlock()

	curIn := counters[0].BytesRecv
	curOut := counters[0].BytesSent
	elapsed := now.Sub(c.prevTime).Seconds()

	// Guard against counter reset (reboot, interface reset)
	if elapsed > 0 && curIn >= c.prevNetIn && curOut >= c.prevNetOut {
		report.Network = client.Network{
			InBytesPerSec:  uint64(float64(curIn-c.prevNetIn) / elapsed),
			OutBytesPerSec: uint64(float64(curOut-c.prevNetOut) / elapsed),
		}
	}

	c.prevNetIn = curIn
	c.prevNetOut = curOut
	c.prevTime = now
}

func (c *Collector) collectLoad(ctx context.Context, report *client.MetricsReport) {
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
}

func (c *Collector) collectDisks(ctx context.Context) ([]client.DiskReport, error) {
	partitions, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		return nil, err
	}

	disks := make([]client.DiskReport, 0, len(partitions))
	for _, p := range partitions {
		// Per-disk timeout to avoid hanging on stale NFS mounts
		diskCtx, cancel := context.WithTimeout(ctx, diskTimeout)
		usage, err := disk.UsageWithContext(diskCtx, p.Mountpoint)
		cancel()
		if err != nil {
			log.WithFields(log.Fields{
				"mount": p.Mountpoint,
			}).WithError(err).Debug("skipping disk (timeout or error)")
			continue
		}
		if usage.Total == 0 {
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

func (c *Collector) collectProcesses(ctx context.Context) []client.Process {
	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		log.WithError(err).Warn("failed to list processes")
		return nil
	}

	// First pass: get only Name and lightweight CPU/Memory percent
	type procInfo struct {
		proc   *process.Process
		cpuPct float64
		memPct float32
		name   string
	}

	candidates := make([]procInfo, 0, len(procs))
	for _, p := range procs {
		name, _ := p.NameWithContext(ctx)
		cpuPct, _ := p.CPUPercentWithContext(ctx)
		memPct, _ := p.MemoryPercentWithContext(ctx)

		if cpuPct < 0.1 && memPct < 0.1 {
			continue
		}

		candidates = append(candidates, procInfo{
			proc:   p,
			cpuPct: cpuPct,
			memPct: memPct,
			name:   name,
		})
	}

	// Sort by CPU+Memory descending, take top N
	sort.Slice(candidates, func(i, j int) bool {
		scoreI := candidates[i].cpuPct + float64(candidates[i].memPct)
		scoreJ := candidates[j].cpuPct + float64(candidates[j].memPct)
		return scoreI > scoreJ
	})
	if len(candidates) > maxProcesses {
		candidates = candidates[:maxProcesses]
	}

	// Second pass: get detailed info only for top processes
	result := make([]client.Process, 0, len(candidates))
	for _, cand := range candidates {
		memInfo, _ := cand.proc.MemoryInfoWithContext(ctx)
		user, _ := cand.proc.UsernameWithContext(ctx)
		cmdline, _ := cand.proc.CmdlineWithContext(ctx)

		var memBytes uint64
		if memInfo != nil {
			memBytes = memInfo.RSS
		}

		result = append(result, client.Process{
			PID:           cand.proc.Pid,
			Name:          cand.name,
			CPUPercent:    round2(cand.cpuPct),
			MemoryPercent: round2(float64(cand.memPct)),
			MemoryBytes:   memBytes,
			User:          user,
			Command:       cmdline,
		})
	}
	return result
}

func (c *Collector) collectDockerContainers(ctx context.Context) []client.DockerContainer {
	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		return nil
	}

	out, err := exec.CommandContext(ctx, dockerPath, "ps", "-a",
		"--format", "{{.ID}}\t{{.Names}}\t{{.Image}}\t{{.Status}}\t{{.State}}\t{{.Ports}}",
	).Output()
	if err != nil {
		log.WithError(err).Debug("failed to list docker containers")
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return nil
	}

	statsOut, err := exec.CommandContext(ctx, dockerPath, "stats", "--no-stream",
		"--format", "{{.ID}}\t{{.CPUPerc}}\t{{.MemUsage}}",
	).Output()
	if err != nil {
		log.WithError(err).Debug("failed to get docker stats")
	}
	statsMap := parseDockerStats(string(statsOut))

	containers := make([]client.DockerContainer, 0, len(lines))
	for _, line := range lines {
		parts := strings.SplitN(line, "\t", 6)
		if len(parts) < 6 {
			continue
		}

		id := parts[0]
		shortID := id
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}
		stats := statsMap[id]

		var ports []string
		if parts[5] != "" {
			for _, p := range strings.Split(parts[5], ", ") {
				trimmed := strings.TrimSpace(p)
				if trimmed != "" {
					ports = append(ports, trimmed)
				}
			}
		}

		containers = append(containers, client.DockerContainer{
			ID:          shortID,
			Name:        parts[1],
			Image:       parts[2],
			Status:      parts[3],
			State:       parts[4],
			CPUPercent:  stats.cpu,
			MemoryUsage: stats.memUsage,
			MemoryLimit: stats.memLimit,
			Ports:       ports,
		})
	}
	return containers
}

type containerStats struct {
	cpu      float64
	memUsage uint64
	memLimit uint64
}

func parseDockerStats(output string) map[string]containerStats {
	result := make(map[string]containerStats)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		cpuStr := strings.TrimSuffix(parts[1], "%")
		var cpuVal float64
		if _, err := fmt.Sscanf(cpuStr, "%f", &cpuVal); err != nil {
			log.WithFields(log.Fields{
				"raw": parts[1],
			}).WithError(err).Debug("failed to parse docker CPU percent")
		}

		memUsage, memLimit := parseMemUsage(parts[2])

		result[parts[0]] = containerStats{
			cpu:      round2(cpuVal),
			memUsage: memUsage,
			memLimit: memLimit,
		}
	}
	return result
}

func parseMemUsage(s string) (uint64, uint64) {
	parts := strings.Split(s, " / ")
	if len(parts) != 2 {
		return 0, 0
	}
	return parseByteSize(strings.TrimSpace(parts[0])), parseByteSize(strings.TrimSpace(parts[1]))
}

func parseByteSize(s string) uint64 {
	var val float64
	var unit string
	fmt.Sscanf(s, "%f%s", &val, &unit)
	switch strings.ToLower(unit) {
	case "b":
		return uint64(val)
	case "kib":
		return uint64(val * 1024)
	case "mib":
		return uint64(val * 1024 * 1024)
	case "gib":
		return uint64(val * 1024 * 1024 * 1024)
	case "tib":
		return uint64(val * 1024 * 1024 * 1024 * 1024)
	}
	return 0
}

func round2(v float64) float64 {
	return float64(int(v*100)) / 100
}
