package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fpbpi/podman-tui/internal/podman"
)

// AppModel is the root Bubble Tea model.
//
// Layout:
//
//	┌──────────────────────────────────────┐  ← header: SystemDF bar (1 line)
//	│ containers table  OR  log viewer     │  ← main content (fills remaining height)
//	└──────────────────────────────────────┘
//	  status / keybinding hint              ← 1 line
type AppModel struct {
	containers ContainersModel
	logs       LogsModel
	systemDF   SystemDFModel
	showLogs   bool
	width      int
	height     int
	service    *podman.Service
}

// NewAppModel constructs the root model.
func NewAppModel(svc *podman.Service) AppModel {
	m := AppModel{
		containers: newContainersModel(svc),
		logs:       newLogsModel(svc),
		systemDF:   newSystemDFModel(svc),
		service:    svc,
	}
	m.containers.SetFocused(true)
	return m
}

func (m AppModel) Init() tea.Cmd {
	return tea.Batch(
		m.containers.Init(),
		m.logs.Init(),
		m.systemDF.Init(),
	)
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.applyLayout()
		return m, nil

	case tea.KeyMsg:
		// Global quit always works.
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		if m.showLogs {
			// All other keys go to the logs window.
			var cmd tea.Cmd
			m.logs, cmd = m.logs.Update(msg)
			cmds = append(cmds, cmd)
		} else {
			// Keys go to containers.
			var cmd tea.Cmd
			m.containers, cmd = m.containers.Update(msg)
			cmds = append(cmds, cmd)
		}

	// ---- navigation between views ----
	case ShowLogsMsg:
		m.showLogs = true
		m.applyLayout()
		var cmd tea.Cmd
		m.logs, cmd = m.logs.Update(msg)
		cmds = append(cmds, cmd)

	case BackToContainersMsg:
		m.showLogs = false
		m.applyLayout()

	// ---- data messages ----
	case ContainersLoadedMsg, ContainerActionDoneMsg:
		var cmd tea.Cmd
		m.containers, cmd = m.containers.Update(msg)
		cmds = append(cmds, cmd)

	case LogsLoadedMsg:
		var cmd tea.Cmd
		m.logs, cmd = m.logs.Update(msg)
		cmds = append(cmds, cmd)

	case SystemDFLoadedMsg:
		var cmd tea.Cmd
		m.systemDF, cmd = m.systemDF.Update(msg)
		cmds = append(cmds, cmd)

	default:
		// Spinner ticks and other internal messages.
		var c1, c2, c3 tea.Cmd
		m.containers, c1 = m.containers.Update(msg)
		m.logs, c2 = m.logs.Update(msg)
		m.systemDF, c3 = m.systemDF.Update(msg)
		cmds = append(cmds, c1, c2, c3)
	}

	return m, tea.Batch(cmds...)
}

// headerH is the height consumed by the SystemDF header bar.
const headerH = 1

func (m AppModel) View() string {
	if m.width == 0 {
		return "Initialising…"
	}

	header := m.systemDF.HeaderView(m.width)

	var main string
	if m.showLogs {
		border := focusedBorder.Width(m.width - 2).Height(m.mainHeight())
		main = border.Render(m.logs.View())
	} else {
		border := focusedBorder.Width(m.width - 2).Height(m.mainHeight())
		main = border.Render(m.containers.View())
	}

	statusBar := m.renderStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left, header, main, statusBar)
}

// mainHeight is the inner content area height (excluding header + border overhead + status bar).
func (m AppModel) mainHeight() int {
	h := m.height - headerH - 1 // 1 for status bar
	if h < 4 {
		h = 4
	}
	return h - 2 // subtract top+bottom border
}

func (m *AppModel) renderStatusBar() string {
	var hint string
	if m.showLogs {
		name := "none"
		if m.logs.container != nil {
			name = m.logs.container.Name
		}
		hint = "Logs: " + name + "  │  r:refresh  c:clear  ↑↓/PgUp/PgDn:scroll  esc:back  q:quit"
	} else {
		hint = "r:refresh  l:logs  s:stop  t:start  p:pause  u:unpause  d:delete  q:quit"
	}
	return statusStyle.Width(m.width).Render(hint)
}

func (m *AppModel) applyLayout() {
	if m.width == 0 {
		return
	}
	innerW := m.width - 4 // subtract border (2) + padding (2)
	innerH := m.mainHeight()
	m.containers.SetSize(innerW, innerH)
	m.logs.SetSize(innerW, innerH)
}
