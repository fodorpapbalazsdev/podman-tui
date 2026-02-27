package ui

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fpbpi/podman-tui/internal/models"
)

// ---- formatPorts ----

func TestFormatPorts_Empty(t *testing.T) {
	assert.Equal(t, "-", formatPorts(nil))
	assert.Equal(t, "-", formatPorts([]models.PortMapping{}))
}

func TestFormatPorts_WithHost(t *testing.T) {
	ports := []models.PortMapping{
		{HostIP: "0.0.0.0", HostPort: "80", ContainerPort: "80", Protocol: "tcp"},
	}
	assert.Equal(t, "0.0.0.0:80→80/tcp", formatPorts(ports))
}

func TestFormatPorts_ContainerOnly(t *testing.T) {
	// Empty HostPort means the port is exposed but not published.
	ports := []models.PortMapping{
		{ContainerPort: "3000", Protocol: "tcp"},
	}
	assert.Equal(t, "3000/tcp", formatPorts(ports))
}

func TestFormatPorts_Multiple(t *testing.T) {
	ports := []models.PortMapping{
		{HostIP: "0.0.0.0", HostPort: "80", ContainerPort: "80", Protocol: "tcp"},
		{HostIP: "0.0.0.0", HostPort: "443", ContainerPort: "443", Protocol: "tcp"},
	}
	assert.Equal(t, "0.0.0.0:80→80/tcp, 0.0.0.0:443→443/tcp", formatPorts(ports))
}

// ---- truncate ----

func TestTruncate_ShortString(t *testing.T) {
	assert.Equal(t, "hello", truncate("hello", 10))
}

func TestTruncate_ExactLength(t *testing.T) {
	assert.Equal(t, "hello", truncate("hello", 5))
}

func TestTruncate_LongString(t *testing.T) {
	assert.Equal(t, "hello w…", truncate("hello world", 8))
}

// ---- colorizeTableStatuses ----

func TestColorizeTableStatuses_Running(t *testing.T) {
	// "running" (7 chars) + 3 spaces fills the 10-wide status column.
	assert.Equal(t, "\x1b[38;5;82mrunning\x1b[39m   ", colorizeTableStatuses("running   "))
}

func TestColorizeTableStatuses_Exited(t *testing.T) {
	// "exited" (6 chars) + 4 spaces.
	assert.Equal(t, "\x1b[38;5;196mexited\x1b[39m    ", colorizeTableStatuses("exited    "))
}

func TestColorizeTableStatuses_Paused(t *testing.T) {
	assert.Equal(t, "\x1b[38;5;214mpaused\x1b[39m    ", colorizeTableStatuses("paused    "))
}

func TestColorizeTableStatuses_Created(t *testing.T) {
	assert.Equal(t, "\x1b[38;5;39mcreated\x1b[39m   ", colorizeTableStatuses("created   "))
}

func TestColorizeTableStatuses_NoMatch(t *testing.T) {
	input := "some unrelated text"
	assert.Equal(t, input, colorizeTableStatuses(input))
}

// ---- logsToContent ----

func TestLogsToContent_Nil(t *testing.T) {
	assert.Equal(t, "(no log output)", logsToContent(nil))
}

func TestLogsToContent_EmptySlice(t *testing.T) {
	assert.Equal(t, "(no log output)", logsToContent([]models.LogEntry{}))
}

func TestLogsToContent_WithTimestamp(t *testing.T) {
	ts, _ := time.Parse(time.RFC3339, "2024-01-02T15:04:05Z")
	logs := []models.LogEntry{{Timestamp: ts, Message: "Hello world"}}
	assert.Equal(t, "[15:04:05] Hello world\n", logsToContent(logs))
}

func TestLogsToContent_WithoutTimestamp(t *testing.T) {
	logs := []models.LogEntry{{Message: "plain message"}}
	assert.Equal(t, "plain message\n", logsToContent(logs))
}

func TestLogsToContent_MultipleEntries(t *testing.T) {
	logs := []models.LogEntry{
		{Message: "line one"},
		{Message: "line two"},
	}
	assert.Equal(t, "line one\nline two\n", logsToContent(logs))
}

// ---- containersToRows ----

func TestContainersToRows_Fields(t *testing.T) {
	cons := []models.Container{
		{
			ID:          "abc123def456ghi789",
			Name:        "web",
			Image:       "nginx:latest",
			Status:      models.StatusRunning,
			Ports:       []models.PortMapping{{HostIP: "0.0.0.0", HostPort: "80", ContainerPort: "80", Protocol: "tcp"}},
			MemoryUsage: "50MiB / 2GiB",
			CPUUsage:    "0.50%",
		},
	}
	rows := containersToRows(cons)
	require.Len(t, rows, 1)
	r := rows[0]
	assert.Equal(t, "web", r[0])
	assert.Equal(t, "abc123def456", r[1]) // ID truncated to 12 chars
	assert.Equal(t, "nginx:latest", r[2])
	assert.Equal(t, "running", r[3])
	assert.Equal(t, "0.0.0.0:80→80/tcp", r[4])
	assert.Equal(t, "50MiB / 2GiB", r[5])
	assert.Equal(t, "0.50%", r[6])
}

func TestContainersToRows_ShortID(t *testing.T) {
	cons := []models.Container{
		{ID: "short", Name: "c", Status: models.StatusRunning},
	}
	rows := containersToRows(cons)
	assert.Equal(t, "short", rows[0][1])
}

func TestContainersToRows_Empty(t *testing.T) {
	assert.Empty(t, containersToRows(nil))
}

// ---- ContainersModel update logic ----

func TestContainersModel_ContainersLoadedMsg(t *testing.T) {
	m := newContainersModel(nil)
	cons := []models.Container{
		{ID: "abc", Name: "web", Status: models.StatusRunning},
	}
	m, _ = m.Update(ContainersLoadedMsg{Containers: cons, WithStats: true})
	require.Len(t, m.containers, 1)
	assert.Equal(t, "web", m.containers[0].Name)
	assert.False(t, m.loading, "loading should be false after ContainersLoadedMsg")
	assert.False(t, m.fetching, "fetching should be false after ContainersLoadedMsg")
}

func TestContainersModel_ContainersLoadedMsg_Error(t *testing.T) {
	m := newContainersModel(nil)
	m, _ = m.Update(ContainersLoadedMsg{Err: fmt.Errorf("load failed"), WithStats: false})
	assert.Error(t, m.err)
	assert.False(t, m.loading, "loading should be false after error")
}

func TestContainersModel_StatsPreservedOnLightRefresh(t *testing.T) {
	m := newContainersModel(nil)

	// Full load: populate containers with stats.
	cons := []models.Container{
		{ID: "abc123", Name: "web", Status: models.StatusRunning, MemoryUsage: "50MiB", CPUUsage: "1%"},
	}
	m, _ = m.Update(ContainersLoadedMsg{Containers: cons, WithStats: true})

	// Light refresh: same container, updated status, no new stats (empty strings).
	lightCons := []models.Container{
		{ID: "abc123", Name: "web", Status: models.StatusExited},
	}
	m, _ = m.Update(ContainersLoadedMsg{Containers: lightCons, WithStats: false})

	assert.Equal(t, "50MiB", m.containers[0].MemoryUsage, "MemoryUsage should be preserved")
	assert.Equal(t, "1%", m.containers[0].CPUUsage, "CPUUsage should be preserved")
	assert.Equal(t, models.StatusExited, m.containers[0].Status, "Status should be updated")
}

func TestContainersModel_AutoRefresh_SetsFetching(t *testing.T) {
	m := newContainersModel(nil)
	m.fetching = false
	m.loading = false
	m, _ = m.Update(autoRefreshMsg{})
	assert.True(t, m.fetching, "fetching should be true after autoRefreshMsg when idle")
}

func TestContainersModel_AutoRefresh_SkipsWhenFetching(t *testing.T) {
	m := newContainersModel(nil)
	m.fetching = true
	m, _ = m.Update(autoRefreshMsg{})
	assert.True(t, m.fetching, "fetching flag should remain true when already fetching")
}

// ---- AppModel navigation ----

func TestAppModel_ShowLogsMsg(t *testing.T) {
	m := NewAppModel(nil)
	con := models.Container{ID: "abc123", Name: "web"}
	newModel, _ := m.Update(ShowLogsMsg{Container: con})
	assert.True(t, newModel.(AppModel).showLogs)
}

func TestAppModel_BackToContainersMsg(t *testing.T) {
	m := NewAppModel(nil)
	m.showLogs = true
	newModel, _ := m.Update(BackToContainersMsg{})
	assert.False(t, newModel.(AppModel).showLogs)
}

func TestAppModel_QuitKey(t *testing.T) {
	m := NewAppModel(nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	require.NotNil(t, cmd)
	_, ok := cmd().(tea.QuitMsg)
	assert.True(t, ok, "expected tea.QuitMsg")
}

func TestAppModel_CtrlCQuit(t *testing.T) {
	m := NewAppModel(nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	require.NotNil(t, cmd)
	_, ok := cmd().(tea.QuitMsg)
	assert.True(t, ok, "expected tea.QuitMsg")
}
