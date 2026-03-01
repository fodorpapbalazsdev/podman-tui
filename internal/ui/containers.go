package ui

import (
	"fmt"
	"sort"
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
	width           int
	height          int
}

func newContainersModel(svc *podman.Service) ContainersModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle() // coloring is applied by injectActionSpinner at render time

	cols := []table.Column{
		{Title: "Name", Width: 20},
		{Title: "ID", Width: 12},
		{Title: "Image", Width: 30},
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
		table:   t,
		spinner: sp,
		service: svc,
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
			if con := m.selectedContainer(); con != nil {
				id := con.ID
				m.actionPendingID = id
				m.rebuildRows(m.containers)
				cmds = append(cmds, m.spinner.Tick, func() tea.Msg {
					return ContainerActionDoneMsg{Err: m.service.StartContainer(id)}
				})
			}

		case "t":
			if con := m.selectedContainer(); con != nil {
				id := con.ID
				m.actionPendingID = id
				m.rebuildRows(m.containers)
				cmds = append(cmds, m.spinner.Tick, func() tea.Msg {
					return ContainerActionDoneMsg{Err: m.service.StopContainer(id)}
				})
			}

		case "p":
			if con := m.selectedContainer(); con != nil {
				id := con.ID
				m.actionPendingID = id
				m.rebuildRows(m.containers)
				cmds = append(cmds, m.spinner.Tick, func() tea.Msg {
					return ContainerActionDoneMsg{Err: m.service.PauseContainer(id)}
				})
			}

		case "u":
			if con := m.selectedContainer(); con != nil {
				id := con.ID
				m.actionPendingID = id
				m.rebuildRows(m.containers)
				cmds = append(cmds, m.spinner.Tick, func() tea.Msg {
					return ContainerActionDoneMsg{Err: m.service.UnpauseContainer(id)}
				})
			}

		case "d":
			if con := m.selectedContainer(); con != nil {
				id := con.ID
				m.actionPendingID = id
				m.rebuildRows(m.containers)
				cmds = append(cmds, m.spinner.Tick, func() tea.Msg {
					return ContainerActionDoneMsg{Err: m.service.RemoveContainer(id, false)}
				})
			}

		default:
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
	nameW := w * 18 / 100
	idW := 13
	imgW := w * 25 / 100
	statusW := 10
	portsW := w * 18 / 100
	memW := 14
	cpuW := 8
	// clamp
	if nameW < 10 {
		nameW = 10
	}
	if imgW < 15 {
		imgW = 15
	}
	if portsW < 12 {
		portsW = 12
	}
	m.table.SetColumns([]table.Column{
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
	if row == nil || row[1] == "" { // empty ID means a group header row
		return nil
	}
	shortID := row[1]
	for i, c := range m.containers {
		if strings.HasPrefix(c.ID, shortID) {
			return &m.containers[i]
		}
	}
	return nil
}

// rebuildRows rebuilds the table with compose grouping and the action spinner placeholder.
func (m *ContainersModel) rebuildRows(containers []models.Container) {
	m.table.SetRows(buildGroupedRows(containers, m.actionPendingID))
}

// buildGroupedRows returns table rows sorted and grouped by compose project.
// Compose groups appear first (alphabetically), each preceded by a header row;
// standalone containers follow. Within each group containers are sorted by name.
// If pendingID is non-empty, that container's status cell gets the spinner placeholder.
func buildGroupedRows(containers []models.Container, pendingID string) []table.Row {
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

	sep := table.Row{"", "", "", "", "", "", ""}

	var rows []table.Row
	for i, g := range groups {
		if i > 0 {
			rows = append(rows, sep)
		}
		// Group header row: name column holds the project label; ID column is empty
		// so selectedContainer() skips it (empty ID never matches a real container).
		rows = append(rows, table.Row{"◆ " + g.project, "", "", "", "", "", ""})
		for _, c := range g.containers {
			rows = append(rows, containerRow(c, pendingID, true))
		}
	}
	if len(groups) > 0 && len(standalone) > 0 {
		rows = append(rows, sep)
	}
	for _, c := range standalone {
		rows = append(rows, containerRow(c, pendingID, false))
	}
	return rows
}

func containerRow(c models.Container, pendingID string, inGroup bool) table.Row {
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
		name = "  " + name
	}
	return table.Row{
		name,
		shortID,
		truncate(c.Image, 30),
		status,
		formatPorts(c.Ports),
		c.MemoryUsage,
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
	if row := m.table.SelectedRow(); row != nil && row[1] != "" {
		selectedIsPending = strings.HasPrefix(m.actionPendingID, row[1])
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
	for _, c := range containers {
		shortID := c.ID
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}

		ports := formatPorts(c.Ports)

		rows = append(rows, table.Row{
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
		if strings.Contains(line, "◆ ") {
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
