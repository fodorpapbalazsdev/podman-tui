package ui

import (
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fpbpi/podman-tui/internal/podman"
)

// AppModel is the root Bubble Tea model.
//
// Layout:
//
//	╭──────────────────────────────────────╮
//	│ podman-tui                 15:04:05  │  ← title row
//	│──────────────────────────────────────│  ← divider
//	│ Machine: …  │  Images: …             │  ← header body (4 rows)
//	│ CPU: …      │  Containers: …         │
//	│ Mem: …      │  Volumes: …            │
//	│ Disk: …     │  Reclaimable: …        │
//	╰──────────────────────────────────────╯
//	╭──────────────────────────────────────╮
//	│ containers table                     │  ← main content
//	╰──────────────────────────────────────╯
//	  status / keybinding hint               ← 1 line
type AppModel struct {
	containers     ContainersModel
	systemDF       SystemDFModel
	pruneConfirm   bool
	pruneDone      bool   // true while the result dialog is visible
	pruneReclaimed string // reclaimed space reported by podman (may be empty)
	pruneErr       error  // non-nil if the prune failed
	loadingMsg     string // non-empty while logs/inspect is being fetched
	width          int
	height         int
	service        *podman.Service
}

// Internal messages for the bat ExecProcess flow.

type logsReadyMsg struct {
	text string
	name string
	err  error
}

type logsExitedMsg struct{ err error }

type inspectReadyMsg struct {
	json string
	name string
	err  error
}

type inspectExitedMsg struct{ err error }

const defaultLogLines = 200

// NewAppModel constructs the root model.
func NewAppModel(svc *podman.Service) AppModel {
	m := AppModel{
		containers: newContainersModel(svc),
		systemDF:   newSystemDFModel(svc),
		service:    svc,
	}
	m.containers.SetFocused(true)
	return m
}

func (m AppModel) Init() tea.Cmd {
	return tea.Batch(
		m.containers.Init(),
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

		// Any keypress dismisses the result dialog without further action.
		if m.pruneDone {
			m.pruneDone = false
			m.pruneReclaimed = ""
			m.pruneErr = nil
			break
		}

		if m.pruneConfirm {
			if msg.String() == "enter" {
				cmds = append(cmds, m.pruneCmd())
			}
			m.pruneConfirm = false
		} else {
			if msg.String() == "P" {
				m.pruneConfirm = true
			} else {
				var cmd tea.Cmd
				m.containers, cmd = m.containers.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

	// ---- bat-backed views ----
	case ShowLogsMsg:
		con := msg.Container
		svc := m.service
		m.loadingMsg = "Fetching logs for " + con.Name + "…"
		return m, func() tea.Msg {
			text, err := svc.GetContainerLogsRaw(con.ID, defaultLogLines)
			return logsReadyMsg{text: text, name: con.Name, err: err}
		}

	case logsReadyMsg:
		m.loadingMsg = ""
		if msg.err != nil {
			break
		}
		batCmd := exec.Command("bat",
			"--language=log", "--paging=always",
			"--file-name", msg.name+" (logs)")
		batCmd.Stdin = strings.NewReader(msg.text)
		return m, tea.ExecProcess(batCmd, func(err error) tea.Msg {
			return logsExitedMsg{err: err}
		})

	case logsExitedMsg:
		// TUI resumes automatically; nothing to do.

	case ShowInspectMsg:
		con := msg.Container
		svc := m.service
		m.loadingMsg = "Fetching inspect for " + con.Name + "…"
		return m, func() tea.Msg {
			json, err := svc.GetContainerInspectJSON(con.ID)
			return inspectReadyMsg{json: json, name: con.Name, err: err}
		}

	case inspectReadyMsg:
		m.loadingMsg = ""
		if msg.err != nil {
			break
		}
		batCmd := exec.Command("bat",
			"--language=json", "--paging=always",
			"--file-name", msg.name+".json")
		batCmd.Stdin = strings.NewReader(msg.json)
		return m, tea.ExecProcess(batCmd, func(err error) tea.Msg {
			return inspectExitedMsg{err: err}
		})

	case inspectExitedMsg:
		// TUI resumes automatically; nothing to do.

	// ---- data messages ----
	case ContainersLoadedMsg, ContainerActionDoneMsg:
		var cmd tea.Cmd
		m.containers, cmd = m.containers.Update(msg)
		cmds = append(cmds, cmd)

	case SystemDFLoadedMsg, MachineInfoLoadedMsg, SystemInfoLoadedMsg:
		var cmd tea.Cmd
		m.systemDF, cmd = m.systemDF.Update(msg)
		cmds = append(cmds, cmd)

	case PruneDoneMsg:
		m.pruneDone = true
		m.pruneReclaimed = msg.Reclaimed
		m.pruneErr = msg.Err
		if msg.Err == nil {
			// Refresh container list and header stats after a successful prune.
			var cmd tea.Cmd
			m.containers, cmd = m.containers.Update(ContainerActionDoneMsg{})
			cmds = append(cmds, cmd)
			cmds = append(cmds, m.systemDF.Refresh())
		}

	default:
		// Spinner ticks and other internal messages.
		var c1, c2 tea.Cmd
		m.containers, c1 = m.containers.Update(msg)
		m.systemDF, c2 = m.systemDF.Update(msg)
		cmds = append(cmds, c1, c2)
	}

	return m, tea.Batch(cmds...)
}

// headerH is the height consumed by the header: 1 title row + 1 divider + 4 data rows + 2 border rows.
const headerH = 8

func (m AppModel) View() string {
	if m.width == 0 {
		return "Initialising…"
	}

	// Modal dialogs float over the dimmed background.
	if m.loadingMsg != "" {
		return placeOverlay(m.renderLoadingDialog(), dimBackground(m.normalView()), m.width, m.height)
	}
	if m.pruneConfirm {
		return placeOverlay(m.renderPruneConfirmDialog(), dimBackground(m.normalView()), m.width, m.height)
	}
	if m.pruneDone {
		return placeOverlay(m.renderPruneResultDialog(), dimBackground(m.normalView()), m.width, m.height)
	}

	return m.normalView()
}

func (m AppModel) normalView() string {
	innerW := m.width - 2 // border adds 1 char on each side
	header := focusedBorder.Width(innerW).Render(m.systemDF.HeaderView(innerW))
	main := focusedBorder.Width(m.width - 2).Height(m.mainHeight()).Render(m.containers.View())
	return lipgloss.JoinVertical(lipgloss.Left, header, main, m.renderStatusBar())
}

const dialogContentW = 38

func (m AppModel) renderLoadingDialog() string {
	center := lipgloss.NewStyle().Width(dialogContentW).Align(lipgloss.Center)
	content := center.Render(m.loadingMsg)
	return dialogStyle.BorderForeground(lipgloss.Color("62")).Render(content)
}

func (m AppModel) renderPruneConfirmDialog() string {
	center := lipgloss.NewStyle().Width(dialogContentW).Align(lipgloss.Center)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")).Render("System Prune")

	body := "Remove all unused resources:\n" +
		"  · stopped containers\n" +
		"  · dangling images\n" +
		"  · unused networks\n" +
		"  · build cache"

	content := lipgloss.JoinVertical(lipgloss.Left,
		center.Render(title),
		"",
		body,
		"",
		center.Render(dim.Render("enter:confirm   esc:cancel")),
	)
	return dialogStyle.BorderForeground(lipgloss.Color("214")).Render(content)
}

func (m AppModel) renderPruneResultDialog() string {
	center := lipgloss.NewStyle().Width(dialogContentW).Align(lipgloss.Center)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	var title, body string
	var borderColor lipgloss.Color

	if m.pruneErr != nil {
		borderColor = lipgloss.Color("196")
		title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196")).Render("Prune Failed")
		body = errorStyle.Render(m.pruneErr.Error())
	} else {
		borderColor = lipgloss.Color("82")
		title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82")).Render("Prune Complete")
		reclaimed := m.pruneReclaimed
		if reclaimed == "" {
			reclaimed = "nothing to reclaim"
		}
		body = lipgloss.JoinVertical(lipgloss.Left,
			"Total reclaimed space:",
			"",
			center.Render(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("82")).Render(reclaimed)),
		)
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		center.Render(title),
		"",
		body,
		"",
		center.Render(dim.Render("press any key to close")),
	)
	return dialogStyle.BorderForeground(borderColor).Render(content)
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
	hint := "r:refresh  enter/l:logs  i:inspect  s:start  t:stop  p:pause  u:unpause  d:delete  P:prune  q:quit"
	return statusStyle.Width(m.width).Render(hint)
}

func (m AppModel) pruneCmd() tea.Cmd {
	return func() tea.Msg {
		reclaimed, err := m.service.SystemPrune()
		return PruneDoneMsg{Reclaimed: reclaimed, Err: err}
	}
}

func (m *AppModel) applyLayout() {
	if m.width == 0 {
		return
	}
	innerW := m.width - 4 // subtract border (2) + padding (2)
	innerH := m.mainHeight()
	m.containers.SetSize(innerW, innerH)
}
