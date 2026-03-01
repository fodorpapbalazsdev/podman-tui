// Package ui contains all Bubble Tea models and messages for podman-tui.
package ui

import "github.com/fpbpi/podman-tui/internal/models"

// ContainersLoadedMsg is sent when the container list has been fetched.
// WithStats is false for lightweight auto-refresh fetches; the Update handler
// will preserve the existing MemoryUsage/CPUUsage values in that case.
type ContainersLoadedMsg struct {
	Containers []models.Container
	Err        error
	WithStats  bool
}

// SystemDFLoadedMsg is sent when system df info has been fetched.
type SystemDFLoadedMsg struct {
	Info *models.SystemDFInfo
	Err  error
}

// ContainerActionDoneMsg is sent after a start/stop/pause/unpause/remove completes.
type ContainerActionDoneMsg struct {
	Err error
}

// ShowLogsMsg requests the logs window to open for a container.
type ShowLogsMsg struct {
	Container models.Container
}

// MachineInfoLoadedMsg is sent when podman machine info has been fetched.
type MachineInfoLoadedMsg struct {
	Info *models.MachineInfo
	Err  error
}

// SystemInfoLoadedMsg is sent when live host resource usage has been fetched.
type SystemInfoLoadedMsg struct {
	Info *models.SystemInfo
	Err  error
}

// PruneDoneMsg is sent after a system prune completes.
type PruneDoneMsg struct {
	Reclaimed string // human-readable reclaimed space, e.g. "293.8 MB"
	Err       error
}

// ShowInspectMsg requests bat to open the inspect JSON for a container.
type ShowInspectMsg struct {
	Container models.Container
}

// ShowDeleteConfirmMsg is sent when the user presses d on a container,
// asking the app layer to show a confirmation dialog before deleting.
type ShowDeleteConfirmMsg struct {
	Container models.Container
}

// autoRefreshMsg is sent by the periodic tick to trigger a background refresh.
type autoRefreshMsg struct{}
