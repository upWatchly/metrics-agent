package client

// MetricsReport is the payload sent to the backend each cycle.
type MetricsReport struct {
	Hostname           string       `json:"hostname"`
	OS                 string       `json:"os"`
	KernelVersion      string       `json:"kernelVersion"`
	CPUPercent         float64      `json:"cpuPercent"`
	MemoryPercent      float64      `json:"memoryPercent"`
	MemoryUsedBytes    uint64       `json:"memoryUsedBytes"`
	MemoryTotalBytes   uint64       `json:"memoryTotalBytes"`
	Disks              []DiskReport `json:"disks"`
	NetworkInBytesPS   uint64       `json:"networkInBytesPerSec"`
	NetworkOutBytesPS  uint64       `json:"networkOutBytesPerSec"`
	Load1m             float64      `json:"load1m"`
	Load5m             float64      `json:"load5m"`
	Load15m            float64      `json:"load15m"`
	Uptime             uint64       `json:"uptime"`
}

// DiskReport represents usage for a single mount point.
type DiskReport struct {
	MountPoint   string  `json:"mountPoint"`
	UsagePercent float64 `json:"usagePercent"`
	TotalBytes   uint64  `json:"totalBytes"`
	UsedBytes    uint64  `json:"usedBytes"`
	FreeBytes    uint64  `json:"freeBytes"`
	Filesystem   string  `json:"filesystem"`
}

// ServerConfig is the configuration returned by the backend.
type ServerConfig struct {
	ReportInterval int        `json:"reportInterval"`
	GracePeriod    int        `json:"gracePeriod"`
	Thresholds     Thresholds `json:"thresholds"`
}

// Thresholds defines alerting thresholds.
type Thresholds struct {
	CPUPercent         float64 `json:"cpuPercent"`
	CPUDurationMinutes int     `json:"cpuDurationMinutes"`
	RAMPercent         float64 `json:"ramPercent"`
	DiskPercent        float64 `json:"diskPercent"`
}
