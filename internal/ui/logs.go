package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fpbpi/podman-tui/internal/models"
	"github.com/fpbpi/podman-tui/internal/podman"
)

const defaultLogLines = 200

// LogsModel is a full-screen log viewer opened on top of the containers view.
// Press Esc to return to containers.
type LogsModel struct {
	viewport  viewport.Model
	spinner   spinner.Model
	loading   bool
	err       error
	container *models.Container
	service   *podman.Service
	width     int
	height    int
}

func newLogsModel(svc *podman.Service) LogsModel {
	vp := viewport.New(80, 20)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("62"))

	return LogsModel{
		viewport: vp,
		spinner:  sp,
		service:  svc,
	}
}

func (m LogsModel) fetchLogs() tea.Cmd {
	if m.container == nil {
		return nil
	}
	id := m.container.ID
	return func() tea.Msg {
		logs, err := m.service.GetContainerLogs(id, defaultLogLines)
		return LogsLoadedMsg{Logs: logs, Err: err}
	}
}

func (m LogsModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m LogsModel) Update(msg tea.Msg) (LogsModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case ShowLogsMsg:
		m.container = &msg.Container
		m.loading = true
		m.err = nil
		cmds = append(cmds, m.spinner.Tick, m.fetchLogs())

	case LogsLoadedMsg:
		m.loading = false
		m.err = msg.Err
		if msg.Err == nil {
			m.viewport.SetContent(logsToContent(msg.Logs))
			m.viewport.GotoBottom()
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "backspace":
			// Signal the app to return to the containers view.
			cmds = append(cmds, func() tea.Msg { return BackToContainersMsg{} })
		case "r":
			if m.container != nil {
				m.loading = true
				cmds = append(cmds, m.spinner.Tick, m.fetchLogs())
			}
		case "c":
			m.container = nil
			m.err = nil
			m.viewport.SetContent("")
		default:
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)
		}

	default:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m LogsModel) View() string {
	containerName := "none"
	if m.container != nil {
		containerName = m.container.Name
	}
	title := titleStyle.Render(fmt.Sprintf("Logs: %s", containerName))

	var body string
	if m.loading {
		body = fmt.Sprintf("  %s Loading…", m.spinner.View())
	} else if m.err != nil {
		body = errorStyle.Render("Error: " + m.err.Error())
	} else {
		body = m.viewport.View()
	}

	help := statusStyle.Render("r:refresh  c:clear  ↑↓/PgUp/PgDn:scroll  esc:back")
	return lipgloss.JoinVertical(lipgloss.Left, title, body, help)
}

func (m *LogsModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.viewport.Width = w - 2
	m.viewport.Height = h - 4 // title + help lines + border
}

func logsToContent(logs []models.LogEntry) string {
	if len(logs) == 0 {
		return "(no log output)"
	}
	var sb strings.Builder
	for _, e := range logs {
		if e.Timestamp.IsZero() {
			sb.WriteString(e.Message)
		} else {
			sb.WriteString(fmt.Sprintf("[%s] %s", e.Timestamp.Format("15:04:05"), e.Message))
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}
