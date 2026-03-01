// Package podman wraps podman CLI calls and returns typed data models.
package podman

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fpbpi/podman-tui/internal/models"
)

const defaultTimeout = 10 * time.Second

// Service executes podman commands and parses their output.
type Service struct {
	podmanCmd string
}

// NewService returns a Service using the system's "podman" binary.
func NewService() *Service {
	return &Service{podmanCmd: "podman"}
}

// runCommand executes a podman sub-command and returns stdout, stderr, exit code.
func (s *Service) runCommand(args ...string) (string, string, int) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, s.podmanCmd, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}
	return stdout.String(), stderr.String(), exitCode
}

// ---- raw JSON shapes returned by podman CLI ----

type rawContainer struct {
	ID        string            `json:"Id"`
	Names     []string          `json:"Names"`
	Image     string            `json:"Image"`
	State     string            `json:"State"`
	Created   int64             `json:"Created"`
	StartedAt int64             `json:"StartedAt"`
	Labels    map[string]string `json:"Labels"`
	Ports     []struct {
		HostIP        string `json:"host_ip"`
		HostPort      uint16 `json:"host_port"`
		ContainerPort uint16 `json:"container_port"`
		Protocol      string `json:"protocol"`
	} `json:"Ports"`
}

// composeProject extracts the compose project name from container labels,
// checking both the Docker Compose and Podman Compose label keys.
func composeProject(labels map[string]string) string {
	for _, key := range []string{"com.docker.compose.project", "io.podman.compose.project"} {
		if v, ok := labels[key]; ok && v != "" {
			return v
		}
	}
	return ""
}

type statsEntry struct {
	MemUsage string
	CPU      string
}

// getStatsMap returns a map of short-container-ID → statsEntry by running
// `podman stats --no-stream --format json`.
// podman stats uses 12-char short IDs as keys.
func (s *Service) getStatsMap() (map[string]statsEntry, error) {
	stdout, _, code := s.runCommand("stats", "--no-stream", "--format", "json")
	if code != 0 {
		return map[string]statsEntry{}, nil
	}

	// actual podman stats JSON fields are all lowercase
	type rawStat struct {
		ID       string `json:"id"`
		MemUsage string `json:"mem_usage"`
		CPU      string `json:"cpu_percent"`
	}
	var raw []rawStat
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		return map[string]statsEntry{}, nil
	}

	result := make(map[string]statsEntry, len(raw))
	for _, r := range raw {
		result[r.ID] = statsEntry{MemUsage: r.MemUsage, CPU: r.CPU}
	}
	return result, nil
}

// GetContainers fetches the container list and optionally live stats.
// When withStats is false the stats goroutine is skipped; MemoryUsage and
// CPUUsage will be empty strings for every container.
func (s *Service) GetContainers(all, withStats bool) ([]models.Container, error) {
	var (
		wg       sync.WaitGroup
		rawCons  []rawContainer
		statsMap map[string]statsEntry
		psErr    error
		statsErr error
	)

	// goroutine 1: podman ps --format json
	wg.Add(1)
	go func() {
		defer wg.Done()
		args := []string{"ps", "--format", "json"}
		if all {
			args = append(args, "--all")
		}
		stdout, _, code := s.runCommand(args...)
		if code != 0 {
			psErr = fmt.Errorf("podman ps failed (exit %d)", code)
			return
		}
		psErr = json.Unmarshal([]byte(stdout), &rawCons)
	}()

	// goroutine 2: podman stats (optional)
	if withStats {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statsMap, statsErr = s.getStatsMap()
		}()
	}

	wg.Wait()

	if psErr != nil {
		return nil, psErr
	}
	_ = statsErr // non-fatal: continue without stats

	containers := make([]models.Container, 0, len(rawCons))
	for _, rc := range rawCons {
		name := ""
		if len(rc.Names) > 0 {
			name = strings.TrimPrefix(rc.Names[0], "/")
		}

		status := models.StatusUnknown
		switch strings.ToLower(rc.State) {
		case "running":
			status = models.StatusRunning
		case "exited":
			status = models.StatusExited
		case "paused":
			status = models.StatusPaused
		case "created":
			status = models.StatusCreated
		}

		ports := make([]models.PortMapping, 0, len(rc.Ports))
		for _, p := range rc.Ports {
			ports = append(ports, models.PortMapping{
				HostIP:        p.HostIP,
				HostPort:      strconv.Itoa(int(p.HostPort)),
				ContainerPort: strconv.Itoa(int(p.ContainerPort)),
				Protocol:      p.Protocol,
			})
		}

		mem, cpu := "-", "-"
		shortID := rc.ID
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}
		if st, ok := statsMap[shortID]; ok {
			mem = st.MemUsage
			cpu = st.CPU
		}

		containers = append(containers, models.Container{
			ID:             rc.ID,
			Name:           name,
			Image:          rc.Image,
			Status:         status,
			Created:        time.Unix(rc.Created, 0),
			Started:        time.Unix(rc.StartedAt, 0),
			Ports:          ports,
			MemoryUsage:    mem,
			CPUUsage:       cpu,
			ComposeProject: composeProject(rc.Labels),
		})
	}

	return containers, nil
}

// StartContainer starts a stopped/created container.
func (s *Service) StartContainer(id string) error {
	_, stderr, code := s.runCommand("start", id)
	if code != 0 {
		return fmt.Errorf("podman start: %s", strings.TrimSpace(stderr))
	}
	return nil
}

// StopContainer stops a running container.
func (s *Service) StopContainer(id string) error {
	_, stderr, code := s.runCommand("stop", id)
	if code != 0 {
		return fmt.Errorf("podman stop: %s", strings.TrimSpace(stderr))
	}
	return nil
}

// PauseContainer pauses a running container.
func (s *Service) PauseContainer(id string) error {
	_, stderr, code := s.runCommand("pause", id)
	if code != 0 {
		return fmt.Errorf("podman pause: %s", strings.TrimSpace(stderr))
	}
	return nil
}

// UnpauseContainer resumes a paused container.
func (s *Service) UnpauseContainer(id string) error {
	_, stderr, code := s.runCommand("unpause", id)
	if code != 0 {
		return fmt.Errorf("podman unpause: %s", strings.TrimSpace(stderr))
	}
	return nil
}

// RemoveContainer removes a container (force=true sends SIGKILL first).
func (s *Service) RemoveContainer(id string, force bool) error {
	args := []string{"rm"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, id)
	_, stderr, code := s.runCommand(args...)
	if code != 0 {
		return fmt.Errorf("podman rm: %s", strings.TrimSpace(stderr))
	}
	return nil
}

// GetContainerInspectJSON returns the raw JSON output of `podman container inspect`.
func (s *Service) GetContainerInspectJSON(id string) (string, error) {
	stdout, stderr, code := s.runCommand("container", "inspect", id)
	if code != 0 {
		return "", fmt.Errorf("podman container inspect: %s", strings.TrimSpace(stderr))
	}
	return stdout, nil
}

// GetContainerLogsRaw returns the raw text output of podman logs (stdout + stderr combined).
func (s *Service) GetContainerLogsRaw(id string, lines int) (string, error) {
	stdout, stderr, code := s.runCommand(
		"logs", "--tail", strconv.Itoa(lines), "--timestamps", id,
	)
	if code != 0 {
		return "", fmt.Errorf("podman logs: %s", strings.TrimSpace(stderr))
	}
	return stdout + stderr, nil
}

// GetContainerLogs retrieves the last `lines` log lines for a container.
func (s *Service) GetContainerLogs(id string, lines int) ([]models.LogEntry, error) {
	stdout, stderr, code := s.runCommand(
		"logs", "--tail", strconv.Itoa(lines), "--timestamps", id,
	)
	if code != 0 {
		return nil, fmt.Errorf("podman logs: %s", strings.TrimSpace(stderr))
	}

	// podman logs --timestamps outputs: "2024-01-02T15:04:05.999Z message"
	raw := strings.TrimRight(stdout+stderr, "\n")
	if raw == "" {
		return []models.LogEntry{}, nil
	}

	lines2 := strings.Split(raw, "\n")
	entries := make([]models.LogEntry, 0, len(lines2))
	for _, line := range lines2 {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		entry := models.LogEntry{Level: "INFO"}
		if len(parts) == 2 {
			if t, err := time.Parse(time.RFC3339Nano, parts[0]); err == nil {
				entry.Timestamp = t
			}
			entry.Message = parts[1]
		} else {
			entry.Message = line
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// ---- machine info ----

type rawMachineResources struct {
	CPUs     int   `json:"CPUs"`
	DiskSize int64 `json:"DiskSize"` // GiB
	Memory   int64 `json:"Memory"`   // MiB
}

type rawMachineInspect struct {
	Name      string              `json:"Name"`
	Resources rawMachineResources `json:"Resources"`
}

// GetMachineInfo returns resource allocation for the default Podman machine.
// Returns nil, nil when no machine is configured (e.g. native Linux).
func (s *Service) GetMachineInfo() (*models.MachineInfo, error) {
	stdout, _, code := s.runCommand("machine", "inspect")
	if code != 0 {
		return nil, nil
	}
	var machines []rawMachineInspect
	if err := json.Unmarshal([]byte(stdout), &machines); err != nil {
		return nil, fmt.Errorf("parse machine inspect: %w", err)
	}
	if len(machines) == 0 {
		return nil, nil
	}
	m := machines[0]
	return &models.MachineInfo{
		Name:     strings.TrimRight(m.Name, "*"),
		CPUs:     m.Resources.CPUs,
		MemoryMB: m.Resources.Memory,
		DiskGB:   m.Resources.DiskSize,
	}, nil
}

// SystemPrune removes all unused containers, images, networks, and build cache.
// It returns the reclaimed-space summary line reported by podman (e.g. "293.8 MB").
func (s *Service) SystemPrune() (string, error) {
	stdout, stderr, code := s.runCommand("system", "prune", "-f")
	if code != 0 {
		return "", fmt.Errorf("podman system prune: %s", strings.TrimSpace(stderr))
	}
	reclaimed := ""
	for _, line := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(line, "Total reclaimed space:") {
			reclaimed = strings.TrimSpace(strings.TrimPrefix(line, "Total reclaimed space:"))
			break
		}
	}
	return reclaimed, nil
}

// ---- system df ----

// rawDFEntry matches one element of the flat array podman system df --format json returns:
//
//	[{"Type":"Images","Total":4,"Size":"294.3MB","RawReclaimable":110007591,...}, ...]
type rawDFEntry struct {
	Type           string `json:"Type"`
	Total          int    `json:"Total"`
	RawReclaimable int64  `json:"RawReclaimable"`
	Size           string `json:"Size"`
	Reclaimable    string `json:"Reclaimable"`
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// GetSystemDF fetches disk usage info from `podman system df --format json`.
func (s *Service) GetSystemDF() (*models.SystemDFInfo, error) {
	stdout, stderr, code := s.runCommand("system", "df", "--format", "json")
	if code != 0 {
		return nil, fmt.Errorf("podman system df: %s", strings.TrimSpace(stderr))
	}

	var entries []rawDFEntry
	if err := json.Unmarshal([]byte(stdout), &entries); err != nil {
		return nil, fmt.Errorf("parse system df: %w", err)
	}

	info := &models.SystemDFInfo{Timestamp: time.Now()}
	var totalReclaimable int64

	for _, e := range entries {
		totalReclaimable += e.RawReclaimable
		switch e.Type {
		case "Images":
			info.ImagesCount = e.Total
			info.ImagesSize = e.Size
		case "Containers":
			info.ContainersCount = e.Total
			info.ContainersSize = e.Size
		case "Local Volumes", "Volumes":
			info.VolumesCount = e.Total
			info.VolumesSize = e.Size
		}
	}
	info.TotalReclaimable = formatBytes(totalReclaimable)

	return info, nil
}
