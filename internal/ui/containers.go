package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fpbpi/podman-tui/internal/models"
	"github.com/fpbpi/podman-tui/internal/podman"
)

const autoRefreshInterval = 1 * time.Second

// ContainersModel is the Bubble Tea model for the containers pane.
type ContainersModel struct {
	table      table.Model
	spinner    spinner.Model
	loading    bool
	fetching   bool // true while any fetchContainers goroutine is in flight
	focused    bool
	err        error
	service    *podman.Service
	containers []models.Container
	width      int
	height     int
}

func newContainersModel(svc *podman.Service) ContainersModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("62"))

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
		if m.loading {
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
			m.table.SetRows(containersToRows(containers))
		}

	case autoRefreshMsg:
		if !m.loading && !m.fetching {
			m.fetching = true
			cmds = append(cmds, m.fetchContainersLight())
		}
		cmds = append(cmds, tickRefresh())

	case ContainerActionDoneMsg:
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

		case "l":
			if con := m.selectedContainer(); con != nil {
				cmds = append(cmds, func() tea.Msg { return ShowLogsMsg{Container: *con} })
			}

		case "s":
			if con := m.selectedContainer(); con != nil {
				id := con.ID
				cmds = append(cmds, func() tea.Msg {
					return ContainerActionDoneMsg{Err: m.service.StartContainer(id)}
				})
			}

		case "t":
			if con := m.selectedContainer(); con != nil {
				id := con.ID
				cmds = append(cmds, func() tea.Msg {
					return ContainerActionDoneMsg{Err: m.service.StopContainer(id)}
				})
			}

		case "p":
			if con := m.selectedContainer(); con != nil {
				id := con.ID
				cmds = append(cmds, func() tea.Msg {
					return ContainerActionDoneMsg{Err: m.service.PauseContainer(id)}
				})
			}

		case "u":
			if con := m.selectedContainer(); con != nil {
				id := con.ID
				cmds = append(cmds, func() tea.Msg {
					return ContainerActionDoneMsg{Err: m.service.UnpauseContainer(id)}
				})
			}

		case "d":
			if con := m.selectedContainer(); con != nil {
				id := con.ID
				cmds = append(cmds, func() tea.Msg {
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
		body = colorizeTableStatuses(m.table.View())
	}

	help := statusStyle.Render("r:refresh  s:start  t:stop  p:pause  u:unpause  d:delete  l:logs  (auto-refresh 3s)")
	return lipgloss.JoinVertical(lipgloss.Left, title, body, help)
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
	m.table.SetHeight(h - 4) // reserve rows for title + help
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
	if row == nil {
		return nil
	}
	// match by truncated ID
	shortID := row[1]
	for i, c := range m.containers {
		if strings.HasPrefix(c.ID, shortID) {
			return &m.containers[i]
		}
	}
	return nil
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
func colorizeTableStatuses(s string) string {
	const statusColWidth = 10
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
