// Package models defines the data structures used throughout podman-tui.
package models

import "time"

// ContainerStatus represents the current state of a container.
type ContainerStatus string

const (
	StatusRunning ContainerStatus = "running"
	StatusExited  ContainerStatus = "exited"
	StatusPaused  ContainerStatus = "paused"
	StatusCreated ContainerStatus = "created"
	StatusUnknown ContainerStatus = "unknown"
)

// PortMapping holds a single host↔container port binding.
type PortMapping struct {
	HostIP        string
	HostPort      string
	ContainerPort string
	Protocol      string
}

// Container represents a single Podman container with its runtime stats.
type Container struct {
	ID             string
	Name           string
	Status         ContainerStatus
	Image          string
	Created        time.Time
	Started        time.Time
	Ports          []PortMapping
	MemoryUsage    string
	CPUUsage       string
	ComposeProject string // empty for standalone containers
}

// SystemDFInfo holds disk-usage summary from `podman system df`.
type SystemDFInfo struct {
	ImagesCount      int
	ImagesSize       string
	ContainersCount  int
	ContainersSize   string
	VolumesCount     int
	VolumesSize      string
	TotalReclaimable string
	Timestamp        time.Time
}

// MachineInfo holds resource allocation for the active Podman machine.
// Fields are zero-valued when no machine is configured (e.g. native Linux).
type MachineInfo struct {
	Name     string
	CPUs     int
	MemoryMB int64 // total memory allocated in MiB
	DiskGB   int64 // total disk space allocated in GiB
}

// SystemInfo holds live host resource usage from `podman system info`.
type SystemInfo struct {
	MemTotalBytes  int64
	MemFreeBytes   int64
	DiskTotalBytes int64 // graphRoot filesystem total
	DiskUsedBytes  int64 // graphRoot filesystem used
}

// LogEntry is a single line from `podman logs`.
type LogEntry struct {
	Timestamp time.Time
	Message   string
	Level     string
}
