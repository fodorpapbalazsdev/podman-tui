package ui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fpbpi/podman-tui/internal/models"
)

// stripANSI removes ANSI escape sequences so tests can assert on visible text.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

// ---- formatMachineMem ----

func TestFormatMachineMem(t *testing.T) {
	assert.Equal(t, "512 MiB", formatMachineMem(512))
	assert.Equal(t, "1.0 GiB", formatMachineMem(1024))
	assert.Equal(t, "2.0 GiB", formatMachineMem(2048))
	assert.Equal(t, "3.6 GiB", formatMachineMem(3712))
}

// ---- formatMemory ----

func TestFormatMemory(t *testing.T) {
	assert.Equal(t, "-", formatMemory("-"))
	assert.Equal(t, "", formatMemory(""))
	assert.Equal(t, "no slash", formatMemory("no slash")) // no "/" → passthrough
	// 50 MiB = 52 MB (decimal); 2 GiB total → 2.4%
	assert.Equal(t, "52MB (2.4%)", formatMemory("50MiB / 2GiB"))
	// 1.5 GiB = 1.6 GB; 8 GiB total → 18.8%
	assert.Equal(t, "1.6GB (18.8%)", formatMemory("1.5GiB / 8GiB"))
	// 256 MiB = 268 MB; 16 GiB total → 1.6%
	assert.Equal(t, "268MB (1.6%)", formatMemory("256MiB / 16GiB"))
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
	rows := buildGroupedRows(containers, "", 1)

	// row 0: group header for "myapp"
	require.Greater(t, len(rows), 0)
	assert.Equal(t, "◆", rows[0][0])
	assert.Equal(t, "myapp", rows[0][1])
	assert.Equal(t, "", rows[0][2], "group header should have empty ID")

	// rows 1 & 2: compose containers (sorted by name: db, web) — indented
	assert.Equal(t, " db", rows[1][1])
	assert.Equal(t, " web", rows[2][1])

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
	rows := buildGroupedRows(containers, "abc123def456", 1)

	// row 0 is header, row 1 is the container
	require.Len(t, rows, 2)
	assert.Equal(t, pendingStatusPlaceholder, rows[1][4], "pending container should have placeholder status")
}

func TestBuildGroupedRows_NoCompose(t *testing.T) {
	containers := []models.Container{
		{ID: "bbb", Name: "beta", Status: models.StatusRunning},
		{ID: "aaa", Name: "alpha", Status: models.StatusRunning},
	}
	rows := buildGroupedRows(containers, "", 1)

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
	// start requires a non-running container.
	t.Run("start (s)", func(t *testing.T) {
		m := newContainersModel(nil)
		m.SetFocused(true)
		cons := []models.Container{{ID: "abc123def456", Name: "web", Status: models.StatusExited}}
		m, _ = m.Update(ContainersLoadedMsg{Containers: cons, WithStats: true})
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
		assert.NotNil(t, cmd, "s should dispatch when container is stopped")
	})

	// stop/more/delete operate on a running container.
	for _, tc := range []struct {
		name string
		msg  tea.KeyMsg
	}{
		{"stop (t)", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}}},
		{"more (m)", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}}},
		{"delete (d)", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := containersModelWithSelection(t)
			_, cmd := m.Update(tc.msg)
			assert.NotNil(t, cmd, "key %q should dispatch a command when a container is selected", tc.name)
		})
	}

	// more (m) also works on a paused container.
	t.Run("more (m) for paused", func(t *testing.T) {
		m := newContainersModel(nil)
		m.SetFocused(true)
		cons := []models.Container{{ID: "abc123def456", Name: "web", Status: models.StatusPaused}}
		m, _ = m.Update(ContainersLoadedMsg{Containers: cons, WithStats: true})
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
		assert.NotNil(t, cmd, "m should dispatch when container is paused")
	})

	// stop (t) also works on paused containers.
	t.Run("stop paused (t)", func(t *testing.T) {
		m := newContainersModel(nil)
		m.SetFocused(true)
		cons := []models.Container{{ID: "abc123def456", Name: "web", Status: models.StatusPaused}}
		m, _ = m.Update(ContainersLoadedMsg{Containers: cons, WithStats: true})
		_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
		assert.NotNil(t, cmd, "t should dispatch when container is paused")
	})
}

func TestContainersModel_ActionKeys_NoOpWithoutSelection(t *testing.T) {
	// No containers loaded → no selection → action keys should be no-ops.
	m := newContainersModel(nil)
	m.SetFocused(true)
	for _, r := range []rune{'l', 's', 't', 'm', 'd'} {
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

// ---- formatBytesGiB ----

func TestFormatBytesGiB(t *testing.T) {
	gib := int64(1024 * 1024 * 1024)
	mib := int64(1024 * 1024)
	for _, tc := range []struct {
		in   int64
		want string
	}{
		{0, "0 MiB"},
		{512 * mib, "512 MiB"},
		{gib, "1.0 GiB"},
		{int64(float64(gib) * 5.4), "5.4 GiB"},
		{16 * gib, "16.0 GiB"},
	} {
		assert.Equal(t, tc.want, formatBytesGiB(tc.in), "formatBytesGiB(%d)", tc.in)
	}
}

// ---- renderUsageBar ----

func TestRenderUsageBar_HalfFull(t *testing.T) {
	const gib = int64(1024 * 1024 * 1024)
	plain := stripANSI(renderUsageBar(5*gib, 10*gib, 10))
	assert.Contains(t, plain, "█████░░░░░", "half-full bar should have 5 filled and 5 empty blocks")
	assert.Contains(t, plain, "5.0 GiB")
	assert.Contains(t, plain, "50%")
}

func TestRenderUsageBar_ZeroTotal(t *testing.T) {
	plain := stripANSI(renderUsageBar(0, 0, 10))
	assert.Contains(t, plain, "░░░░░░░░░░", "zero-total bar should be fully empty")
	assert.Contains(t, plain, " 0%")
}

func TestRenderUsageBar_FullWidth(t *testing.T) {
	const gib = int64(1024 * 1024 * 1024)
	plain := stripANSI(renderUsageBar(10*gib, 10*gib, 10))
	assert.Contains(t, plain, "██████████", "fully used bar should be all filled")
	assert.Contains(t, plain, "100%")
}

// ---- groupHasStartable / groupHasStoppable ----

func TestGroupHelpers_MixedGroup(t *testing.T) {
	m := newContainersModel(nil)
	cons := []models.Container{
		{ID: "a", Name: "web", Status: models.StatusRunning, ComposeProject: "app"},
		{ID: "b", Name: "db", Status: models.StatusExited, ComposeProject: "app"},
	}
	m, _ = m.Update(ContainersLoadedMsg{Containers: cons, WithStats: false})
	assert.True(t, m.groupHasStartable("app"), "exited container makes group startable")
	assert.True(t, m.groupHasStoppable("app"), "running container makes group stoppable")
}

func TestGroupHelpers_AllRunning(t *testing.T) {
	m := newContainersModel(nil)
	cons := []models.Container{
		{ID: "a", Name: "web", Status: models.StatusRunning, ComposeProject: "app"},
	}
	m, _ = m.Update(ContainersLoadedMsg{Containers: cons, WithStats: false})
	assert.False(t, m.groupHasStartable("app"), "all running → nothing to start")
	assert.True(t, m.groupHasStoppable("app"))
}

func TestGroupHelpers_AllStopped(t *testing.T) {
	m := newContainersModel(nil)
	cons := []models.Container{
		{ID: "a", Name: "web", Status: models.StatusExited, ComposeProject: "app"},
	}
	m, _ = m.Update(ContainersLoadedMsg{Containers: cons, WithStats: false})
	assert.True(t, m.groupHasStartable("app"))
	assert.False(t, m.groupHasStoppable("app"), "all stopped → nothing to stop")
}

func TestGroupHelpers_UnknownProject(t *testing.T) {
	m := newContainersModel(nil)
	cons := []models.Container{
		{ID: "a", Name: "web", Status: models.StatusRunning, ComposeProject: "app"},
	}
	m, _ = m.Update(ContainersLoadedMsg{Containers: cons, WithStats: false})
	assert.False(t, m.groupHasStartable("other"))
	assert.False(t, m.groupHasStoppable("other"))
}

// ---- "more actions" dialog flow ----

// appWithMoreContainer creates an AppModel with moreContainer set to a container
// of the given status.
func appWithMoreContainer(status models.ContainerStatus) AppModel {
	app := NewAppModel(nil, nil)
	con := models.Container{ID: "abc123def456", Name: "web", Status: status}
	app.moreContainer = &con
	return app
}

func TestAppModel_ShowMoreMsg_SetsMoreContainer(t *testing.T) {
	app := NewAppModel(nil, nil)
	con := models.Container{ID: "abc123", Name: "web", Status: models.StatusRunning}
	newModel, _ := app.Update(ShowMoreMsg{Container: con})
	got := newModel.(AppModel)
	require.NotNil(t, got.moreContainer)
	assert.Equal(t, "web", got.moreContainer.Name)
}

func TestAppModel_MoreContainer_PKey_DispatchesForRunning(t *testing.T) {
	app := appWithMoreContainer(models.StatusRunning)
	newModel, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	assert.Nil(t, newModel.(AppModel).moreContainer, "dialog should be dismissed")
	assert.NotNil(t, cmd, "p on running container should dispatch pause")
}

func TestAppModel_MoreContainer_PKey_NoOpWhenNotRunning(t *testing.T) {
	app := appWithMoreContainer(models.StatusPaused)
	newModel, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	assert.Nil(t, newModel.(AppModel).moreContainer)
	assert.Nil(t, cmd, "p on non-running container should not dispatch")
}

func TestAppModel_MoreContainer_UKey_DispatchesForPaused(t *testing.T) {
	app := appWithMoreContainer(models.StatusPaused)
	newModel, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	assert.Nil(t, newModel.(AppModel).moreContainer)
	assert.NotNil(t, cmd, "u on paused container should dispatch unpause")
}

func TestAppModel_MoreContainer_UKey_NoOpWhenNotPaused(t *testing.T) {
	app := appWithMoreContainer(models.StatusRunning)
	newModel, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	assert.Nil(t, newModel.(AppModel).moreContainer)
	assert.Nil(t, cmd, "u on non-paused container should not dispatch")
}

func TestAppModel_MoreContainer_RKey_DispatchesForRunning(t *testing.T) {
	app := appWithMoreContainer(models.StatusRunning)
	newModel, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	assert.Nil(t, newModel.(AppModel).moreContainer)
	assert.NotNil(t, cmd, "r on running container should dispatch restart")
}

func TestAppModel_MoreContainer_RKey_DispatchesForPaused(t *testing.T) {
	app := appWithMoreContainer(models.StatusPaused)
	newModel, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	assert.Nil(t, newModel.(AppModel).moreContainer)
	assert.NotNil(t, cmd, "r on paused container should dispatch restart")
}

func TestAppModel_MoreContainer_RKey_NoOpWhenStopped(t *testing.T) {
	app := appWithMoreContainer(models.StatusExited)
	newModel, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	assert.Nil(t, newModel.(AppModel).moreContainer)
	assert.Nil(t, cmd, "r on exited container should not dispatch")
}

func TestAppModel_MoreContainer_OtherKeyDismissesWithoutAction(t *testing.T) {
	app := appWithMoreContainer(models.StatusRunning)
	newModel, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	assert.Nil(t, newModel.(AppModel).moreContainer, "any other key should dismiss the dialog")
	assert.Nil(t, cmd, "no action should be dispatched")
}

// ---- port-forwards dialog ----

func TestAppModel_ShowPortsMsg_SetsPortsContainer(t *testing.T) {
	app := NewAppModel(nil, nil)
	con := models.Container{ID: "abc123", Name: "web"}
	newModel, _ := app.Update(ShowPortsMsg{Container: con})
	got := newModel.(AppModel)
	require.NotNil(t, got.portsContainer)
	assert.Equal(t, "web", got.portsContainer.Name)
}

func TestAppModel_PortsDismiss_AnyKey(t *testing.T) {
	app := NewAppModel(nil, nil)
	con := models.Container{ID: "abc123", Name: "web"}
	app.portsContainer = &con
	newModel, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	assert.Nil(t, newModel.(AppModel).portsContainer, "any key should close the ports dialog")
}

// ---- renderStatusBar context-sensitivity ----

// appWithLoadedContainer returns an AppModel with a single container loaded and selected.
func appWithLoadedContainer(t *testing.T, status models.ContainerStatus) AppModel {
	t.Helper()
	app := NewAppModel(nil, nil)
	cons := []models.Container{{ID: "abc123def456", Name: "web", Status: status}}
	newModel, _ := app.Update(ContainersLoadedMsg{Containers: cons, WithStats: false})
	return newModel.(AppModel)
}

func TestRenderStatusBar_NoSelection(t *testing.T) {
	app := NewAppModel(nil, nil)
	hint := stripANSI(app.renderStatusBar())
	assert.Contains(t, hint, "r:refresh")
	assert.Contains(t, hint, "q:quit")
	assert.NotContains(t, hint, "logs")
	assert.NotContains(t, hint, "d:delete")
}

func TestRenderStatusBar_RunningContainer(t *testing.T) {
	app := appWithLoadedContainer(t, models.StatusRunning)
	hint := stripANSI(app.renderStatusBar())
	assert.Contains(t, hint, "t:stop")
	assert.Contains(t, hint, "m:more")
	assert.Contains(t, hint, "d:delete")
	assert.NotContains(t, hint, "s:start", "start should not appear for a running container")
}

func TestRenderStatusBar_PausedContainer(t *testing.T) {
	app := appWithLoadedContainer(t, models.StatusPaused)
	hint := stripANSI(app.renderStatusBar())
	assert.Contains(t, hint, "s:start")
	assert.Contains(t, hint, "t:stop")
	assert.Contains(t, hint, "m:more")
}

func TestRenderStatusBar_ExitedContainer(t *testing.T) {
	app := appWithLoadedContainer(t, models.StatusExited)
	hint := stripANSI(app.renderStatusBar())
	assert.Contains(t, hint, "s:start")
	assert.NotContains(t, hint, "t:stop", "stop should not appear for an exited container")
	assert.NotContains(t, hint, "m:more", "more should not appear when no actions are available")
}
