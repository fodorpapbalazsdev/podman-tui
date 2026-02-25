package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fpbpi/podman-tui/internal/models"
	"github.com/fpbpi/podman-tui/internal/podman"
)

// SystemDFModel fetches disk-usage data and renders it as a header bar.
type SystemDFModel struct {
	spinner spinner.Model
	loading bool
	err     error
	info    *models.SystemDFInfo
	service *podman.Service
}

func newSystemDFModel(svc *podman.Service) SystemDFModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))

	return SystemDFModel{
		spinner: sp,
		service: svc,
		loading: true,
	}
}

func (m SystemDFModel) fetchDF() tea.Cmd {
	return func() tea.Msg {
		info, err := m.service.GetSystemDF()
		return SystemDFLoadedMsg{Info: info, Err: err}
	}
}

func (m SystemDFModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.fetchDF())
}

func (m SystemDFModel) Update(msg tea.Msg) (SystemDFModel, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	case SystemDFLoadedMsg:
		m.loading = false
		m.err = msg.Err
		m.info = msg.Info
	}
	return m, nil
}

// Refresh triggers a new background fetch of system df data.
func (m *SystemDFModel) Refresh() tea.Cmd {
	m.loading = true
	return tea.Batch(m.spinner.Tick, m.fetchDF())
}

// ---- styles ----

var (
	headerBg = lipgloss.Color("62")

	headerBarStyle = lipgloss.NewStyle().
			Background(headerBg).
			Foreground(lipgloss.Color("15"))

	headerAppStyle = lipgloss.NewStyle().
			Background(headerBg).
			Foreground(lipgloss.Color("15")).
			Bold(true).
			Padding(0, 1)

	headerSepStyle = lipgloss.NewStyle().
			Background(headerBg).
			Foreground(lipgloss.Color("105"))

	headerLabelStyle = lipgloss.NewStyle().
				Background(headerBg).
				Foreground(lipgloss.Color("189"))

	headerValueStyle = lipgloss.NewStyle().
				Background(headerBg).
				Foreground(lipgloss.Color("15")).
				Bold(true)

	headerTimeStyle = lipgloss.NewStyle().
			Background(headerBg).
			Foreground(lipgloss.Color("189")).
			Padding(0, 1)
)

// HeaderView renders the one-line header bar at the top of the screen.
func (m SystemDFModel) HeaderView(width int) string {
	sep := headerSepStyle.Render(" │ ")

	left := headerAppStyle.Render("podman-tui") + sep

	if m.loading {
		left += headerBarStyle.Render(fmt.Sprintf("%s loading…", m.spinner.View()))
	} else if m.err != nil {
		left += headerBarStyle.Render("system df unavailable")
	} else if m.info != nil {
		stat := func(label, val string) string {
			return headerLabelStyle.Render(label+": ") + headerValueStyle.Render(val)
		}
		left += stat("Images", fmt.Sprintf("%d (%s)", m.info.ImagesCount, m.info.ImagesSize))
		left += headerBarStyle.Render("  ")
		left += stat("Containers", fmt.Sprintf("%d (%s)", m.info.ContainersCount, m.info.ContainersSize))
		left += headerBarStyle.Render("  ")
		left += stat("Volumes", fmt.Sprintf("%d (%s)", m.info.VolumesCount, m.info.VolumesSize))
		left += sep
		left += stat("Reclaimable", m.info.TotalReclaimable)
	}

	right := headerTimeStyle.Render(time.Now().Format("15:04:05"))

	// Pad the middle to push the timestamp to the right edge.
	leftLen := lipgloss.Width(left)
	rightLen := lipgloss.Width(right)
	pad := width - leftLen - rightLen
	if pad < 0 {
		pad = 0
	}
	middle := headerBarStyle.Render(strings.Repeat(" ", pad))

	return left + middle + right
}

// renderDFInfo is kept for potential future use (e.g. a detail popup).
func renderDFInfo(info *models.SystemDFInfo) string {
	row := func(label, value string) string {
		return dfLabelStyle.Render(label) + dfValueStyle.Render(value)
	}
	return strings.Join([]string{
		row("  Images:      ", fmt.Sprintf("%d  (%s)", info.ImagesCount, info.ImagesSize)),
		row("  Containers:  ", fmt.Sprintf("%d  (%s)", info.ContainersCount, info.ContainersSize)),
		row("  Volumes:     ", fmt.Sprintf("%d  (%s)", info.VolumesCount, info.VolumesSize)),
		row("  Reclaimable: ", info.TotalReclaimable),
	}, "\n")
}

var (
	dfLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Width(22)
	dfValueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
)
