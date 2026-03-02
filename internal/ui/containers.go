package ui

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fpbpi/podman-tui/internal/models"
	"github.com/fpbpi/podman-tui/internal/podman"
)

const (
	autoRefreshInterval      = 1 * time.Second
	statusColWidth           = 10
	pendingStatusPlaceholder = "pending   " // exactly statusColWidth visible chars; not a real status
)

// groupStatusRe matches the "X/Y up" summary injected into group header rows.
var groupStatusRe = regexp.MustCompile(`(\d+)/(\d+) up`)

// ContainersModel is the Bubble Tea model for the containers pane.
type ContainersModel struct {
	table           table.Model
	spinner         spinner.Model
	loading         bool
	fetching        bool   // true while any fetchContainers goroutine is in flight
	actionPendingID string // non-empty while a start/stop/pause/… action is in flight
	focused         bool
	err             error
	service         *podman.Service
	containers      []models.Container
	rows            []table.Row // mirrors what is in m.table; used for row-type checks
	numberMap       []int       // numberMap[i] = table row index of container number i+1
	numWidth        int         // digits needed for the largest container number (1 for ≤9, 2 for ≤99, …)
	flashMsg        string      // transient status hint shown in the title; cleared on the next keypress
	digitBuf        string      // accumulates typed digits for nG jump (e.g. "10" → press g → jump to #10)
	width           int
	height          int
}

func newContainersModel(svc *podman.Service) ContainersModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle() // coloring is applied by injectActionSpinner at render time

	cols := []table.Column{
		{Title: "#", Width: 2},
		{Title: "Name", Width: 20},
		{Title: "ID", Width: 12},
		{Title: "Image", Width: 40},
		{Title: "Status", Width: 10},
		{Title: "Ports", Width: 20},
		{Title: "Memory", Width: 12},
		{Title: "CPU", Width: 8},
	}

	t := table.New(
		table.WithColumns(cols),
		table.WithFocused(true),
		table.WithHeight(10),
	)
	ts := table.DefaultStyles()
	ts.Header = ts.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)
	ts.Selected = ts.Selected.
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("62")).
		Bold(false)
	t.SetStyles(ts)

	return ContainersModel{
		table:    t,
		spinner:  sp,
		service:  svc,
		numWidth: 2,
	}
}

// fetchContainers fetches containers with live CPU/memory stats (used on manual refresh).
func (m ContainersModel) fetchContainers() tea.Cmd {
	return func() tea.Msg {
		cons, err := m.service.GetContainers(true, true)
		return ContainersLoadedMsg{Containers: cons, Err: err, WithStats: true}
	}
}

// fetchContainersLight fetches only container list/status, skipping the slow
// podman-stats call. Used by the 1s auto-refresh tick.
func (m ContainersModel) fetchContainersLight() tea.Cmd {
	return func() tea.Msg {
		cons, err := m.service.GetContainers(true, false)
		return ContainersLoadedMsg{Containers: cons, Err: err, WithStats: false}
	}
}

func tickRefresh() tea.Cmd {
	return tea.Tick(autoRefreshInterval, func(time.Time) tea.Msg {
		return autoRefreshMsg{}
	})
}

func (m ContainersModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.fetchContainers(), tickRefresh())
}

func (m ContainersModel) Update(msg tea.Msg) (ContainersModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case spinner.TickMsg:
		if m.loading || m.actionPendingID != "" {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case ContainersLoadedMsg:
		m.loading = false
		m.fetching = false
		m.err = msg.Err
		if msg.Err == nil {
			containers := msg.Containers
			if !msg.WithStats {
				// Preserve existing CPU/memory values — stats are only
				// refreshed on manual refresh (r) or after an action.
				cache := make(map[string]*models.Container, len(m.containers))
				for i := range m.containers {
					cache[m.containers[i].ID] = &m.containers[i]
				}
				for i := range containers {
					if old, ok := cache[containers[i].ID]; ok {
						containers[i].MemoryUsage = old.MemoryUsage
						containers[i].CPUUsage = old.CPUUsage
					}
				}
			}
			m.containers = containers
			m.rebuildRows(containers)
		}

	case autoRefreshMsg:
		if !m.loading && !m.fetching {
			m.fetching = true
			cmds = append(cmds, m.fetchContainersLight())
		}
		cmds = append(cmds, tickRefresh())

	case ContainerActionDoneMsg:
		m.actionPendingID = ""
		m.err = msg.Err
		m.loading = true
		m.fetching = true
		cmds = append(cmds, m.spinner.Tick, m.fetchContainers())

	case tea.KeyMsg:
		m.flashMsg = "" // clear on every keypress
		if !m.focused {
			break
		}
		switch msg.String() {
		case "r":
			m.loading = true
			cmds = append(cmds, m.spinner.Tick, m.fetchContainers())

		case "l", "enter":
			if con := m.selectedContainer(); con != nil {
				cmds = append(cmds, func() tea.Msg { return ShowLogsMsg{Container: *con} })
			}

		case "i":
			if con := m.selectedContainer(); con != nil {
				cmds = append(cmds, func() tea.Msg { return ShowInspectMsg{Container: *con} })
			}

		case "s":
			if group := m.selectedGroupName(); group != "" {
				cmds = append(cmds, m.startGroupCmd(group))
			} else if con := m.selectedContainer(); con != nil {
				if con.Status == models.StatusRunning {
					m.flashMsg = "container is already running"
				} else {
					id := con.ID
					isPaused := con.Status == models.StatusPaused
					svc := m.service
					m.actionPendingID = id
					m.rebuildRows(m.containers)
					cmds = append(cmds, m.spinner.Tick, func() tea.Msg {
						var err error
						if isPaused {
							err = svc.UnpauseContainer(id)
						} else {
							err = svc.StartContainer(id)
						}
						return ContainerActionDoneMsg{Err: err}
					})
				}
			}

		case "t":
			if group := m.selectedGroupName(); group != "" {
				cmds = append(cmds, m.stopGroupCmd(group))
			} else if con := m.selectedContainer(); con != nil {
				if con.Status != models.StatusRunning && con.Status != models.StatusPaused {
					m.flashMsg = "container is already stopped"
				} else {
					id := con.ID
					isPaused := con.Status == models.StatusPaused
					svc := m.service
					m.actionPendingID = id
					m.rebuildRows(m.containers)
					cmds = append(cmds, m.spinner.Tick, func() tea.Msg {
						if isPaused {
							if err := svc.UnpauseContainer(id); err != nil {
								return ContainerActionDoneMsg{Err: err}
							}
						}
						return ContainerActionDoneMsg{Err: svc.StopContainer(id)}
					})
				}
			}

		case "m":
			if con := m.selectedContainer(); con != nil {
				c := *con
				cmds = append(cmds, func() tea.Msg { return ShowMoreMsg{Container: c} })
			}

		case "d":
			if con := m.selectedContainer(); con != nil {
				c := *con
				cmds = append(cmds, func() tea.Msg { return ShowDeleteConfirmMsg{Container: c} })
			}

		case "p":
			if con := m.selectedContainer(); con != nil {
				c := *con
				cmds = append(cmds, func() tea.Msg { return ShowPortsMsg{Container: c} })
			}

		case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
			m.digitBuf += msg.String()

		case "g":
			if m.digitBuf != "" {
				if n, err := strconv.Atoi(m.digitBuf); err == nil && n > 0 && n-1 < len(m.numberMap) {
					m.table.SetCursor(m.numberMap[n-1])
				}
				m.digitBuf = ""
			} else {
				var cmd tea.Cmd
				m.table, cmd = m.table.Update(msg)
				cmds = append(cmds, cmd)
			}

		case "up", "k":
			m.digitBuf = ""
			var cmd tea.Cmd
			m.table, cmd = m.table.Update(msg)
			cmds = append(cmds, cmd)
			m.skipSeparatorRows(-1)

		case "down", "j":
			m.digitBuf = ""
			var cmd tea.Cmd
			m.table, cmd = m.table.Update(msg)
			cmds = append(cmds, cmd)
			m.skipSeparatorRows(1)

		default:
			m.digitBuf = ""
			var cmd tea.Cmd
			m.table, cmd = m.table.Update(msg)
			cmds = append(cmds, cmd)
		}

	default:
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m ContainersModel) View() string {
	title := titleStyle.Render("Containers")
	if m.flashMsg != "" {
		title = title + "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(m.flashMsg)
	}
	if m.digitBuf != "" {
		jump := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true).Render(m.digitBuf)
		title = title + "  " + jump
	}

	var body string
	if m.loading {
		body = fmt.Sprintf("  %s Loading…", m.spinner.View())
	} else if m.err != nil {
		body = errorStyle.Render("Error: " + m.err.Error())
	} else {
		body = colorizeGroupHeaders(m.injectActionSpinner(colorizeTableStatuses(m.table.View())))
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, body)
}

func (m *ContainersModel) SetFocused(focused bool) {
	m.focused = focused
	m.table.Focus()
	if !focused {
		m.table.Blur()
	}
}

func (m *ContainersModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.table.SetHeight(h - 1) // reserve 1 row for title
	// Distribute column widths proportionally
	nameW := w * 16 / 100
	idW := 13
	imgW := w * 30 / 100
	statusW := 10
	portsW := w * 14 / 100
	memW := 15
	cpuW := 8
	// clamp
	if nameW < 10 {
		nameW = 10
	}
	if imgW < 20 {
		imgW = 20
	}
	if portsW < 12 {
		portsW = 12
	}
	numW := m.numWidth
	if numW < 1 {
		numW = 1
	}
	m.table.SetColumns([]table.Column{
		{Title: "#", Width: numW},
		{Title: "Name", Width: nameW},
		{Title: "ID", Width: idW},
		{Title: "Image", Width: imgW},
		{Title: "Status", Width: statusW},
		{Title: "Ports", Width: portsW},
		{Title: "Memory", Width: memW},
		{Title: "CPU", Width: cpuW},
	})
}

func (m *ContainersModel) selectedContainer() *models.Container {
	row := m.table.SelectedRow()
	if row == nil || row[2] == "" { // empty ID means a group header or separator row
		return nil
	}
	shortID := row[2]
	for i, c := range m.containers {
		if strings.HasPrefix(c.ID, shortID) {
			return &m.containers[i]
		}
	}
	return nil
}

// confirmDelete sets the pending spinner state and returns the delete command.
// Called by AppModel after the user confirms the deletion dialog.
func (m *ContainersModel) confirmDelete(id string, force bool) tea.Cmd {
	m.actionPendingID = id
	m.rebuildRows(m.containers)
	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		return ContainerActionDoneMsg{Err: m.service.RemoveContainer(id, force)}
	})
}

// pauseCmd sets the pending spinner state and returns the pause command.
func (m *ContainersModel) pauseCmd(id string) tea.Cmd {
	m.actionPendingID = id
	m.rebuildRows(m.containers)
	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		return ContainerActionDoneMsg{Err: m.service.PauseContainer(id)}
	})
}

// unpauseCmd sets the pending spinner state and returns the unpause command.
func (m *ContainersModel) unpauseCmd(id string) tea.Cmd {
	m.actionPendingID = id
	m.rebuildRows(m.containers)
	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		return ContainerActionDoneMsg{Err: m.service.UnpauseContainer(id)}
	})
}

// restartCmd sets the pending spinner state and returns the restart command.
func (m *ContainersModel) restartCmd(id string) tea.Cmd {
	m.actionPendingID = id
	m.rebuildRows(m.containers)
	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		return ContainerActionDoneMsg{Err: m.service.RestartContainer(id)}
	})
}

// rebuildRows rebuilds the table with compose grouping and the action spinner placeholder.
// It also recomputes numberMap and numWidth so digit-jump and column alignment stay correct.
func (m *ContainersModel) rebuildRows(containers []models.Container) {
	// Compute how many digits the largest container number needs (minimum 2 so
	// both numbers and ◆ always have a leading space, e.g. " 1", " ◆").
	numWidth := 2
	if n := len(containers); n >= 100 {
		numWidth = len(strconv.Itoa(n))
	}
	if numWidth != m.numWidth {
		m.numWidth = numWidth
		if m.width > 0 {
			m.SetSize(m.width, m.height) // recompute column widths with new numWidth
		}
	}

	m.rows = buildGroupedRows(containers, m.actionPendingID, numWidth)
	m.table.SetRows(m.rows)
	m.numberMap = nil
	for i, row := range m.rows {
		if row[2] != "" { // non-empty ID column = actual container row
			m.numberMap = append(m.numberMap, i)
		}
	}
}

// skipSeparatorRows advances the table cursor past blank separator rows in the
// given direction (+1 = down, -1 = up). Group-header rows are NOT skipped so
// that the user can press s/t on them to act on the whole group.
func (m *ContainersModel) skipSeparatorRows(dir int) {
	cursor := m.table.Cursor()
	for cursor >= 0 && cursor < len(m.rows) {
		row := m.rows[cursor]
		// Stop at real container rows or group-header rows; skip blank separator rows.
		if row[2] != "" || strings.HasSuffix(row[0], "◆") {
			break
		}
		cursor += dir
	}
	if cursor >= 0 && cursor < len(m.rows) {
		m.table.SetCursor(cursor)
	}
}

// selectedGroupName returns the compose project name when the cursor is on a
// group-header row, or "" otherwise.
func (m *ContainersModel) selectedGroupName() string {
	row := m.table.SelectedRow()
	if row == nil || !strings.HasSuffix(row[0], "◆") {
		return ""
	}
	return row[1]
}

// groupHasStartable reports whether the compose group has any container that can be started.
func (m *ContainersModel) groupHasStartable(project string) bool {
	for _, c := range m.containers {
		if c.ComposeProject == project && c.Status != models.StatusRunning {
			return true
		}
	}
	return false
}

// groupHasStoppable reports whether the compose group has any container that can be stopped.
func (m *ContainersModel) groupHasStoppable(project string) bool {
	for _, c := range m.containers {
		if c.ComposeProject == project && (c.Status == models.StatusRunning || c.Status == models.StatusPaused) {
			return true
		}
	}
	return false
}

// startGroupCmd starts all non-running containers that belong to project.
// Paused containers are unpaused; stopped/created containers are started.
func (m ContainersModel) startGroupCmd(project string) tea.Cmd {
	type entry struct {
		id     string
		paused bool
	}
	var entries []entry
	for _, c := range m.containers {
		if c.ComposeProject == project && c.Status != models.StatusRunning {
			entries = append(entries, entry{id: c.ID, paused: c.Status == models.StatusPaused})
		}
	}
	svc := m.service
	return func() tea.Msg {
		var lastErr error
		for _, e := range entries {
			var err error
			if e.paused {
				err = svc.UnpauseContainer(e.id)
			} else {
				err = svc.StartContainer(e.id)
			}
			if err != nil {
				lastErr = err
			}
		}
		return ContainerActionDoneMsg{Err: lastErr}
	}
}

// stopGroupCmd stops all running/paused containers that belong to project.
func (m ContainersModel) stopGroupCmd(project string) tea.Cmd {
	type entry struct {
		id     string
		paused bool
	}
	var entries []entry
	for _, c := range m.containers {
		if c.ComposeProject == project && (c.Status == models.StatusRunning || c.Status == models.StatusPaused) {
			entries = append(entries, entry{id: c.ID, paused: c.Status == models.StatusPaused})
		}
	}
	svc := m.service
	return func() tea.Msg {
		var lastErr error
		for _, e := range entries {
			if e.paused {
				if err := svc.UnpauseContainer(e.id); err != nil {
					lastErr = err
					continue
				}
			}
			if err := svc.StopContainer(e.id); err != nil {
				lastErr = err
			}
		}
		return ContainerActionDoneMsg{Err: lastErr}
	}
}

// buildGroupedRows returns table rows sorted and grouped by compose project.
// Compose groups appear first (alphabetically), each preceded by a header row;
// standalone containers follow. Within each group containers are sorted by name.
// If pendingID is non-empty, that container's status cell gets the spinner placeholder.
// numWidth is the digit-width of the largest container number, used to right-align
// the # column values.
func buildGroupedRows(containers []models.Container, pendingID string, numWidth int) []table.Row {
	type group struct {
		project    string
		containers []models.Container
	}

	var groups []group
	seen := make(map[string]int)
	var standalone []models.Container

	for _, c := range containers {
		if c.ComposeProject == "" {
			standalone = append(standalone, c)
			continue
		}
		if idx, ok := seen[c.ComposeProject]; ok {
			groups[idx].containers = append(groups[idx].containers, c)
		} else {
			seen[c.ComposeProject] = len(groups)
			groups = append(groups, group{project: c.ComposeProject, containers: []models.Container{c}})
		}
	}

	sort.Slice(groups, func(i, j int) bool { return groups[i].project < groups[j].project })
	for i := range groups {
		sort.Slice(groups[i].containers, func(a, b int) bool {
			return groups[i].containers[a].Name < groups[i].containers[b].Name
		})
	}
	sort.Slice(standalone, func(i, j int) bool { return standalone[i].Name < standalone[j].Name })

	sep := table.Row{"", "", "", "", "", "", "", ""}
	groupIndicator := strings.Repeat(" ", numWidth-1) + "◆"

	var rows []table.Row
	containerNum := 0
	for i, g := range groups {
		if i > 0 {
			rows = append(rows, sep)
		}
		// Group header row: # column holds right-aligned ◆, name column holds the project label;
		// ID column is empty so selectedContainer() skips it.
		// Status column shows "X/Y up" for the group.
		running := 0
		for _, c := range g.containers {
			if c.Status == models.StatusRunning {
				running++
			}
		}
		groupStatus := fmt.Sprintf("%d/%d up", running, len(g.containers))
		rows = append(rows, table.Row{groupIndicator, g.project, "", "", groupStatus, "", "", ""})
		for _, c := range g.containers {
			containerNum++
			rows = append(rows, containerRow(c, pendingID, true, containerNum, numWidth))
		}
	}
	if len(groups) > 0 && len(standalone) > 0 {
		rows = append(rows, sep)
	}
	for _, c := range standalone {
		containerNum++
		rows = append(rows, containerRow(c, pendingID, false, containerNum, numWidth))
	}
	return rows
}

func containerRow(c models.Container, pendingID string, inGroup bool, num int, numWidth int) table.Row {
	shortID := c.ID
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}
	status := string(c.Status)
	if pendingID != "" && c.ID == pendingID {
		status = pendingStatusPlaceholder
	}
	name := c.Name
	if inGroup {
		name = " " + name
	}
	return table.Row{
		fmt.Sprintf("%*d", numWidth, num),
		name,
		shortID,
		c.Image,
		status,
		formatPorts(c.Ports),
		formatMemory(c.MemoryUsage),
		c.CPUUsage,
	}
}

// injectActionSpinner replaces the pendingStatusPlaceholder in the already-rendered
// table string with the current spinner frame. Same post-processing approach as
// colorizeTableStatuses to avoid ANSI-in-cell runewidth truncation.
// When the pending container is the selected row, no foreground color is applied so
// the selected-row highlight (white on blue) shows through cleanly.
func (m ContainersModel) injectActionSpinner(s string) string {
	if m.actionPendingID == "" {
		return s
	}
	sv := m.spinner.View()
	pad := strings.Repeat(" ", statusColWidth-lipgloss.Width(sv))

	// Check whether the pending container is currently the highlighted row.
	// Group header rows have an empty ID column — skip those.
	selectedIsPending := false
	if row := m.table.SelectedRow(); row != nil && row[2] != "" {
		selectedIsPending = strings.HasPrefix(m.actionPendingID, row[2])
	}

	var replacement string
	if selectedIsPending {
		replacement = sv + pad
	} else {
		replacement = "\x1b[38;5;214m" + sv + "\x1b[39m" + pad
	}
	return strings.Replace(s, pendingStatusPlaceholder, replacement, 1)
}

func containersToRows(containers []models.Container) []table.Row {
	rows := make([]table.Row, 0, len(containers))
	for i, c := range containers {
		shortID := c.ID
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}
		ports := formatPorts(c.Ports)
		rows = append(rows, table.Row{
			fmt.Sprintf("%2d", i+1),
			c.Name,
			shortID,
			truncate(c.Image, 30),
			string(c.Status),
			ports,
			c.MemoryUsage,
			c.CPUUsage,
		})
	}
	return rows
}

// colorizeTableStatuses injects ANSI foreground colors into an already-rendered
// table string. The bubbles table uses runewidth.Truncate (which counts visible
// chars inside escape sequences) before lipgloss renders each cell, so embedding
// ANSI codes in cell values causes premature truncation. Post-processing the
// rendered output avoids that. \x1b[39m resets only the foreground so that the
// selected-row background highlight is not wiped out.
// colorizeGroupHeaders applies bold + accent color to group header rows.
// Header rows are identified by the ◆ prefix in the Name column.
func colorizeGroupHeaders(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if strings.Contains(line, "◆") {
			// Colorize the "X/Y up" status: green = all up, amber = partial, red = none.
			line = groupStatusRe.ReplaceAllStringFunc(line, func(match string) string {
				sub := groupStatusRe.FindStringSubmatch(match)
				running, _ := strconv.Atoi(sub[1])
				total, _ := strconv.Atoi(sub[2])
				var color string
				switch {
				case running == total:
					color = "82"  // green
				case running == 0:
					color = "196" // red
				default:
					color = "214" // amber
				}
				return fmt.Sprintf("\x1b[2m\x1b[38;5;%sm%s\x1b[22m\x1b[1m\x1b[38;5;62m", color, match)
			})
			lines[i] = "\x1b[1m\x1b[38;5;62m" + line + "\x1b[22m\x1b[39m"
		} else if len(line) > 0 && strings.TrimSpace(line) == "" {
			// All-whitespace line: only render as a dim horizontal rule when
			// there is actual content below (i.e. this is an intentional
			// separator row, not a table height-padding row at the bottom).
			hasContentBelow := false
			for j := i + 1; j < len(lines); j++ {
				if strings.TrimSpace(lines[j]) != "" {
					hasContentBelow = true
					break
				}
			}
			if hasContentBelow {
				lines[i] = "\x1b[2m\x1b[38;5;240m" + strings.Repeat("─", len(line)) + "\x1b[0m"
			}
		}
	}
	return strings.Join(lines, "\n")
}

func colorizeTableStatuses(s string) string {
	for _, entry := range []struct {
		status models.ContainerStatus
		code   string
	}{
		{models.StatusRunning, "82"},
		{models.StatusExited, "196"},
		{models.StatusPaused, "214"},
		{models.StatusCreated, "39"},
	} {
		text := string(entry.status)
		pad := strings.Repeat(" ", statusColWidth-len(text))
		s = strings.ReplaceAll(s,
			text+pad,
			fmt.Sprintf("\x1b[38;5;%sm%s\x1b[39m%s", entry.code, text, pad),
		)
	}
	return s
}

// parseMemBytes parses a memory string like "50MiB", "2GiB", "512MB" into bytes.
func parseMemBytes(s string) float64 {
	s = strings.TrimSpace(s)
	lower := strings.ToLower(s)
	for _, u := range []struct {
		suffix string
		factor float64
	}{
		{"tib", 1024 * 1024 * 1024 * 1024},
		{"gib", 1024 * 1024 * 1024},
		{"mib", 1024 * 1024},
		{"kib", 1024},
		{"tb", 1e12},
		{"gb", 1e9},
		{"mb", 1e6},
		{"kb", 1e3},
		{"b", 1},
	} {
		if strings.HasSuffix(lower, u.suffix) {
			val, err := strconv.ParseFloat(strings.TrimSpace(s[:len(s)-len(u.suffix)]), 64)
			if err == nil {
				return val * u.factor
			}
		}
	}
	return 0
}

// formatMemory converts a raw "used / total" string from podman stats into
// a compact "NNmb (X.X%)" display string. Returns the input unchanged when it
// cannot be parsed (e.g. "-" placeholder).
func formatMemory(raw string) string {
	if raw == "" || raw == "-" {
		return raw
	}
	parts := strings.SplitN(raw, "/", 2)
	if len(parts) != 2 {
		return raw
	}
	used := parseMemBytes(parts[0])
	total := parseMemBytes(parts[1])
	if total == 0 {
		return raw
	}
	pct := used / total * 100
	const gb = 1e9
	var usedStr string
	if used >= gb {
		usedStr = fmt.Sprintf("%.1fGB", used/gb)
	} else {
		usedStr = fmt.Sprintf("%.0fMB", used/1e6)
	}
	return fmt.Sprintf("%s (%.1f%%)", usedStr, pct)
}

func formatPorts(ports []models.PortMapping) string {
	if len(ports) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(ports))
	for _, p := range ports {
		if p.HostPort != "" {
			parts = append(parts, fmt.Sprintf("%s:%s→%s/%s", p.HostIP, p.HostPort, p.ContainerPort, p.Protocol))
		} else {
			parts = append(parts, fmt.Sprintf("%s/%s", p.ContainerPort, p.Protocol))
		}
	}
	return strings.Join(parts, ", ")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}
