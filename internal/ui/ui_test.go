package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fpbpi/podman-tui/internal/models"
)

// ---- formatMachineMem ----

func TestFormatMachineMem(t *testing.T) {
	assert.Equal(t, "512 MiB", formatMachineMem(512))
	assert.Equal(t, "1.0 GiB", formatMachineMem(1024))
	assert.Equal(t, "2.0 GiB", formatMachineMem(2048))
	assert.Equal(t, "3.6 GiB", formatMachineMem(3712))
}

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
	assert.Equal(t, " 1", r[0]) // line number
	assert.Equal(t, "web", r[1])
	assert.Equal(t, "abc123def456", r[2]) // ID truncated to 12 chars
	assert.Equal(t, "nginx:latest", r[3])
	assert.Equal(t, "running", r[4])
	assert.Equal(t, "0.0.0.0:80→80/tcp", r[5])
	assert.Equal(t, "50MiB / 2GiB", r[6])
	assert.Equal(t, "0.50%", r[7])
}

func TestContainersToRows_ShortID(t *testing.T) {
	cons := []models.Container{
		{ID: "short", Name: "c", Status: models.StatusRunning},
	}
	rows := containersToRows(cons)
	assert.Equal(t, "short", rows[0][2])
}

func TestContainersToRows_Empty(t *testing.T) {
	assert.Empty(t, containersToRows(nil))
}

// ---- buildGroupedRows ----

func TestBuildGroupedRows_ComposeGroupsFirst(t *testing.T) {
	containers := []models.Container{
		{ID: "aaa", Name: "standalone", Status: models.StatusRunning},
		{ID: "bbb", Name: "web", Status: models.StatusRunning, ComposeProject: "myapp"},
		{ID: "ccc", Name: "db", Status: models.StatusRunning, ComposeProject: "myapp"},
	}
	rows := buildGroupedRows(containers, "")

	// row 0: group header for "myapp"
	require.Greater(t, len(rows), 0)
	assert.Equal(t, "◆ myapp", rows[0][1])
	assert.Equal(t, "", rows[0][2], "group header should have empty ID")

	// rows 1 & 2: compose containers (sorted by name: db, web) — indented
	assert.Equal(t, "  db", rows[1][1])
	assert.Equal(t, "  web", rows[2][1])

	// row 3: separator between group and standalone
	assert.Equal(t, "", rows[3][2], "separator row should have empty ID")

	// row 4: standalone container (no indentation)
	require.Greater(t, len(rows), 4)
	assert.Equal(t, "standalone", rows[4][1])
}

func TestBuildGroupedRows_PendingStatusPlaceholder(t *testing.T) {
	containers := []models.Container{
		{ID: "abc123def456", Name: "web", Status: models.StatusRunning, ComposeProject: "myapp"},
	}
	rows := buildGroupedRows(containers, "abc123def456")

	// row 0 is header, row 1 is the container
	require.Len(t, rows, 2)
	assert.Equal(t, pendingStatusPlaceholder, rows[1][4], "pending container should have placeholder status")
}

func TestBuildGroupedRows_NoCompose(t *testing.T) {
	containers := []models.Container{
		{ID: "bbb", Name: "beta", Status: models.StatusRunning},
		{ID: "aaa", Name: "alpha", Status: models.StatusRunning},
	}
	rows := buildGroupedRows(containers, "")

	// No group headers; sorted by name.
	require.Len(t, rows, 2)
	assert.Equal(t, "alpha", rows[0][1])
	assert.Equal(t, "beta", rows[1][1])
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

func TestAppModel_ShowLogsMsg_DispatchesCmd(t *testing.T) {
	m := NewAppModel(nil, nil)
	con := models.Container{ID: "abc123", Name: "web"}
	_, cmd := m.Update(ShowLogsMsg{Container: con})
	assert.NotNil(t, cmd, "ShowLogsMsg should dispatch a fetch command")
}

func TestAppModel_ShowInspectMsg_DispatchesCmd(t *testing.T) {
	m := NewAppModel(nil, nil)
	con := models.Container{ID: "abc123", Name: "web"}
	_, cmd := m.Update(ShowInspectMsg{Container: con})
	assert.NotNil(t, cmd, "ShowInspectMsg should dispatch a fetch command")
}

func TestAppModel_QuitKey(t *testing.T) {
	m := NewAppModel(nil, nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	require.NotNil(t, cmd)
	_, ok := cmd().(tea.QuitMsg)
	assert.True(t, ok, "expected tea.QuitMsg")
}

func TestAppModel_CtrlCQuit(t *testing.T) {
	m := NewAppModel(nil, nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	require.NotNil(t, cmd)
	_, ok := cmd().(tea.QuitMsg)
	assert.True(t, ok, "expected tea.QuitMsg")
}

// ---- AppModel prune state machine ----

func TestAppModel_PruneKey_SetsPruneConfirm(t *testing.T) {
	m := NewAppModel(nil, nil)
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	assert.True(t, newModel.(AppModel).pruneConfirm)
}

func TestAppModel_PruneConfirm_EnterDispatchesCmd(t *testing.T) {
	m := NewAppModel(nil, nil)
	m.pruneConfirm = true
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.False(t, newModel.(AppModel).pruneConfirm, "confirm flag should be cleared")
	assert.NotNil(t, cmd, "prune command should be dispatched")
}

func TestAppModel_PruneConfirm_EscCancels(t *testing.T) {
	m := NewAppModel(nil, nil)
	m.pruneConfirm = true
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, newModel.(AppModel).pruneConfirm, "confirm flag should be cleared")
	assert.Nil(t, cmd, "no command should be dispatched on cancel")
}

func TestAppModel_PruneConfirm_OtherKeyCancels(t *testing.T) {
	m := NewAppModel(nil, nil)
	m.pruneConfirm = true
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	assert.False(t, newModel.(AppModel).pruneConfirm)
	assert.Nil(t, cmd)
}

func TestAppModel_PruneDoneMsg_Success(t *testing.T) {
	m := NewAppModel(nil, nil)
	newModel, cmd := m.Update(PruneDoneMsg{Reclaimed: "293.8 MB"})
	app := newModel.(AppModel)
	assert.True(t, app.pruneDone)
	assert.Equal(t, "293.8 MB", app.pruneReclaimed)
	assert.NoError(t, app.pruneErr)
	assert.NotNil(t, cmd, "refresh commands should be dispatched on success")
}

func TestAppModel_PruneDoneMsg_Error(t *testing.T) {
	m := NewAppModel(nil, nil)
	newModel, cmd := m.Update(PruneDoneMsg{Err: fmt.Errorf("prune failed")})
	app := newModel.(AppModel)
	assert.True(t, app.pruneDone)
	assert.Error(t, app.pruneErr)
	assert.Nil(t, cmd, "no refresh should be dispatched on error")
}

func TestAppModel_PruneDone_AnyKeyDismisses(t *testing.T) {
	m := NewAppModel(nil, nil)
	m.pruneDone = true
	m.pruneReclaimed = "100 MB"
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	assert.False(t, newModel.(AppModel).pruneDone)
	assert.Empty(t, newModel.(AppModel).pruneReclaimed)
}

func TestAppModel_PruneDone_EnterDismissesWithoutOpeningLogs(t *testing.T) {
	m := NewAppModel(nil, nil)
	// Load a container so enter would normally trigger ShowLogsMsg.
	cons := []models.Container{{ID: "abc123", Name: "web", Status: models.StatusRunning}}
	m.containers, _ = m.containers.Update(ContainersLoadedMsg{Containers: cons, WithStats: true})
	m.pruneDone = true

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	app := newModel.(AppModel)
	assert.False(t, app.pruneDone, "dialog should be dismissed")
	assert.Nil(t, cmd, "no command should be dispatched (logs must NOT be opened)")
}

// ---- ContainersModel key bindings ----

// containersModelWithSelection returns a focused ContainersModel with one loaded container.
func containersModelWithSelection(t *testing.T) ContainersModel {
	t.Helper()
	m := newContainersModel(nil)
	m.SetFocused(true)
	cons := []models.Container{
		{ID: "abc123def456", Name: "web", Status: models.StatusRunning},
	}
	m, _ = m.Update(ContainersLoadedMsg{Containers: cons, WithStats: true})
	return m
}

func TestContainersModel_EnterKey_EmitsShowLogsMsg(t *testing.T) {
	m := containersModelWithSelection(t)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(ShowLogsMsg)
	assert.True(t, ok, "enter should emit ShowLogsMsg")
}

func TestContainersModel_LKey_EmitsShowLogsMsg(t *testing.T) {
	m := containersModelWithSelection(t)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(ShowLogsMsg)
	assert.True(t, ok, "l should emit ShowLogsMsg")
}

func TestContainersModel_ActionKeys_DispatchCmd(t *testing.T) {
	keys := []struct {
		name string
		msg  tea.KeyMsg
	}{
		{"start (s)", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}},
		{"stop (t)", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}}},
		{"pause (p)", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}},
		{"unpause (u)", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}}},
		{"delete (d)", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}},
	}
	for _, tc := range keys {
		t.Run(tc.name, func(t *testing.T) {
			m := containersModelWithSelection(t)
			_, cmd := m.Update(tc.msg)
			assert.NotNil(t, cmd, "key %q should dispatch a command when a container is selected", tc.name)
		})
	}
}

func TestContainersModel_ActionKeys_NoOpWithoutSelection(t *testing.T) {
	// No containers loaded → no selection → action keys should be no-ops.
	m := newContainersModel(nil)
	m.SetFocused(true)
	for _, r := range []rune{'l', 's', 't', 'p', 'u', 'd'} {
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		assert.Nil(t, cmd, "key %q should not dispatch when nothing is selected", string(r))
	}
}

// ---- dimBackground ----

func TestDimBackground_AddsDimPrefix(t *testing.T) {
	result := dimBackground("hello")
	assert.True(t, strings.HasPrefix(result, "\x1b[2m"), "should start with dim code")
	assert.Contains(t, result, "hello")
	assert.True(t, strings.HasSuffix(result, "\x1b[0m"), "should end with reset")
}

func TestDimBackground_ReinsertsDimAfterReset(t *testing.T) {
	result := dimBackground("foo\x1b[0mbar")
	assert.Contains(t, result, "\x1b[0m\x1b[2m", "dim should be reinserted after every reset")
	assert.Contains(t, result, "foo")
	assert.Contains(t, result, "bar")
}

// ---- placeOverlay ----

func TestPlaceOverlay_CentersDialogOnBackground(t *testing.T) {
	// 10-wide × 5-tall background of 'A's.
	bgLine := strings.Repeat("A", 10)
	bg := strings.Join([]string{bgLine, bgLine, bgLine, bgLine, bgLine}, "\n")

	fg := "XXXX" // 4 wide, 1 tall → centered at x=3, y=2

	result := placeOverlay(fg, bg, 10, 5)
	lines := strings.Split(result, "\n")

	require.Len(t, lines, 5)
	// Row 2 (0-indexed) should contain the dialog content.
	assert.Contains(t, lines[2], "XXXX")
	// Surrounding rows should remain unchanged.
	assert.Equal(t, bgLine, lines[0])
	assert.Equal(t, bgLine, lines[4])
}

func TestPlaceOverlay_PadsShortBackgroundLines(t *testing.T) {
	// Background lines shorter than startX should be padded with spaces.
	bg := strings.Join([]string{"AB", "AB", "AB"}, "\n")
	fg := "X" // 1 wide, centered at x=4 in a 10-wide terminal

	// Should not panic.
	result := placeOverlay(fg, bg, 10, 3)
	assert.NotEmpty(t, result)
}
