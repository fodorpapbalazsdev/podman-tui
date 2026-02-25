// Package ui contains all Bubble Tea models and messages for podman-tui.
package ui

import "github.com/fpbpi/podman-tui/internal/models"

// ContainersLoadedMsg is sent when the container list has been fetched.
type ContainersLoadedMsg struct {
	Containers []models.Container
	Err        error
}

// LogsLoadedMsg is sent when container logs have been fetched.
type LogsLoadedMsg struct {
	Logs []models.LogEntry
	Err  error
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

// BackToContainersMsg is sent from the logs window when the user presses Esc.
type BackToContainersMsg struct{}

// autoRefreshMsg is sent by the periodic tick to trigger a background refresh.
type autoRefreshMsg struct{}
