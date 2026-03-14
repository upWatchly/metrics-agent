package client

// MetricsReport is the payload sent to the backend each cycle.
type MetricsReport struct {
	ServerID       string       `json:"serverId,omitempty"`
	OrganizationID string       `json:"organizationId,omitempty"`
	Hostname       string       `json:"hostname"`
	PublicIPv4     string       `json:"publicIpv4,omitempty"`
	PublicIPv6     string       `json:"publicIpv6,omitempty"`
	OS             string       `json:"os"`
	KernelVersion  string       `json:"kernelVersion"`
	CPU            float64      `json:"cpu"`
	CPUCores       int          `json:"cpuCores"`
	Memory         Memory       `json:"memory"`
	Disks          []DiskReport `json:"disks"`
	Network        Network      `json:"network"`
	Load           Load         `json:"load"`
	Uptime         uint64       `json:"uptime"`

	// Live mode extended data
	Processes        []Process         `json:"processes,omitempty"`
	DockerContainers []DockerContainer `json:"dockerContainers,omitempty"`
}

type Memory struct {
	UsedBytes  uint64 `json:"usedBytes"`
	TotalBytes uint64 `json:"totalBytes"`
}

type DiskReport struct {
	MountPoint string `json:"mountPoint"`
	UsedBytes  uint64 `json:"usedBytes"`
	TotalBytes uint64 `json:"totalBytes"`
}

type Network struct {
	InBytesPerSec  uint64 `json:"inBytesPerSec"`
	OutBytesPerSec uint64 `json:"outBytesPerSec"`
}

type Load struct {
	Load1m  float64 `json:"load1m"`
	Load5m  float64 `json:"load5m"`
	Load15m float64 `json:"load15m"`
}

type Process struct {
	PID           int32   `json:"pid"`
	Name          string  `json:"name"`
	CPUPercent    float64 `json:"cpuPercent"`
	MemoryPercent float64 `json:"memoryPercent"`
	MemoryBytes   uint64  `json:"memoryBytes"`
	User          string  `json:"user"`
	Command       string  `json:"command"`
}

type DockerContainer struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Image       string   `json:"image"`
	Status      string   `json:"status"`
	State       string   `json:"state"`
	CPUPercent  float64  `json:"cpuPercent"`
	MemoryUsage uint64   `json:"memoryUsage"`
	MemoryLimit uint64   `json:"memoryLimit"`
	Ports       []string `json:"ports"`
}

// ServerConfig is the configuration returned by the backend.
type ServerConfig struct {
	ReportInterval int    `json:"reportInterval"`
	LiveMode       bool   `json:"liveMode"`
	LiveInterval   int    `json:"liveInterval,omitempty"`
	ServerID       string `json:"serverId"`
	OrganizationID string `json:"organizationId"`
}
