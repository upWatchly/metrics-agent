package client

// MetricsReport is the payload sent to the backend each cycle.
type MetricsReport struct {
	Hostname      string       `json:"hostname"`
	PublicIPv4    string       `json:"publicIpv4,omitempty"`
	PublicIPv6    string       `json:"publicIpv6,omitempty"`
	OS            string       `json:"os"`
	KernelVersion string       `json:"kernelVersion"`
	CPU           float64      `json:"cpu"`
	CPUCores      int          `json:"cpuCores"`
	Memory        Memory       `json:"memory"`
	Disks         []DiskReport `json:"disks"`
	Network       Network      `json:"network"`
	Load          Load         `json:"load"`
	Uptime        uint64       `json:"uptime"`
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

// ServerConfig is the configuration returned by the backend.
type ServerConfig struct {
	ReportInterval int `json:"reportInterval"`
}
